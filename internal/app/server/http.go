package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"prospero/assets"
	"prospero/internal/features/shakespert"
	"prospero/internal/features/topten"
	"prospero/internal/mcp"
	"prospero/internal/web/handlers"
)

// rawTerminalLogger is a custom logging middleware that uses \r\n for raw terminal mode
func rawTerminalLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		t1 := time.Now()

		defer func() {
			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}
			log.Printf("\"%s %s://%s%s %s\" from %s - %d %dB in %s\r",
				r.Method,
				scheme,
				r.Host,
				r.RequestURI,
				r.Proto,
				r.RemoteAddr,
				ww.Status(),
				ww.BytesWritten(),
				time.Since(t1),
			)
		}()

		next.ServeHTTP(ww, r)
	})
}

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

	// Initialize the MCP server
	mcpServer := mcp.NewServer("prospero", "1.0.0")
	promptFS := assets.GetEmbeddedPrompts()
	definitions, err := mcp.LoadPromptsFromTOML(promptFS)
	if err != nil {
		return fmt.Errorf("failed to load MCP prompts: %w", err)
	}

	// Register prompts with handlers
	for _, def := range definitions {
		prompt := def.ToPrompt()
		var handler mcp.PromptHandler
		if def.Content != "" {
			handler = def.CreateHandler()
		} else {
			handler = func(ctx context.Context, args map[string]string) (*mcp.GetPromptResult, error) {
				return nil, fmt.Errorf("handler not implemented for prompt: %s", prompt.Name)
			}
		}
		mcpServer.RegisterPrompt(prompt, handler)
	}

	// Create router
	r := chi.NewRouter()

	// Add middleware
	r.Use(rawTerminalLogger)
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
	r.Get("/api/info", handlers.Info())
	r.Get("/api/topten", handlers.TopTen(toptenService))

	// Shakespert routes
	r.Get("/api/shakespert/works", handlers.ShakespertWorks(shakespertService))
	r.Get("/api/shakespert/works/*", handlers.ShakespertWork(shakespertService))
	r.Get("/api/shakespert/genres", handlers.ShakespertGenres(shakespertService))

	// MCP routes
	r.HandleFunc("/mcp", mcpServer.HTTPHandler())

	// Create server
	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%s", host, port),
		Handler: r,
	}

	fmt.Printf("🌐 HTTP Server starting on http://%s:%s\r\n", host, port)
	fmt.Printf("📡 Endpoints:\r\n")
	fmt.Printf("   GET  /health                    - Health check\r\n")
	fmt.Printf("   GET  /api/info                  - Server information\r\n")
	fmt.Printf("   GET  /api/topten                - Random Top 10 list\r\n")
	fmt.Printf("   GET  /api/shakespert/works      - List Shakespeare works\r\n")
	fmt.Printf("   GET  /api/shakespert/works/{id} - Get specific work details\r\n")
	fmt.Printf("   GET  /api/shakespert/genres     - List available genres\r\n")
	fmt.Printf("   💡 curl auto-detects and returns ASCII format\r\n")
	fmt.Printf("   (Add ?format=json|text|ascii to override)\r\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\r\n")
	fmt.Printf("🤖 MCP Server:\r\n")
	fmt.Printf("   POST /mcp                       - MCP JSON-RPC endpoint\r\n")
	fmt.Printf("   GET  /mcp                       - MCP SSE stream endpoint\r\n")
	fmt.Printf("   Loaded %d prompts from TOML files\r\n", len(definitions))
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
