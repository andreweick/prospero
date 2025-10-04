package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"prospero/internal/web/handlers"
)

func TestInfo(t *testing.T) {
	t.Run("should return JSON by default when not using curl", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/info", nil)
		req.Header.Set("User-Agent", "Mozilla/5.0")
		w := httptest.NewRecorder()

		handler := handlers.Info()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, "prospero", response["service"])
		assert.Contains(t, response, "endpoints")
		assert.Contains(t, response, "notes")
	})

	t.Run("should return ASCII format when user-agent contains curl", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/info", nil)
		req.Header.Set("User-Agent", "curl/7.68.0")
		w := httptest.NewRecorder()

		handler := handlers.Info()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))

		body := w.Body.String()
		assert.Contains(t, body, "Prospero HTTP API")
		assert.Contains(t, body, "Available Endpoints:")
	})

	t.Run("should return JSON when format=json is explicitly set", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/info?format=json", nil)
		req.Header.Set("User-Agent", "curl/7.68.0")
		w := httptest.NewRecorder()

		handler := handlers.Info()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, "prospero", response["service"])
	})

	t.Run("should return text format when format=text", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/info?format=text", nil)
		w := httptest.NewRecorder()

		handler := handlers.Info()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))

		body := w.Body.String()
		assert.Contains(t, body, "Prospero HTTP API")
	})

	t.Run("should return text format when format=ascii", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/info?format=ascii", nil)
		w := httptest.NewRecorder()

		handler := handlers.Info()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))

		body := w.Body.String()
		assert.Contains(t, body, "Prospero HTTP API")
	})

	t.Run("should return error for invalid format parameter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/info?format=invalid", nil)
		w := httptest.NewRecorder()

		handler := handlers.Info()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid format parameter")
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
				req := httptest.NewRequest(http.MethodGet, "/api/info", nil)
				req.Header.Set("User-Agent", test.userAgent)
				w := httptest.NewRecorder()

				handler := handlers.Info()
				handler.ServeHTTP(w, req)

				assert.Equal(t, http.StatusOK, w.Code)

				if test.wantASCII {
					assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
					assert.Contains(t, w.Body.String(), "Prospero HTTP API")
				} else {
					assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
					var response map[string]interface{}
					err := json.NewDecoder(w.Body).Decode(&response)
					require.NoError(t, err)
				}
			})
		}
	})

	t.Run("should include all expected endpoints in JSON response", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/info?format=json", nil)
		w := httptest.NewRecorder()

		handler := handlers.Info()
		handler.ServeHTTP(w, req)

		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		endpoints := response["endpoints"].([]interface{})
		paths := make([]string, 0, len(endpoints))
		for _, ep := range endpoints {
			endpoint := ep.(map[string]interface{})
			paths = append(paths, endpoint["path"].(string))
		}

		expectedPaths := []string{
			"/health",
			"/api/info",
			"/api/topten",
			"/api/shakespert/works",
			"/api/shakespert/works/{id}",
			"/api/shakespert/genres",
		}

		for _, expected := range expectedPaths {
			assert.Contains(t, paths, expected, "missing endpoint: %s", expected)
		}
	})

	t.Run("should include all expected endpoints in text response", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/info?format=text", nil)
		w := httptest.NewRecorder()

		handler := handlers.Info()
		handler.ServeHTTP(w, req)

		body := w.Body.String()
		expectedEndpoints := []string{
			"/health",
			"/api/info",
			"/api/topten",
			"/api/shakespert/works",
			"/api/shakespert/works/{id}",
			"/api/shakespert/genres",
		}

		for _, endpoint := range expectedEndpoints {
			assert.Contains(t, body, endpoint, "missing endpoint in text: %s", endpoint)
		}
	})
}
