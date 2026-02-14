package main

import (
	"context"
	"fmt"
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
	// Load .env (ignore error if file doesn't exist — env vars may be set directly)
	_ = godotenv.Load()

	// Initialize structured JSON logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load configuration
	cfg := config.Load()
	slog.Info("configuration loaded", "port", cfg.Port)

	// Create auth database connection pool (stocks-data)
	ctx := context.Background()
	authPool, err := db.NewPoolWithSearchPath(ctx, cfg.DatabaseURL, "auth")
	if err != nil {
		slog.Error("failed to create auth database pool", "error", err)
		os.Exit(1)
	}
	defer authPool.Close()
	slog.Info("auth database pool established")

	// Create market database connection pool (stocks / ingest schema)
	marketPool, err := db.NewPool(ctx, cfg.MarketDatabaseURL)
	if err != nil {
		slog.Error("failed to create market database pool", "error", err)
		os.Exit(1)
	}
	defer marketPool.Close()
	slog.Info("market database pool established")

	// Create market status checker
	checker := market.NewChecker()

	// Create handlers
	authHandler := auth.NewHandler(authPool, cfg)
	instrumentsHandler := handlers.NewInstrumentsHandler(authPool, marketPool)
	favoritesHandler := handlers.NewFavoritesHandler(authPool, marketPool)
	marketHandler := handlers.NewMarketHandler(checker)
	dashboardHandler := handlers.NewDashboardHandler(authPool, marketPool, checker)
	streamHandler := handlers.NewStreamHandler(authPool, marketPool, checker)
	adminHandler := handlers.NewAdminHandler(authPool, cfg.AdminSecret)
	apiKeysHandler := handlers.NewAPIKeysHandler(authPool)
	apiV1Handler := handlers.NewAPIV1Handler(authPool, marketPool)

	// Set up router
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.RequestID)
	r.Use(slogMiddleware)
	r.Use(chimw.Recoverer)
	r.Use(chimw.RealIP)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.CORSOrigin},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Admin-Secret", "X-API-Key"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check endpoints
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		pingCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := authPool.Ping(pingCtx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"not ready","error":"auth database unreachable"}`))
			return
		}
		if err := marketPool.Ping(pingCtx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"not ready","error":"market database unreachable"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	})

	// Auth routes
	r.Route("/api/auth", func(r chi.Router) {
		r.Use(auth.RateLimit(10))
		r.Post("/signup", authHandler.Signup)
		r.Post("/login", authHandler.Login)
		r.Post("/logout", authHandler.Logout)
		r.Post("/refresh", authHandler.RefreshToken)
		r.With(auth.RequireAuth(cfg.JWTSecret)).Get("/me", authHandler.Me)
	})

	// Instrument routes (public with optional auth for is_favorite)
	r.Route("/api/instruments", func(r chi.Router) {
		r.With(auth.OptionalAuth(cfg.JWTSecret)).Get("/", instrumentsHandler.List)
		r.Get("/filters", instrumentsHandler.Filters)
		r.Get("/{symbol}", instrumentsHandler.Detail)
		r.Get("/{symbol}/profile", instrumentsHandler.Profile)
		r.Get("/{symbol}/fundamentals", instrumentsHandler.Fundamentals)
		r.Get("/{symbol}/prices", instrumentsHandler.Prices)
	})

	// Favorites routes (authenticated)
	r.Route("/api/favorites", func(r chi.Router) {
		r.Use(auth.RequireAuth(cfg.JWTSecret))
		r.Get("/", favoritesHandler.Get)
		r.Put("/", favoritesHandler.Update)
	})

	// Market status routes
	r.Route("/api/market", func(r chi.Router) {
		r.Get("/status", marketHandler.Status)
	})

	// Dashboard route (authenticated)
	r.With(auth.RequireAuth(cfg.JWTSecret)).Get("/api/dashboard", dashboardHandler.Get)

	// SSE streaming routes
	r.Route("/api/stream", func(r chi.Router) {
		// SSE endpoints accept token via query param, so use OptionalAuth
		// for the instrument stream (public) and RequireAuth pattern for favorites
		r.Get("/{symbol}", streamHandler.InstrumentStream)
		r.With(auth.RequireAuth(cfg.JWTSecret)).Get("/favorites", streamHandler.FavoritesStream)
	})

	// Admin routes
	r.Route("/api/admin", func(r chi.Router) {
		r.Use(adminHandler.RequireAdminSecret)
		r.Post("/referral-codes", adminHandler.CreateReferralCode)
	})

	// API key management routes (authenticated via JWT)
	r.Route("/api/api-keys", func(r chi.Router) {
		r.Use(auth.RequireAuth(cfg.JWTSecret))
		r.Post("/", apiKeysHandler.Create)
		r.Get("/", apiKeysHandler.List)
		r.Delete("/{id}", apiKeysHandler.Revoke)
	})

	// Public API v1 routes (authenticated via API key)
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(auth.RequireAPIKey(authPool))
		r.Use(auth.APIKeyRateLimit(60))

		r.Get("/instruments", apiV1Handler.ListInstruments)
		r.Get("/instruments/{symbol}", apiV1Handler.GetInstrument)
		r.Get("/instruments/{symbol}/prices", apiV1Handler.GetPrices)
		r.Get("/instruments/{symbol}/quotes", apiV1Handler.GetQuotes)
		r.Get("/instruments/{symbol}/profile", apiV1Handler.GetProfile)
		r.Get("/instruments/{symbol}/fundamentals", apiV1Handler.GetFundamentals)

		// SSE stream via API key
		r.Get("/stream/{symbol}", streamHandler.InstrumentStream)
	})

	// Create server
	addr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second, // longer for SSE
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		slog.Info("server starting", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("shutting down server", "signal", sig.String())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped gracefully")
}

// slogMiddleware is a chi-compatible middleware that logs requests using slog.
func slogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)

		defer func() {
			slog.Info("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", chimw.GetReqID(r.Context()),
				"remote_addr", r.RemoteAddr,
			)
		}()

		next.ServeHTTP(ww, r)
	})
}
