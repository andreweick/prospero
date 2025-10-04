package handlers_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"prospero/internal/features/shakespert"
	"prospero/internal/web/handlers"
)

type mockShakespertService struct {
	works       []shakespert.WorkSummary
	work        *shakespert.WorkDetail
	genres      []shakespert.Genre
	listErr     error
	getErr      error
	genresErr   error
	byGenreWorks []shakespert.WorkSummary
	byGenreErr  error
}

func (m *mockShakespertService) ListWorks(ctx context.Context) ([]shakespert.WorkSummary, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.works, nil
}

func (m *mockShakespertService) GetWork(ctx context.Context, workID string) (*shakespert.WorkDetail, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.work, nil
}

func (m *mockShakespertService) ListGenres(ctx context.Context) ([]shakespert.Genre, error) {
	if m.genresErr != nil {
		return nil, m.genresErr
	}
	return m.genres, nil
}

func (m *mockShakespertService) GetWorksByGenre(ctx context.Context, genreType string) ([]shakespert.WorkSummary, error) {
	if m.byGenreErr != nil {
		return nil, m.byGenreErr
	}
	return m.byGenreWorks, nil
}

func TestShakespertWorks(t *testing.T) {
	sampleWorks := []shakespert.WorkSummary{
		{
			WorkID:          "hamlet",
			Title:           "Hamlet",
			LongTitle:       "The Tragedy of Hamlet, Prince of Denmark",
			Date:            1600,
			GenreType:       "t",
			GenreName:       "Tragedy",
			TotalWords:      30000,
			TotalParagraphs: 500,
		},
		{
			WorkID:          "macbeth",
			Title:           "Macbeth",
			LongTitle:       "The Tragedy of Macbeth",
			Date:            1606,
			GenreType:       "t",
			GenreName:       "Tragedy",
			TotalWords:      18000,
			TotalParagraphs: 300,
		},
	}

	t.Run("should list all works in JSON format", func(t *testing.T) {
		service := &mockShakespertService{works: sampleWorks}
		req := httptest.NewRequest(http.MethodGet, "/api/shakespert/works?format=json", nil)
		w := httptest.NewRecorder()

		handler := handlers.ShakespertWorks(service)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, float64(2), response["count"])
		works := response["works"].([]interface{})
		assert.Len(t, works, 2)
	})

	t.Run("should list all works in text format", func(t *testing.T) {
		service := &mockShakespertService{works: sampleWorks}
		req := httptest.NewRequest(http.MethodGet, "/api/shakespert/works?format=text", nil)
		w := httptest.NewRecorder()

		handler := handlers.ShakespertWorks(service)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))

		body := w.Body.String()
		assert.Contains(t, body, "Hamlet")
		assert.Contains(t, body, "Macbeth")
		assert.Contains(t, body, "Tragedy")
	})

	t.Run("should filter works by genre", func(t *testing.T) {
		filteredWorks := []shakespert.WorkSummary{sampleWorks[0]}
		service := &mockShakespertService{byGenreWorks: filteredWorks}
		req := httptest.NewRequest(http.MethodGet, "/api/shakespert/works?genre=t&format=json", nil)
		w := httptest.NewRecorder()

		handler := handlers.ShakespertWorks(service)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, float64(1), response["count"])
	})

	t.Run("should return error when service fails", func(t *testing.T) {
		service := &mockShakespertService{listErr: errors.New("database error")}
		req := httptest.NewRequest(http.MethodGet, "/api/shakespert/works", nil)
		w := httptest.NewRecorder()

		handler := handlers.ShakespertWorks(service)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "Failed to list works")
	})

	t.Run("should return error when genre filter fails", func(t *testing.T) {
		service := &mockShakespertService{byGenreErr: errors.New("database error")}
		req := httptest.NewRequest(http.MethodGet, "/api/shakespert/works?genre=t", nil)
		w := httptest.NewRecorder()

		handler := handlers.ShakespertWorks(service)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "Failed to get works by genre")
	})

	t.Run("should return error for invalid format", func(t *testing.T) {
		service := &mockShakespertService{works: sampleWorks}
		req := httptest.NewRequest(http.MethodGet, "/api/shakespert/works?format=invalid", nil)
		w := httptest.NewRecorder()

		handler := handlers.ShakespertWorks(service)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid format parameter")
	})
}

