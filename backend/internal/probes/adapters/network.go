package adapters

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"math"
	"net"
	"os"
	"runtime"
	"strconv"
	"time"

	gosnmp "github.com/gosnmp/gosnmp"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"

	"opslira/backend/internal/probes"
)

// Network probes NETWORK_DEVICE targets: ICMP ping with optional SNMP OID
// polling and a TCP fallback when ICMP is blocked
// (NetworkSnmpProbeAdapter). It handles both ICMP_SNMP and ICMP_PING like the
// PHP factory map; INTERNET targets go through Internet, which pins the
// ping-only behaviour.
type Network struct {
	adapterBase
	forceICMPOnly bool
}

// NewNetwork builds the ICMP_SNMP/ICMP_PING adapter.
func NewNetwork() *Network { return &Network{adapterBase: newBase()} }

func (a *Network) SupportsProtocol(protocolType string) bool {
	return protocolType == probes.ProtocolICMPSNMP || protocolType == probes.ProtocolICMPPing
}

func (a *Network) Configure(config, credentials map[string]any) probes.Adapter {
	a.apply(config, credentials)
	return a
}

func (a *Network) Tune(timeoutSeconds, degradedThresholdMs int) probes.Adapter {
	a.tune(timeoutSeconds, degradedThresholdMs)
	return a
}

func (a *Network) Execute(ctx context.Context) probes.ProbeResult {
	host := a.cfgString("host", "")
	if host == "" {
		return probes.Down("Missing host in config_payload", nil, nil)
	}

	// An empty form field stores null; absent or invalid means the schema
	// default of 4 packets (PHP icmp_count handling).
	count := 4
	if raw, ok := a.config["icmp_count"]; ok && raw != nil {
		if n := toInt(raw, 0); n > 0 {
			count = n
		}
	}
	// INTERNET targets are ping only: no TCP fallback, no SNMP reading.
	icmpOnly := a.cfgBool("icmp_only", false) || a.forceICMPOnly

	icmp := a.icmpPing(ctx, host, count)
	latency := icmp.avgMs

	if !icmp.success {
		// Go's ICMP client has no ping binary stdout, so the PHP "raw output"
		// detail slot always reads as the generic placeholder.
		message := fmt.Sprintf("Ping failed (%s): no output from ping", icmp.error)

		if icmpOnly {
			return probes.Down(message, nil, latency)
		}

		// Fallback: TCP connect check when ICMP is unavailable.
		tcpLatency, tcpOK := a.tcpFallback(ctx, host)
		if !tcpOK {
			return probes.Down(message, nil, latency)
		}
		metrics := map[string]any{
			"packet_loss_pct": nil,
			"icmp_available":  false,
			"tcp_fallback":    true,
			"icmp_error":      icmp.error,
		}
		return probes.Up(tcpLatency, metrics, nil)
	}

	metrics := map[string]any{
		"packet_loss_pct": icmp.packetLoss,
		"jitter_ms":       icmp.jitter,
		"icmp_available":  true,
	}

	// Optional SNMP metrics.
	if !icmpOnly && a.cfgBool("enable_snmp", false) {
		for key, value := range a.snmpMetrics(ctx, host) {
			metrics[key] = value
		}
	}

	status := a.classifyLatency(latency)
	return probes.ProbeResult{Status: status, ResponseTimeMs: latency, MetricsPayload: metrics}
}

type icmpStats struct {
	success    bool
	avgMs      *float64
	packetLoss *float64 // percent, like spatie/ping
	jitter     *float64
	error      string
}

// privilegedPing: raw ICMP sockets need CAP_NET_RAW (or root); without it the
// probe falls back to unprivileged datagram sockets on Linux. Windows only
// has raw sockets.
func privilegedPing() bool {
	if runtime.GOOS == "windows" {
		return true
	}
	return os.Geteuid() == 0
}

// icmpPing sends count echo requests over a raw (or unprivileged datagram)
// ICMP socket and reports loss as a percentage, jitter as the standard
// deviation of round trips, and failure whenever any packet is lost (the PHP
// ping binary exits non-zero then).
func (a *Network) icmpPing(ctx context.Context, host string, count int) icmpStats {
	// Resolve first so a DNS failure reads "host not found" instead of a
	// syscall error from the socket write.
	resolved, err := net.DefaultResolver.LookupIP(ctx, "ip4", host)
	if err != nil || len(resolved) == 0 {
		return icmpStats{error: "host not found"}
	}
	target := resolved[0]

	network := "udp4"
	if privilegedPing() {
		network = "ip4:icmp"
	}
	conn, err := icmp.ListenPacket(network, "")
	if err != nil {
		return icmpStats{error: pingErrorTag(err)}
	}
	defer conn.Close()

	identifier := os.Getpid() & 0xffff
	deadline := time.Now().Add(time.Duration(a.timeoutSeconds) * time.Second)
	_ = conn.SetDeadline(deadline)

	metrics := icmpStats{}
	var rtts []float64
	sent := 0

	for seq := 1; seq <= count; seq++ {
		if ctx.Err() != nil {
			break
		}
		request := icmp.Message{
			Type: ipv4.ICMPTypeEcho,
			Code: 0,
			Body: &icmp.Echo{ID: identifier, Seq: seq, Data: []byte("opslira")},
		}
		raw, err := request.Marshal(nil)
		if err != nil {
			return icmpStats{error: err.Error()}
		}
		start := time.Now()
		if _, err := conn.WriteTo(raw, &net.IPAddr{IP: target}); err != nil {
			return icmpStats{error: pingErrorTag(err)}
		}
		sent++

		// Wait for this packet's reply (the socket deadline bounds the wait).
		buffer := make([]byte, 1500)
		for {
			n, _, err := conn.ReadFrom(buffer)
			if err != nil {
				break // per-packet timeout or socket deadline: lost
			}
			parsed, err := icmp.ParseMessage(1, buffer[:n])
			if err != nil {
				continue
			}
			echo, ok := parsed.Body.(*icmp.Echo)
			if !ok || parsed.Type != ipv4.ICMPTypeEchoReply || echo.Seq != seq {
				continue
			}
			rtts = append(rtts, round2(float64(time.Since(start).Microseconds())/1000))
			break
		}

		// Small pacing gap like the ping binary's default interval.
		select {
		case <-ctx.Done():
		case <-time.After(time.Millisecond):
		}
	}

	loss := round2(float64(sent-len(rtts)) * 100 / float64(max(sent, 1)))
	metrics.packetLoss = &loss
	if len(rtts) > 0 {
		var sum float64
		for _, rtt := range rtts {
			sum += rtt
		}
		avg := round2(sum / float64(len(rtts)))
		metrics.avgMs = &avg

		variance := 0.0
		for _, rtt := range rtts {
			variance += (rtt - avg) * (rtt - avg)
		}
		jitter := round2(math.Sqrt(variance / float64(len(rtts))))
		metrics.jitter = &jitter
	}

	switch {
	case sent == 0:
		metrics.error = "context canceled"
	case len(rtts) != sent:
		if len(rtts) == 0 && ctx.Err() == nil {
			metrics.error = "request timeout"
		} else {
			metrics.error = "packet loss"
		}
		return metrics
	}

	metrics.success = true
	return metrics
}

