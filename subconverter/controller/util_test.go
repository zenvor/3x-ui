package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"

	paneldatabase "github.com/mhsanaei/3x-ui/v3/internal/database"
	panelmodel "github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/middleware"
	panelservice "github.com/mhsanaei/3x-ui/v3/internal/web/service/panel"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"
	"github.com/mhsanaei/3x-ui/v3/subconverter/database"
	"github.com/mhsanaei/3x-ui/v3/subconverter/service"
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

func newAuthenticatedSubscriptionTestEngine(t *testing.T) *gin.Engine {
	t.Helper()
	engine := newCheckAPIAuthTestEngine(t)
	if err := database.Reset(); err != nil {
		t.Fatalf("reset converter database: %v", err)
	}
	if err := database.InitDB(); err != nil {
		t.Fatalf("init converter database: %v", err)
	}
	t.Cleanup(func() { _ = database.Reset() })
	api := engine.Group("/panel/api/subconverter")
	api.Use(CheckAPIAuth, middleware.CSRFMiddleware())
	NewSubscriptionController(api)
	return engine
}

func TestCheckAPIAuthEnforcesAdminScopeOnManagementEndpoints(t *testing.T) {
	cases := []struct {
		name  string
		scope string
		want  int
	}{
		{name: "admin", scope: panelmodel.ApiScopeAdmin, want: http.StatusOK},
		{name: "default admin", want: http.StatusOK},
		{name: "monitor", scope: panelmodel.ApiScopeMonitor, want: http.StatusForbidden},
		{name: "node sync", scope: panelmodel.ApiScopeNodeSync, want: http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			engine := newAuthenticatedSubscriptionTestEngine(t)
			token, err := (&panelservice.ApiTokenService{}).Create(tc.name, tc.scope, 0)
			if err != nil {
				t.Fatalf("create token: %v", err)
			}
			for _, method := range []string{http.MethodGet, http.MethodPost} {
				req := httptest.NewRequest(method, "/panel/api/subconverter/settings", strings.NewReader("uaRejectStatus=404"))
				req.Header.Set("Authorization", "Bearer "+token.Token)
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				resp := httptest.NewRecorder()
				engine.ServeHTTP(resp, req)
				if resp.Code != tc.want {
					t.Fatalf("%s status = %d, want %d", method, resp.Code, tc.want)
				}
				if tc.want == http.StatusOK {
					var payload struct {
						Success bool `json:"success"`
					}
					if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil || !payload.Success {
						t.Fatalf("%s failed: %s", method, resp.Body.String())
					}
				}
			}
			settings, err := service.NewSettingsService().Get()
			if err != nil {
				t.Fatalf("get settings: %v", err)
			}
			want := http.StatusForbidden
			if tc.want == http.StatusOK {
				want = http.StatusNotFound
			}
			if settings.UARejectStatus != want {
				t.Fatalf("persisted reject status = %d, want %d", settings.UARejectStatus, want)
			}
		})
	}
}

func TestCheckAPIAuthRejectsDisabledAndExpiredTokens(t *testing.T) {
	for _, state := range []string{"disabled", "expired"} {
		t.Run(state, func(t *testing.T) {
			engine := newAuthenticatedSubscriptionTestEngine(t)
			svc := &panelservice.ApiTokenService{}
			token, err := svc.Create(state, panelmodel.ApiScopeAdmin, 0)
			if err != nil {
				t.Fatalf("create token: %v", err)
			}
			if state == "disabled" {
				err = svc.SetEnabled(token.Id, false)
			} else {
				err = paneldatabase.GetDB().Model(&panelmodel.ApiToken{}).Where("id = ?", token.Id).
					Update("expires_at", time.Now().Add(-time.Minute).UnixMilli()).Error
			}
			if err != nil {
				t.Fatalf("invalidate token: %v", err)
			}
			req := httptest.NewRequest(http.MethodGet, "/panel/api/subconverter/settings", nil)
			req.Header.Set("Authorization", "Bearer "+token.Token)
			resp := httptest.NewRecorder()
			engine.ServeHTTP(resp, req)
			if resp.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", resp.Code)
			}
		})
	}
}

func TestCheckAPIAuthPreservesSessionCSRFProtection(t *testing.T) {
	engine := newAuthenticatedSubscriptionTestEngine(t)
	user, err := (&panelservice.UserService{}).GetFirstUser()
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	var csrfToken string
	engine.GET("/test/login", func(c *gin.Context) {
		if err := session.SetLoginUser(c, user); err != nil {
			t.Fatalf("set login user: %v", err)
		}
		var err error
		csrfToken, err = session.EnsureCSRFToken(c)
		if err != nil {
			t.Fatalf("create CSRF token: %v", err)
		}
		c.Status(http.StatusOK)
	})
	loginResp := httptest.NewRecorder()
	engine.ServeHTTP(loginResp, httptest.NewRequest(http.MethodGet, "/test/login", nil))
	loginCookies := loginResp.Result().Cookies()
	if len(loginCookies) == 0 || csrfToken == "" {
		t.Fatal("login did not establish a session and CSRF token")
	}
	monitor, err := (&panelservice.ApiTokenService{}).Create("monitor", panelmodel.ApiScopeMonitor, 0)
	if err != nil {
		t.Fatalf("create monitor token: %v", err)
	}
	for _, tc := range []struct {
		name string
		csrf string
		auth string
		want int
	}{
		{name: "missing CSRF", want: http.StatusForbidden},
		{name: "valid session and CSRF", csrf: csrfToken, want: http.StatusOK},
		{name: "session cannot elevate monitor token", csrf: csrfToken, auth: "Bearer " + monitor.Token, want: http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/panel/api/subconverter/settings", strings.NewReader("uaRejectStatus=404"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set(session.CSRFHeaderName, tc.csrf)
			req.Header.Set("Authorization", tc.auth)
			// Both session helpers save a cookie; browsers retain the latest one.
			req.AddCookie(loginCookies[len(loginCookies)-1])
			resp := httptest.NewRecorder()
			engine.ServeHTTP(resp, req)
			if resp.Code != tc.want {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, tc.want, resp.Body.String())
			}
		})
	}
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
