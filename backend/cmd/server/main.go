package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"

	"github.com/JoshBaneyCS/stocks-web/backend/internal/auth"
	"github.com/JoshBaneyCS/stocks-web/backend/internal/config"
	"github.com/JoshBaneyCS/stocks-web/backend/internal/db"
	"github.com/JoshBaneyCS/stocks-web/backend/internal/handlers"
	"github.com/JoshBaneyCS/stocks-web/backend/internal/market"
)

func main() {
	// ── Load .env for local dev (ignored in production) ──────────
	_ = godotenv.Load()

	// ── Structured logger ────────────────────────────────────────
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// ── Configuration ────────────────────────────────────────────
	cfg := config.Load()
	slog.Info("config loaded",
		"port", cfg.Port,
		"db_host", cfg.DBHost,
		"cors_origin", cfg.CORSOrigin,
	)

	// ── Database connection pool ─────────────────────────────────
	pool, err := db.NewPool(context.Background(), cfg)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	slog.Info("database pool established")

	// ── Market status checker ────────────────────────────────────
	mkt := market.NewChecker()

	// ── Router ───────────────────────────────────────────────────
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))
	r.Use(structuredLogger)

	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.CORSOrigin},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// ── Health probes ────────────────────────────────────────────
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			http.Error(w, `{"status":"not_ready"}`, http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready"}`))
	})

	// ── Auth handler (public routes) ─────────────────────────────
	authHandler := auth.NewHandler(pool, cfg)

	r.Route("/api/auth", func(r chi.Router) {
		r.Post("/signup", authHandler.Signup)
		r.Post("/login", authHandler.Login)
		r.Post("/logout", authHandler.Logout)
		r.With(auth.Middleware(cfg)).Get("/me", authHandler.Me)
	})

	// ── Protected API routes ─────────────────────────────────────
	r.Route("/api", func(r chi.Router) {
		r.Use(auth.Middleware(cfg))

		// Stocks
		stocksHandler := handlers.NewStocksHandler(pool)
		r.Get("/stocks", stocksHandler.List)
		r.Get("/stocks/{symbol}", stocksHandler.Detail)
		r.Get("/stocks/{symbol}/prices", stocksHandler.Prices)
		r.Get("/stocks/{symbol}/news", stocksHandler.News)

		// Favorites
		favHandler := handlers.NewFavoritesHandler(pool)
		r.Get("/favorites", favHandler.Get)
		r.Put("/favorites", favHandler.Update)

		// Market status
		mktHandler := handlers.NewMarketHandler(mkt)
		r.Get("/market/status", mktHandler.Status)

		// SSE streaming
		streamHandler := handlers.NewStreamHandler(pool, mkt)
		r.Get("/stream/stocks/{symbol}", streamHandler.StockStream)
	})

	// ── Admin routes (secret-gated) ──────────────────────────────
	if cfg.AdminSecret != "" {
		r.Route("/api/admin", func(r chi.Router) {
			r.Use(adminAuth(cfg.AdminSecret))
			r.Post("/referral-codes", authHandler.CreateReferralCode)
		})
	}

	// ── Start server with graceful shutdown ──────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second, // longer for SSE
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		slog.Info("server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server listen error", "error", err)
			os.Exit(1)
		}
	}()

	<-done
	slog.Info("server shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}
	slog.Info("server stopped")
}

// structuredLogger logs each request with slog.
func structuredLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"bytes", ww.BytesWritten(),
			"duration_ms", time.Since(start).Milliseconds(),
			"remote", r.RemoteAddr,
		)
	})
}

// adminAuth validates the X-Admin-Secret header against the configured secret.
func adminAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Admin-Secret") != secret {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
