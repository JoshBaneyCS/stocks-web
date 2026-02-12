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

	// Create database connection pool
	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to create database pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	slog.Info("database connection pool created")

	// Create market status checker
	checker := market.NewChecker()

	// Create handlers
	authHandler := auth.NewHandler(pool, cfg)
	instrumentsHandler := handlers.NewInstrumentsHandler(pool)
	favoritesHandler := handlers.NewFavoritesHandler(pool)
	marketHandler := handlers.NewMarketHandler(checker)
	dashboardHandler := handlers.NewDashboardHandler(pool, checker)
	streamHandler := handlers.NewStreamHandler(pool, checker)
	adminHandler := handlers.NewAdminHandler(pool, cfg.AdminSecret)

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
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Admin-Secret"},
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
		if err := pool.Ping(pingCtx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"not ready","error":"database unreachable"}`))
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
