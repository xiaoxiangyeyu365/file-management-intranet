package handler

import (
	"cloudbox/internal/util/crypto"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func newJWTTestRouter(t *testing.T) (*gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	token, err := crypto.GenerateToken(1, "testuser", "user", 0, true)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	r := gin.New()
	r.GET("/protected", JWTMiddleware(), func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	return r, token
}

func TestJWTMiddlewareWithAuthorizationHeader(t *testing.T) {
	r, token := newJWTTestRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with Authorization header, got %d: %s", w.Code, w.Body.String())
	}
}

func TestJWTMiddlewareWithQueryToken(t *testing.T) {
	r, token := newJWTTestRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected?token="+token, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with query token, got %d: %s", w.Code, w.Body.String())
	}
}

func TestJWTMiddlewareWithoutToken(t *testing.T) {
	r, _ := newJWTTestRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", w.Code)
	}
}

func TestJWTMiddlewareWithInvalidToken(t *testing.T) {
	r, _ := newJWTTestRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected?token=not-a-valid-token", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with invalid token, got %d", w.Code)
	}
}