func TestShakespertWork(t *testing.T) {
	sampleWork := &shakespert.WorkDetail{
		WorkID:          "hamlet",
		Title:           "Hamlet",
		LongTitle:       "The Tragedy of Hamlet, Prince of Denmark",
		ShortTitle:      "Hamlet",
		Date:            1600,
		GenreType:       "t",
		GenreName:       "Tragedy",
		Notes:           "One of Shakespeare's most famous tragedies",
		Source:          "First Folio",
		TotalWords:      30000,
		TotalParagraphs: 500,
	}

	t.Run("should get specific work in JSON format", func(t *testing.T) {
		service := &mockShakespertService{work: sampleWork}
		req := httptest.NewRequest(http.MethodGet, "/api/shakespert/works/hamlet?format=json", nil)
		w := httptest.NewRecorder()

		handler := handlers.ShakespertWork(service)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response shakespert.WorkDetail
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, "hamlet", response.WorkID)
		assert.Equal(t, "Hamlet", response.Title)
		assert.Equal(t, int64(1600), response.Date)
	})

	t.Run("should get specific work in text format", func(t *testing.T) {
		service := &mockShakespertService{work: sampleWork}
		req := httptest.NewRequest(http.MethodGet, "/api/shakespert/works/hamlet?format=text", nil)
		w := httptest.NewRecorder()

		handler := handlers.ShakespertWork(service)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))

		body := w.Body.String()
		assert.Contains(t, body, "Hamlet")
		assert.Contains(t, body, "Tragedy")
		assert.Contains(t, body, "1600")
	})

	t.Run("should return 404 when work not found", func(t *testing.T) {
		service := &mockShakespertService{getErr: errors.New("work not found: unknown")}
		req := httptest.NewRequest(http.MethodGet, "/api/shakespert/works/unknown", nil)
		w := httptest.NewRecorder()

		handler := handlers.ShakespertWork(service)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "Work not found")
	})

	t.Run("should return 500 when service fails", func(t *testing.T) {
		service := &mockShakespertService{getErr: errors.New("database error")}
		req := httptest.NewRequest(http.MethodGet, "/api/shakespert/works/hamlet", nil)
		w := httptest.NewRecorder()

		handler := handlers.ShakespertWork(service)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "Failed to get work")
	})

	t.Run("should return error when work ID is missing", func(t *testing.T) {
		service := &mockShakespertService{work: sampleWork}
		req := httptest.NewRequest(http.MethodGet, "/api/shakespert/works/", nil)
		w := httptest.NewRecorder()

		handler := handlers.ShakespertWork(service)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Work ID is required")
	})

	t.Run("should return error for invalid format", func(t *testing.T) {
		service := &mockShakespertService{work: sampleWork}
		req := httptest.NewRequest(http.MethodGet, "/api/shakespert/works/hamlet?format=invalid", nil)
		w := httptest.NewRecorder()

		handler := handlers.ShakespertWork(service)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid format parameter")
	})
}

func TestShakespertGenres(t *testing.T) {
	sampleGenres := []shakespert.Genre{
		{
			Genretype: "t",
			Genrename: sql.NullString{String: "Tragedy", Valid: true},
		},
		{
			Genretype: "c",
			Genrename: sql.NullString{String: "Comedy", Valid: true},
		},
	}

	t.Run("should list genres in JSON format", func(t *testing.T) {
		service := &mockShakespertService{genres: sampleGenres}
		req := httptest.NewRequest(http.MethodGet, "/api/shakespert/genres?format=json", nil)
		w := httptest.NewRecorder()

		handler := handlers.ShakespertGenres(service)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, float64(2), response["count"])
		genres := response["genres"].([]interface{})
		assert.Len(t, genres, 2)
	})

	t.Run("should list genres in text format", func(t *testing.T) {
		service := &mockShakespertService{genres: sampleGenres}
		req := httptest.NewRequest(http.MethodGet, "/api/shakespert/genres?format=text", nil)
		w := httptest.NewRecorder()

		handler := handlers.ShakespertGenres(service)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))

		body := w.Body.String()
		assert.Contains(t, body, "Tragedy")
		assert.Contains(t, body, "Comedy")
	})

	t.Run("should return error when service fails", func(t *testing.T) {
		service := &mockShakespertService{genresErr: errors.New("database error")}
		req := httptest.NewRequest(http.MethodGet, "/api/shakespert/genres", nil)
		w := httptest.NewRecorder()

		handler := handlers.ShakespertGenres(service)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "Failed to list genres")
	})

	t.Run("should return error for invalid format", func(t *testing.T) {
		service := &mockShakespertService{genres: sampleGenres}
		req := httptest.NewRequest(http.MethodGet, "/api/shakespert/genres?format=invalid", nil)
		w := httptest.NewRecorder()

		handler := handlers.ShakespertGenres(service)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid format parameter")
	})
}
