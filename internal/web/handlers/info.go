package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Info handles the /api/info endpoint
func Info() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get the format parameter
		format := r.URL.Query().Get("format")

		// Auto-detect curl and default to ASCII
		if format == "" {
			userAgent := r.Header.Get("User-Agent")
			if strings.Contains(strings.ToLower(userAgent), "curl") {
				format = "ascii"
			} else {
				format = "json"
			}
		}

		switch format {
		case "text", "ascii":
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			writeInfoAsText(w)

		case "json":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			info := map[string]interface{}{
				"service":     "prospero",
				"description": "An interactive API for exploring classic literature and entertainment",
				"endpoints": []map[string]string{
					{
						"method":      "GET",
						"path":        "/health",
						"description": "Health check endpoint",
					},
					{
						"method":      "GET",
						"path":        "/api/info",
						"description": "Server information and available endpoints",
						"parameters":  "?format=json|text|ascii",
					},
					{
						"method":      "GET",
						"path":        "/api/topten",
						"description": "Get a random Dave's Top 10 list",
						"parameters":  "?format=json|ascii",
					},
					{
						"method":      "GET",
						"path":        "/api/shakespert/works",
						"description": "List all Shakespeare works",
						"parameters":  "?format=json|text|ascii&genre=<type>",
					},
					{
						"method":      "GET",
						"path":        "/api/shakespert/works/{id}",
						"description": "Get specific work details",
						"parameters":  "?format=json|text|ascii",
					},
					{
						"method":      "GET",
						"path":        "/api/shakespert/genres",
						"description": "List all genres",
						"parameters":  "?format=json|text|ascii",
					},
				},
				"notes": []string{
					"Endpoints auto-detect curl and return ASCII format by default",
					"Use ?format=json for JSON responses",
					"Use ?format=text or ?format=ascii for plain text responses",
				},
			}

			if err := json.NewEncoder(w).Encode(info); err != nil {
				http.Error(w, fmt.Sprintf("Failed to encode JSON: %v", err), http.StatusInternalServerError)
				return
			}

		default:
			http.Error(w, "Invalid format parameter. Use 'json', 'text', or 'ascii'", http.StatusBadRequest)
			return
		}
	}
}

func writeInfoAsText(w http.ResponseWriter) {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString("════════════════════════════════════════════════════════════════════\n")
	b.WriteString("                  🎩 Prospero HTTP API                              \n")
	b.WriteString("════════════════════════════════════════════════════════════════════\n")
	b.WriteString("\n")
	b.WriteString("An interactive API for exploring classic literature and entertainment.\n")
	b.WriteString("\n")

	b.WriteString("Available Endpoints:\n")
	b.WriteString("────────────────────────────────────────────────────────────────────\n")
	b.WriteString("\n")

	b.WriteString("  GET  /health\n")
	b.WriteString("       Health check endpoint\n")
	b.WriteString("\n")

	b.WriteString("  GET  /api/info\n")
	b.WriteString("       Server information and available endpoints\n")
	b.WriteString("       Parameters: ?format=json|text|ascii\n")
	b.WriteString("\n")

	b.WriteString("  GET  /api/topten\n")
	b.WriteString("       Get a random Dave's Top 10 list\n")
	b.WriteString("       Parameters: ?format=json|ascii\n")
	b.WriteString("\n")

	b.WriteString("  GET  /api/shakespert/works\n")
	b.WriteString("       List all Shakespeare works\n")
	b.WriteString("       Parameters: ?format=json|text|ascii&genre=<type>\n")
	b.WriteString("\n")

	b.WriteString("  GET  /api/shakespert/works/{id}\n")
	b.WriteString("       Get specific work details\n")
	b.WriteString("       Parameters: ?format=json|text|ascii\n")
	b.WriteString("\n")

	b.WriteString("  GET  /api/shakespert/genres\n")
	b.WriteString("       List all genres\n")
	b.WriteString("       Parameters: ?format=json|text|ascii\n")
	b.WriteString("\n")

	b.WriteString("💡 Tips:\n")
	b.WriteString("────────────────────────────────────────────────────────────────────\n")
	b.WriteString("\n")
	b.WriteString("  • Endpoints auto-detect curl and return ASCII format by default\n")
	b.WriteString("  • Use ?format=json for JSON responses\n")
	b.WriteString("  • Use ?format=text or ?format=ascii for plain text responses\n")
	b.WriteString("\n")

	b.WriteString("Examples:\n")
	b.WriteString("────────────────────────────────────────────────────────────────────\n")
	b.WriteString("\n")
	b.WriteString("  curl http://localhost:8080/api/info\n")
	b.WriteString("  curl http://localhost:8080/api/topten\n")
	b.WriteString("  curl http://localhost:8080/api/shakespert/works\n")
	b.WriteString("  curl http://localhost:8080/api/shakespert/works/hamlet\n")
	b.WriteString("  curl http://localhost:8080/api/shakespert/genres\n")
	b.WriteString("\n")
	b.WriteString("════════════════════════════════════════════════════════════════════\n")
	b.WriteString("\n")

	fmt.Fprint(w, b.String())
}
