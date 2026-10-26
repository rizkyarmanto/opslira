// Command api runs the Opslira Go API: the /api/v1 HTTP surface plus the
// in-process scheduler (probe ticks, report dispatch, daily rollup/prune).
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	opslira "opslira/backend"
	"opslira/backend/internal/auth"
	"opslira/backend/internal/config"
	"opslira/backend/internal/credentials"
	"opslira/backend/internal/dashboard"
	"opslira/backend/internal/db"
	"opslira/backend/internal/httpx"
	"opslira/backend/internal/incidents"
	"opslira/backend/internal/mailer"
	"opslira/backend/internal/probes"
	"opslira/backend/internal/probes/adapters"
	"opslira/backend/internal/redis"
	"opslira/backend/internal/reporting"
	"opslira/backend/internal/scheduler"
	"opslira/backend/internal/targets"
	"opslira/backend/internal/users"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(logger); err != nil {
		logger.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.DB)
	if err != nil {
		return err
	}
	defer pool.Close()

	rdb := redis.NewData(cfg.Redis)
	cache := redis.NewCache(cfg.Redis)
	if err := rdb.Ping(ctx).Err(); err != nil {
		return err
	}
	if err := cache.Ping(ctx).Err(); err != nil {
		return err
	}
	defer func() {
		_ = rdb.Close()
		_ = cache.Close()
	}()

	// Schema bootstrap: embedded SQL migrations, idempotent so a legacy
	// Laravel schema (already migrated by hand) passes through cleanly.
	applied, err := opslira.ApplyMigrations(ctx, pool)
	if err != nil {
		return err
	}
	if applied > 0 {
		logger.Info("migrations applied", "count", applied)
	}
	scheduler.EnsurePartitions(ctx, pool, cfg.Location())

	// Boot-time seeding: ADMIN_PASSWORD switches on ProductionSeeder
	// (superadmin only, exactly like the PHP env-gated path); without it the
	// local DatabaseSeeder runs (4 users / 8 targets / 3 schedules). Both
	// are update-or-create idempotent.
	if cfg.AdminPassword != "" {
		if err := opslira.SeedProduction(ctx, pool, cfg); err != nil {
			logger.Error("production seed failed", "error", err)
		} else {
			logger.Info("production seed done: superadmin only")
		}
	} else {
		if err := opslira.SeedAll(ctx, pool, cfg); err != nil {
			logger.Error("database seed failed", "error", err)
		} else {
			logger.Info("database seed done: users, targets, schedules")
		}
	}

	creds, err := credentials.New(cfg.AppEncryptionKey, cfg.AppKey)
	if err != nil {
		return err
	}

	sessionStore := auth.NewStore(rdb, cfg.SessionTTL)
	authMW := auth.Middleware(pool, sessionStore)
	sender := mailer.New(cfg.Mail)
	alerts := incidents.New(pool, sender, cfg, logger)
	// The factory needs the DB1 cache client: Beszel tokens and the WHOIS
	// rate limit live in the cache database (Laravel's Redis Cache facade).
	probeFactory := adapters.NewFactory(adapters.Deps{
		Cache:               probes.NewRedisCache(cache),
		BeszelTokenCacheTTL: cfg.BeszelTokenCacheTTL,
	})
	probeSvc := probes.NewService(pool, rdb, creds, probeFactory, probes.Hooks{
		OpenIncident:     alerts.Open,
		ResolveIncidents: alerts.ResolveOpen,
		Notify:           alerts.Notify,
	}, logger)
	reportHandler := reporting.NewHandler(pool, sender, cfg, logger)

	router := chi.NewRouter()
	router.Use(httpx.CORS(cfg.FrontendOrigin))

	// Health endpoints: Laravel's /up (outside auth) plus /healthz for the
	// container healthcheck.
	router.Get("/up", healthUp)
	router.Get("/healthz", healthUp)

	router.Route("/api/v1", func(r chi.Router) {
		authHandler := auth.NewHandler(pool, sessionStore, cfg.SessionTTL, cfg.FrontendOrigin)
		authHandler.Mount(r, authMW)
		// /me is the alias the SPA may call when it wants the session without
		// the /auth prefix; both answer identically, anonymous included.
		r.Group(func(r chi.Router) {
			r.Use(authMW)
			r.Get("/me", authHandler.Me)
		})
		users.NewHandler(pool).Mount(r, authMW)
		targets.NewHandler(pool, rdb, cache, creds, probeSvc).Mount(r, authMW)
		reportHandler.Mount(r, authMW)
		dashboard.NewHandler(pool, rdb, cache, reportHandler.SLA(), cfg).Mount(r, authMW)
	})

	sched := scheduler.New(probeSvc, reportHandler, pool, cfg, logger)
	go sched.Start(ctx)

	addr := cfg.ListenAddr
	server := &http.Server{Addr: addr, Handler: router}
	errCh := make(chan error, 1)
	go func() {
		logger.Info("api listening", "addr", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutting down")
	case err := <-errCh:
		if err != nil {
			return err
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return server.Shutdown(shutdownCtx)
}

// healthUp answers the same way Laravel's /up does: 200 when the app is up.
func healthUp(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
