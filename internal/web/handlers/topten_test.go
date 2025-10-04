package handlers_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"prospero/internal/features/topten"
	"prospero/internal/web/handlers"
)

type mockTopTenService struct {
	list *topten.TopTenList
	err  error
}

func (m *mockTopTenService) GetRandomList() (*topten.TopTenList, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.list, nil
}

func TestTopTen(t *testing.T) {
	sampleList := &topten.TopTenList{
		Date:  "2023-05-15",
		Title: "Top Ten Things Overheard at the White House",
		Items: []string{
			"Item 10",
			"Item 9",
			"Item 8",
			"Item 7",
			"Item 6",
			"Item 5",
			"Item 4",
			"Item 3",
			"Item 2",
			"Item 1",
		},
		Year: 2023,
		Show: "Late Show with Dave's",
		URL:  "http://example.com/list123",
	}

	t.Run("should return JSON by default when not using curl", func(t *testing.T) {
		service := &mockTopTenService{list: sampleList}
		req := httptest.NewRequest(http.MethodGet, "/api/topten", nil)
		req.Header.Set("User-Agent", "Mozilla/5.0")
		w := httptest.NewRecorder()

		handler := handlers.TopTen(service)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response topten.TopTenList
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, sampleList.Title, response.Title)
		assert.Equal(t, sampleList.Date, response.Date)
		assert.Equal(t, sampleList.Items, response.Items)
	})

	t.Run("should return ASCII format when user-agent contains curl", func(t *testing.T) {
		service := &mockTopTenService{list: sampleList}
		req := httptest.NewRequest(http.MethodGet, "/api/topten", nil)
		req.Header.Set("User-Agent", "curl/7.68.0")
		w := httptest.NewRecorder()

		handler := handlers.TopTen(service)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))

		body := w.Body.String()
		assert.Contains(t, body, sampleList.Title)
		assert.Contains(t, body, "Item 10")
		assert.Contains(t, body, "Item 1")
	})

	t.Run("should return JSON when format=json is explicitly set", func(t *testing.T) {
		service := &mockTopTenService{list: sampleList}
		req := httptest.NewRequest(http.MethodGet, "/api/topten?format=json", nil)
		req.Header.Set("User-Agent", "curl/7.68.0")
		w := httptest.NewRecorder()

		handler := handlers.TopTen(service)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response topten.TopTenList
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, sampleList.Title, response.Title)
	})

	t.Run("should return ASCII format when format=ascii", func(t *testing.T) {
		service := &mockTopTenService{list: sampleList}
		req := httptest.NewRequest(http.MethodGet, "/api/topten?format=ascii", nil)
		w := httptest.NewRecorder()

		handler := handlers.TopTen(service)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))

		body := w.Body.String()
		assert.Contains(t, body, sampleList.Title)
	})

	t.Run("should return error when format parameter is invalid", func(t *testing.T) {
		service := &mockTopTenService{list: sampleList}
		req := httptest.NewRequest(http.MethodGet, "/api/topten?format=invalid", nil)
		w := httptest.NewRecorder()

		handler := handlers.TopTen(service)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid format parameter")
	})

	t.Run("should return internal server error when service fails", func(t *testing.T) {
		service := &mockTopTenService{err: errors.New("database connection failed")}
		req := httptest.NewRequest(http.MethodGet, "/api/topten", nil)
		w := httptest.NewRecorder()

		handler := handlers.TopTen(service)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "Failed to get random list")
	})

	t.Run("should detect curl in various user-agent strings", func(t *testing.T) {
		tests := []struct {
			name      string
			userAgent string
			wantASCII bool
		}{
			{name: "curl lowercase", userAgent: "curl/7.68.0", wantASCII: true},
			{name: "curl uppercase", userAgent: "CURL/7.68.0", wantASCII: true},
			{name: "curl mixed case", userAgent: "CuRl/7.68.0", wantASCII: true},
			{name: "browser", userAgent: "Mozilla/5.0", wantASCII: false},
			{name: "empty user-agent", userAgent: "", wantASCII: false},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				service := &mockTopTenService{list: sampleList}
				req := httptest.NewRequest(http.MethodGet, "/api/topten", nil)
				req.Header.Set("User-Agent", test.userAgent)
				w := httptest.NewRecorder()

				handler := handlers.TopTen(service)
				handler.ServeHTTP(w, req)

				assert.Equal(t, http.StatusOK, w.Code)

				if test.wantASCII {
					assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
					assert.Contains(t, w.Body.String(), sampleList.Title)
				} else {
					assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
					var response topten.TopTenList
					err := json.NewDecoder(w.Body).Decode(&response)
					require.NoError(t, err)
				}
			})
		}
	})

	t.Run("should include all list fields in JSON response", func(t *testing.T) {
		service := &mockTopTenService{list: sampleList}
		req := httptest.NewRequest(http.MethodGet, "/api/topten?format=json", nil)
		w := httptest.NewRecorder()

		handler := handlers.TopTen(service)
		handler.ServeHTTP(w, req)

		var response topten.TopTenList
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, sampleList.Date, response.Date)
		assert.Equal(t, sampleList.Title, response.Title)
		assert.Equal(t, sampleList.Items, response.Items)
		assert.Equal(t, sampleList.Year, response.Year)
		assert.Equal(t, sampleList.Show, response.Show)
		assert.Equal(t, sampleList.URL, response.URL)
	})
}

func TestHealth(t *testing.T) {
	t.Run("should return healthy status with JSON response", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()

		handler := handlers.Health()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response map[string]string
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, "healthy", response["status"])
		assert.Equal(t, "prospero", response["service"])
	})
}
