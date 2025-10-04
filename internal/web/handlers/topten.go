package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"prospero/internal/features/topten"
)

// topTenService interface for dependency injection
type topTenService interface {
	GetRandomList() (*topten.TopTenList, error)
}

// TopTen handles the /api/top-ten endpoint
func TopTen(service topTenService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get the format parameter (default to json)
		format := r.URL.Query().Get("format")
		if format == "" {
			format = "json"
		}

		// Get a random list
		list, err := service.GetRandomList()
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get random list: %v", err), http.StatusInternalServerError)
			return
		}

		switch format {
		case "ascii":
			// Return ASCII formatted text
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			asciiOutput := topten.FormatListAsASCII(list)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(asciiOutput))

		case "json":
			// Return JSON
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(list); err != nil {
				http.Error(w, fmt.Sprintf("Failed to encode JSON: %v", err), http.StatusInternalServerError)
				return
			}

		default:
			http.Error(w, "Invalid format parameter. Use 'json' or 'ascii'", http.StatusBadRequest)
			return
		}
	}
}

// Health handles the /health endpoint
func Health() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := map[string]string{
			"status":  "healthy",
			"service": "prospero",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}
