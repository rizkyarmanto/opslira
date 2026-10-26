package adapters

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver

	"opslira/backend/internal/probes"
)

// DatabasePing probes DATABASE targets: TCP handshake + native driver
// "SELECT 1" (DatabasePingProbeAdapter). Supported drivers: pgsql and
// mysql/mariadb; sqlsrv has no Go driver in this build and reports DOWN.
type DatabasePing struct{ adapterBase }

// NewDatabasePing builds the DB_NATIVE_PING adapter.
func NewDatabasePing() *DatabasePing { return &DatabasePing{adapterBase: newBase()} }

func (a *DatabasePing) SupportsProtocol(protocolType string) bool {
	return protocolType == probes.ProtocolDBNativePing
}

func (a *DatabasePing) Configure(config, credentials map[string]any) probes.Adapter {
	a.apply(config, credentials)
	return a
}

func (a *DatabasePing) Tune(timeoutSeconds, degradedThresholdMs int) probes.Adapter {
	a.tune(timeoutSeconds, degradedThresholdMs)
	return a
}

func (a *DatabasePing) Execute(ctx context.Context) probes.ProbeResult {
	host := a.cfgString("host", "")
	if host == "" {
		return probes.Down("Missing host in config_payload", nil, nil)
	}
	port := a.cfgInt("port", 5432)
	driver := a.cfgString("db_driver", "pgsql")
	database := a.cfgString("database_name", "")
	query := a.cfgString("query", "SELECT 1")
	username := a.credString("username")
	password := a.credString("password")

	dsn, sqlDriver, err := a.dsn(driver, host, port, database, username, password)
	if err != nil {
		return probes.Down("DB probe error: "+err.Error(), nil, nil)
	}

	start := time.Now()
	db, err := sql.Open(sqlDriver, dsn)
	if err != nil {
		return probes.Down("DB probe error: "+err.Error(), nil, nil)
	}
	defer db.Close()

	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(time.Duration(a.timeoutSeconds) * time.Second)

	// PDO opens the connection lazily too; the handshake is the PingContext.
	pingCtx, cancel := context.WithTimeout(ctx, time.Duration(a.timeoutSeconds)*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return probes.Down("DB probe error: "+err.Error(), nil, nil)
	}
	tcpHandshake := round2(float64(time.Since(start).Microseconds()) / 1000)

	queryStart := time.Now()
	rows, err := db.QueryContext(pingCtx, query)
	if err != nil {
		return probes.Down("DB probe error: "+err.Error(), nil, nil)
	}
	for rows.Next() {
		// Drain like PDO fetchAll(); the value is irrelevant.
	}
	_ = rows.Close()
	queryExec := round2(float64(time.Since(queryStart).Microseconds()) / 1000)

	metrics := map[string]any{
		"tcp_handshake_ms": tcpHandshake,
		"query_exec_ms":    queryExec,
	}

	if a.cfgBool("collect_connection_stats", true) {
		for key, value := range a.connectionStats(db, driver) {
			metrics[key] = value
		}
	}

	// The form sends "true"/"false" as text; a plain cast would turn the
	// string "false" on (PHP filter_var).
	if a.cfgBool("check_replication_lag", false) {
		for key, value := range a.replicationLag(db, driver) {
			metrics[key] = value
		}
	}

	latency := round2(float64(time.Since(start).Microseconds()) / 1000)
	status := a.classifyLatency(msPointer(latency))
	return probes.ProbeResult{Status: status, ResponseTimeMs: msPointer(latency), MetricsPayload: metrics}
}

// dsn maps the PHP PDO drivers onto database/sql drivers; unknown drivers
// fall back to pgsql like PHP's default match arm.
func (a *DatabasePing) dsn(driver, host string, port int, database, username, password string) (string, string, error) {
	switch driver {
	case "mysql", "mariadb":
		cfg := mysql.NewConfig()
		cfg.User = username
		cfg.Passwd = password
		cfg.Net = "tcp"
		cfg.Addr = net.JoinHostPort(host, strconv.Itoa(port))
		cfg.DBName = database
		cfg.Timeout = time.Duration(a.timeoutSeconds) * time.Second
		cfg.ReadTimeout = time.Duration(a.timeoutSeconds) * time.Second
		cfg.WriteTimeout = time.Duration(a.timeoutSeconds) * time.Second
		return cfg.FormatDSN(), "mysql", nil
	case "sqlsrv":
		return "", "", fmt.Errorf("unsupported db_driver 'sqlsrv' (supported: pgsql, mysql, mariadb)")
	default: // pgsql and anything unknown, like PHP's default arm
		dsn := fmt.Sprintf(
			"host=%s port=%d dbname=%s user=%s password=%s connect_timeout=%d sslmode=prefer",
			host, port, database, username, password, a.timeoutSeconds,
		)
		return dsn, "pgx", nil
	}
}

