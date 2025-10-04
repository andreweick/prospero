package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"prospero/internal/features/shakespert"
)

// shakespertService interface for dependency injection
type shakespertService interface {
	ListWorks(ctx context.Context) ([]shakespert.WorkSummary, error)
	GetWork(ctx context.Context, workID string) (*shakespert.WorkDetail, error)
	ListGenres(ctx context.Context) ([]shakespert.Genre, error)
	GetWorksByGenre(ctx context.Context, genreType string) ([]shakespert.WorkSummary, error)
}

// ShakespertWorks handles the /api/shakespert/works endpoint
func ShakespertWorks(service shakespertService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get query parameters
		genre := r.URL.Query().Get("genre")
		format := r.URL.Query().Get("format")
		if format == "" {
			format = "json"
		}

		var works []shakespert.WorkSummary
		var err error

		// Get works by genre or all works
		if genre != "" {
			works, err = service.GetWorksByGenre(ctx, genre)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to get works by genre: %v", err), http.StatusInternalServerError)
				return
			}
		} else {
			works, err = service.ListWorks(ctx)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to list works: %v", err), http.StatusInternalServerError)
				return
			}
		}

		switch format {
		case "text", "ascii":
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			writeWorksAsText(w, works)
		case "json":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"works": works,
				"count": len(works),
			}); err != nil {
				http.Error(w, fmt.Sprintf("Failed to encode JSON: %v", err), http.StatusInternalServerError)
				return
			}
		default:
			http.Error(w, "Invalid format parameter. Use 'json' or 'text'", http.StatusBadRequest)
			return
		}
	}
}

// ShakespertWork handles the /api/shakespert/works/{workID} endpoint
func ShakespertWork(service shakespertService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Extract workID from URL path
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) < 4 {
			http.Error(w, "Work ID is required", http.StatusBadRequest)
			return
		}
		workID := parts[3] // /api/shakespert/works/{workID}

		format := r.URL.Query().Get("format")
		if format == "" {
			format = "json"
		}

		work, err := service.GetWork(ctx, workID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, fmt.Sprintf("Work not found: %s", workID), http.StatusNotFound)
			} else {
				http.Error(w, fmt.Sprintf("Failed to get work: %v", err), http.StatusInternalServerError)
			}
			return
		}

		switch format {
		case "text", "ascii":
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			writeWorkAsText(w, work)
		case "json":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(work); err != nil {
				http.Error(w, fmt.Sprintf("Failed to encode JSON: %v", err), http.StatusInternalServerError)
				return
			}
		default:
			http.Error(w, "Invalid format parameter. Use 'json' or 'text'", http.StatusBadRequest)
			return
		}
	}
}

// ShakespertGenres handles the /api/shakespert/genres endpoint
func ShakespertGenres(service shakespertService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		format := r.URL.Query().Get("format")
		if format == "" {
			format = "json"
		}

		genres, err := service.ListGenres(ctx)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to list genres: %v", err), http.StatusInternalServerError)
			return
		}

		switch format {
		case "text", "ascii":
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			writeGenresAsText(w, genres)
		case "json":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"genres": genres,
				"count":  len(genres),
			}); err != nil {
				http.Error(w, fmt.Sprintf("Failed to encode JSON: %v", err), http.StatusInternalServerError)
				return
			}
		default:
			http.Error(w, "Invalid format parameter. Use 'json' or 'text'", http.StatusBadRequest)
			return
		}
	}
}

// writeWorksAsText formats works as plain text
func writeWorksAsText(w http.ResponseWriter, works []shakespert.WorkSummary) {
	fmt.Fprintf(w, "Shakespeare's Complete Works (%d works)\n", len(works))
	fmt.Fprintf(w, "%s\n", strings.Repeat("=", 50))
	fmt.Fprintf(w, "\n")

	currentGenre := ""
	for _, work := range works {
		if work.GenreName != currentGenre {
			if currentGenre != "" {
				fmt.Fprintf(w, "\n")
			}
			fmt.Fprintf(w, "%s:\n", work.GenreName)
			fmt.Fprintf(w, "%s\n", strings.Repeat("-", len(work.GenreName)+1))
			currentGenre = work.GenreName
		}

		yearStr := ""
		if work.Date > 0 {
			yearStr = fmt.Sprintf(" (%d)", work.Date)
		}

		fmt.Fprintf(w, "  %s - %s%s\n", work.WorkID, work.Title, yearStr)
		fmt.Fprintf(w, "    Words: %d, Paragraphs: %d\n", work.TotalWords, work.TotalParagraphs)
		fmt.Fprintf(w, "\n")
	}
}

// writeWorkAsText formats a single work as plain text
func writeWorkAsText(w http.ResponseWriter, work *shakespert.WorkDetail) {
	fmt.Fprintf(w, "%s\n", work.Title)
	fmt.Fprintf(w, "%s\n", strings.Repeat("=", len(work.Title)))
	fmt.Fprintf(w, "\n")

	if work.LongTitle != work.Title && work.LongTitle != "" {
		fmt.Fprintf(w, "Full Title: %s\n", work.LongTitle)
	}

	if work.ShortTitle != "" {
		fmt.Fprintf(w, "Short Title: %s\n", work.ShortTitle)
	}

	fmt.Fprintf(w, "Work ID: %s\n", work.WorkID)
	fmt.Fprintf(w, "Genre: %s (%s)\n", work.GenreName, work.GenreType)

	if work.Date > 0 {
		fmt.Fprintf(w, "Year: %d\n", work.Date)
	}

	fmt.Fprintf(w, "Words: %d\n", work.TotalWords)
	fmt.Fprintf(w, "Paragraphs: %d\n", work.TotalParagraphs)

	if work.Source != "" {
		fmt.Fprintf(w, "Source: %s\n", work.Source)
	}

	if work.Notes != "" && work.Notes != "null" {
		fmt.Fprintf(w, "Notes: %s\n", work.Notes)
	}
}

// writeGenresAsText formats genres as plain text
func writeGenresAsText(w http.ResponseWriter, genres []shakespert.Genre) {
	fmt.Fprintf(w, "Shakespeare Genres\n")
	fmt.Fprintf(w, "%s\n", strings.Repeat("=", 17))
	fmt.Fprintf(w, "\n")

	for _, genre := range genres {
		genreName := genre.Genrename.String
		if !genre.Genrename.Valid {
			genreName = ""
		}
		fmt.Fprintf(w, "%s - %s\n", genre.Genretype, genreName)
	}
}
