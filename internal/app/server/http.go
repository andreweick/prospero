package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"prospero/internal/features/shakespert"
	"prospero/internal/features/topten"
	"prospero/internal/web/handlers"
)

// StartHTTPServer starts the HTTP server with the given host and port
func StartHTTPServer(ctx context.Context, host, port string) error {
	// Initialize the topten service
	toptenService, err := topten.NewService(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize topten service: %w", err)
	}

	// Initialize the shakespert service
	shakespertService, err := shakespert.NewService(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize shakespert service: %w", err)
	}
	defer shakespertService.Close()

	// Create router
	r := chi.NewRouter()

	// Add middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Timeout(60 * time.Second))

	// Add CORS middleware for API usage
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

			if r.Method == "OPTIONS" {
				return
			}

			next.ServeHTTP(w, r)
		})
	})

	// Routes
	r.Get("/health", handlers.Health())
	r.Get("/api/top-ten", handlers.TopTen(toptenService))

	// Shakespert routes
	r.Get("/api/shakespert/works", handlers.ShakespertWorks(shakespertService))
	r.Get("/api/shakespert/works/*", handlers.ShakespertWork(shakespertService))
	r.Get("/api/shakespert/genres", handlers.ShakespertGenres(shakespertService))

	// Create server
	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%s", host, port),
		Handler: r,
	}

	fmt.Printf("🌐 HTTP Server starting on http://%s:%s\r\n", host, port)
	fmt.Printf("📡 Endpoints:\r\n")
	fmt.Printf("   GET  /health                    - Health check\r\n")
	fmt.Printf("   GET  /api/top-ten               - Random Top 10 list (JSON)\r\n")
	fmt.Printf("   GET  /api/top-ten?format=ascii  - Random Top 10 list (ASCII)\r\n")
	fmt.Printf("   GET  /api/shakespert/works      - List Shakespeare works\r\n")
	fmt.Printf("   GET  /api/shakespert/works/{id} - Get specific work details\r\n")
	fmt.Printf("   GET  /api/shakespert/genres     - List available genres\r\n")
	fmt.Printf("   (Add ?format=text for plain text responses)\r\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\r\n")

	// Start server in a goroutine so we can handle context cancellation
	go func() {
		<-ctx.Done()
		fmt.Printf("🌐 Shutting down HTTP server...\r\n")

		// Give server 5 seconds to shut down gracefully
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			fmt.Printf("HTTP server forced to shutdown: %v\n", err)
		}
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}
