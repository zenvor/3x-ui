package controller

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"

	paneldatabase "github.com/mhsanaei/3x-ui/v3/internal/database"
)

func newCheckAPIAuthTestEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := paneldatabase.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("init panel database: %v", err)
	}
	t.Cleanup(func() { _ = paneldatabase.CloseDB() })

	engine := gin.New()
	engine.Use(sessions.Sessions("3x-ui", cookie.NewStore([]byte("subconverter-api-auth-test-secret"))))
	api := engine.Group("/panel/api/subconverter")
	api.Use(CheckAPIAuth)
	api.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })
	return engine
}

func TestCheckAPIAuthRejectsInvalidBearerWithUnauthorized(t *testing.T) {
	engine := newCheckAPIAuthTestEngine(t)

	cases := []struct {
		name string
		auth string
		xhr  bool
		want int
	}{
		{name: "anonymous request remains hidden", want: http.StatusNotFound},
		{name: "xhr request is unauthorized", xhr: true, want: http.StatusUnauthorized},
		{name: "invalid bearer is unauthorized", auth: "Bearer invalid-token", want: http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/panel/api/subconverter/ping", nil)
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}
			if tc.xhr {
				req.Header.Set("X-Requested-With", "XMLHttpRequest")
			}
			resp := httptest.NewRecorder()
			engine.ServeHTTP(resp, req)
			if resp.Code != tc.want {
				t.Fatalf("status = %d, want %d", resp.Code, tc.want)
			}
		})
	}
}
