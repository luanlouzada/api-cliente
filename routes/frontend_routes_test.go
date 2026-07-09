package routes

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestFrontendRoutesServeHTMLAndAssets(t *testing.T) {
	originalWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalWorkingDirectory); err != nil {
			t.Fatalf("os.Chdir(%q) error = %v", originalWorkingDirectory, err)
		}
	})

	if err := os.Chdir(".."); err != nil {
		t.Fatalf("os.Chdir(..) error = %v", err)
	}

	router := chi.NewRouter()
	FrontendRoutes(router)

	htmlResponse := httptest.NewRecorder()
	router.ServeHTTP(htmlResponse, httptest.NewRequest(http.MethodGet, "/", nil))

	if htmlResponse.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", htmlResponse.Code, http.StatusOK)
	}
	if !strings.Contains(htmlResponse.Body.String(), "Cliente API") {
		t.Fatal("GET / body should contain frontend title")
	}

	assetResponse := httptest.NewRecorder()
	router.ServeHTTP(assetResponse, httptest.NewRequest(http.MethodGet, "/frontend/app.js", nil))

	if assetResponse.Code != http.StatusOK {
		t.Fatalf("GET /frontend/app.js status = %d, want %d", assetResponse.Code, http.StatusOK)
	}
	if !strings.Contains(assetResponse.Body.String(), "/auth/login") {
		t.Fatal("GET /frontend/app.js body should contain auth endpoint logic")
	}
}