// connectionStats ports the best-effort active/max connection metrics; any
// error simply omits them (PHP swallows with an empty catch).
func (a *DatabasePing) connectionStats(db *sql.DB, driver string) map[string]any {
	statsCtx, cancel := context.WithTimeout(context.Background(), time.Duration(a.timeoutSeconds)*time.Second)
	defer cancel()

	switch driver {
	case "pgsql":
		var active, max int
		if err := db.QueryRowContext(statsCtx, "SELECT count(*) FROM pg_stat_activity").Scan(&active); err != nil {
			return nil
		}
		if err := db.QueryRowContext(statsCtx, "SHOW max_connections").Scan(&max); err != nil {
			return nil
		}
		return map[string]any{"active_connections": active, "max_connections": max}
	case "mysql", "mariadb":
		var active, max int
		if err := db.QueryRowContext(statsCtx, "SHOW STATUS LIKE 'Threads_connected'").Scan(new(string), &active); err != nil {
			return nil
		}
		if err := db.QueryRowContext(statsCtx, "SHOW VARIABLES LIKE 'max_connections'").Scan(new(string), &max); err != nil {
			return nil
		}
		return map[string]any{"active_connections": active, "max_connections": max}
	default:
		return nil
	}
}

// replicationLag is best-effort and metric-only: a monitoring user without
// the right grant simply omits it (PHP replicationLag).
func (a *DatabasePing) replicationLag(db *sql.DB, driver string) map[string]any {
	lagCtx, cancel := context.WithTimeout(context.Background(), time.Duration(a.timeoutSeconds)*time.Second)
	defer cancel()

	switch driver {
	case "pgsql":
		// Replay lag on a standby; NULL on a primary maps to 0.
		var lag int
		err := db.QueryRowContext(lagCtx,
			`SELECT COALESCE(CAST(EXTRACT(EPOCH FROM (now() - pg_last_xact_replay_timestamp())) AS int), 0)`,
		).Scan(&lag)
		if err != nil {
			return nil
		}
		return map[string]any{"replication_lag_seconds": lag}
	case "mysql", "mariadb":
		// Seconds_Behind_Source (newer) or Seconds_Behind_Slave (older); the
		// position varies, so the column is looked up by name.
		lag := replicaLagSeconds(lagCtx, db, "SHOW REPLICA STATUS")
		if lag == nil {
			lag = replicaLagSeconds(lagCtx, db, "SHOW SLAVE STATUS")
		}
		if lag == nil {
			return nil
		}
		return map[string]any{"replication_lag_seconds": *lag}
	default:
		return nil
	}
}

// replicaLagSeconds runs a SHOW REPLICA/SLAVE STATUS statement and reads the
// lag column by name; nil when absent or unreadable.
func replicaLagSeconds(ctx context.Context, db *sql.DB, statement string) *int {
	rows, err := db.QueryContext(ctx, statement)
	if err != nil {
		return nil
	}
	defer rows.Close()

	if !rows.Next() {
		return nil
	}
	columns, err := rows.Columns()
	if err != nil {
		return nil
	}
	values := make([]any, len(columns))
	for i := range values {
		values[i] = new(sql.RawBytes)
	}
	if err := rows.Scan(values...); err != nil {
		return nil
	}
	for i, column := range columns {
		if column != "Seconds_Behind_Source" && column != "Seconds_Behind_Slave" {
			continue
		}
		raw, ok := values[i].(*sql.RawBytes)
		if !ok || raw == nil {
			return nil
		}
		if lag, err := strconv.Atoi(string(*raw)); err == nil {
			return &lag
		}
		return nil
	}
	return nil
}