// pingErrorTag classifies low-level ping failures so the DOWN message reads
// "operation not permitted" instead of a wrapped syscall dump.
func pingErrorTag(err error) string {
	switch {
	case errors.Is(err, os.ErrPermission):
		return "operation not permitted (needs CAP_NET_RAW)"
	case errors.Is(err, context.Canceled):
		return "context canceled"
	default:
		return err.Error()
	}
}

// tcpFallback: a plain TCP connect when ICMP is blocked, using the shared
// Port column (PHP tcpFallback; port 0/absent disables it).
func (a *Network) tcpFallback(ctx context.Context, host string) (*float64, bool) {
	port := toInt(a.config["port"], 0)
	if port == 0 {
		return nil, false
	}
	dialer := net.Dialer{Timeout: time.Duration(a.timeoutSeconds) * time.Second}
	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	latency := round2(float64(time.Since(start).Microseconds()) / 1000)
	if err != nil {
		return nil, false
	}
	_ = conn.Close()
	return msPointer(latency), true
}

// snmpMetrics reads the sysUpTime OID with the stored credential. Version is
// driven by the credential payload; there is no silent "public" community
// fallback (PHP snmpCommandArguments).
func (a *Network) snmpMetrics(ctx context.Context, host string) map[string]any {
	session, ok := a.snmpSession(host)
	if !ok {
		return map[string]any{"snmp_available": false, "snmp_skipped": "no credentials configured"}
	}

	if err := session.Connect(); err != nil {
		reason := probes.Mask(err.Error())
		slog.WarnContext(ctx, "SNMP poll failed", "host", host, "error", reason)
		return map[string]any{"snmp_available": false, "snmp_error": reason}
	}
	defer session.Conn.Close()

	if _, err := session.Get([]string{"1.3.6.1.2.1.1.3.0"}); err != nil {
		reason := probes.Mask(err.Error())
		slog.WarnContext(ctx, "SNMP poll failed", "host", host, "error", reason)
		return map[string]any{"snmp_available": false, "snmp_error": reason}
	}
	return map[string]any{"snmp_available": true}
}

// snmpSession builds the gosnmp session from community (v2c) or the v3
// authPriv triple; false means "no SNMP credentials configured".
func (a *Network) snmpSession(host string) (*gosnmp.GoSNMP, bool) {
	session := &gosnmp.GoSNMP{
		Target:  host,
		Port:    161,
		Timeout: time.Duration(a.timeoutSeconds) * time.Second,
		Retries: 0,
		MaxOids: 1,
		Logger:  gosnmp.NewLogger(log.New(io.Discard, "", 0)),
	}

	if community := a.credString("community"); community != "" {
		session.Version = gosnmp.Version2c
		session.Community = community
		return session, true
	}

	if a.credNonEmpty("username", "auth_password") {
		session.Version = gosnmp.Version3
		session.MsgFlags = gosnmp.AuthPriv
		session.SecurityModel = gosnmp.UserSecurityModel
		session.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName:                 a.credString("username"),
			AuthenticationProtocol:   snmpAuthProtocol(a.credString("auth_protocol")),
			AuthenticationPassphrase: a.credString("auth_password"),
			PrivacyProtocol:          snmpPrivProtocol(a.credString("priv_protocol")),
			PrivacyPassphrase:        a.credString("priv_password"),
		}
		return session, true
	}

	return nil, false
}

func snmpAuthProtocol(value string) gosnmp.SnmpV3AuthProtocol {
	switch value {
	case "MD5":
		return gosnmp.MD5
	default: // "SHA" and the validation enum's only other value are covered here
		return gosnmp.SHA
	}
}

func snmpPrivProtocol(value string) gosnmp.SnmpV3PrivProtocol {
	switch value {
	case "DES":
		return gosnmp.DES
	default: // "AES"
		return gosnmp.AES
	}
}
