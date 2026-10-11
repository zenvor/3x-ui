package sub

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestAllExcludedInboundsRemainKnownWithoutLinks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	seedSubDB(t)
	db := database.GetDB()
	const subID = "all-excluded"
	for i, tag := range []string{"hidden-one", "hidden-two"} {
		ib := seedSubInbound(t, subID, tag, 24401+i, i, `{"network":"tcp","security":"none"}`)
		if err := db.Model(ib).Update("exclude_from_sub", true).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&xray.ClientTraffic{InboundId: ib.Id, Email: tag + "@e", Up: 100, Down: 200, Enable: true}).Error; err != nil {
			t.Fatal(err)
		}
	}
	links, _, _, traffic, err := NewSubService("").GetSubs(subID, "sub.example.com")
	if err != nil || links == nil || len(links) != 0 || traffic.Up != 200 || traffic.Down != 400 {
		t.Fatalf("links=%v usage=%+v err=%v, want a known empty subscription with both clients' usage", links, traffic, err)
	}
	oldDistFS := distFS
	distFS = testDistFS
	t.Cleanup(func() { distFS = oldDistFS })
	router := gin.New()
	NewSUBController(router.Group("/"), WithSUBEncryption(false))
	for _, accept := range []string{"text/plain", "text/html"} {
		req := httptest.NewRequest(http.MethodGet, "/sub/"+subID, nil)
		req.Header.Set("Accept", accept)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status=%d body=%s", accept, w.Code, w.Body.String())
		}
		if accept == "text/plain" {
			if w.Body.Len() != 0 || !strings.HasPrefix(w.Header().Get("Subscription-Userinfo"), "upload=200; download=400;") {
				t.Fatalf("raw response body=%q usage=%q", w.Body.String(), w.Header().Get("Subscription-Userinfo"))
			}
		} else if !strings.Contains(w.Body.String(), `"links":[]`) || strings.Contains(w.Body.String(), "vless://") {
			t.Fatalf("HTML must show no links: %s", w.Body.String())
		}
	}
	var rows []xray.ClientTraffic
	if err := db.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.LastSubFetch == 0 {
			t.Fatalf("client %s did not record subscription fetch", row.Email)
		}
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sub/missing-subscription", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("missing subscription status=%d, want 404", w.Code)
	}
}

// An excluded inbound keeps serving its clients, so every renderer must drop
// its links yet still count its clients' usage in the Subscription-Userinfo header.
func TestExcludedInboundHidesLinksButKeepsUsage(t *testing.T) {
	seedSubDB(t)
	db := database.GetDB()

	shown := seedSubInbound(t, "sub-excl", "shown", 24401, 1, `{"network":"tcp","security":"none"}`)
	hidden := seedSubInbound(t, "sub-excl", "hidden", 24402, 2, `{"network":"tcp","security":"none"}`)
	if err := db.Model(hidden).Update("exclude_from_sub", true).Error; err != nil {
		t.Fatalf("mark excluded: %v", err)
	}
	for _, row := range []*xray.ClientTraffic{
		{InboundId: shown.Id, Email: "shown@e", Up: 100, Down: 200, Enable: true},
		{InboundId: hidden.Id, Email: "hidden@e", Up: 1000, Down: 2000, Enable: true},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("seed traffic %s: %v", row.Email, err)
		}
	}

	const wantHeader = "upload=1100; download=2200; "
	assertOnlyShown := func(t *testing.T, out string) {
		t.Helper()
		if !strings.Contains(out, "24401") {
			t.Fatalf("output lost the shown inbound:\n%s", out)
		}
		if strings.Contains(out, "24402") {
			t.Fatalf("output leaked the excluded inbound:\n%s", out)
		}
	}

	t.Run("raw", func(t *testing.T) {
		links, _, _, traffic, err := NewSubService("").GetSubs("sub-excl", "req.example.com")
		if err != nil {
			t.Fatalf("GetSubs: %v", err)
		}
		if len(links) != 1 {
			t.Fatalf("links = %q, want only the shown inbound's link", links)
		}
		assertOnlyShown(t, links[0])
		if traffic.Up != 1100 || traffic.Down != 2200 {
			t.Fatalf("usage = up %d/down %d, want 1100/2200 including the excluded inbound's client", traffic.Up, traffic.Down)
		}
	})

	t.Run("clash", func(t *testing.T) {
		out, header, err := NewSubClashService(false, "", NewSubService("")).GetClash("sub-excl", "req.example.com")
		if err != nil {
			t.Fatalf("GetClash: %v", err)
		}
		assertOnlyShown(t, out)
		if !strings.HasPrefix(header, wantHeader) {
			t.Fatalf("header = %q, want prefix %q", header, wantHeader)
		}
	})

	t.Run("json", func(t *testing.T) {
		out, header, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson("sub-excl", "req.example.com", false)
		if err != nil {
			t.Fatalf("GetJson: %v", err)
		}
		assertOnlyShown(t, out)
		if !strings.HasPrefix(header, wantHeader) {
			t.Fatalf("header = %q, want prefix %q", header, wantHeader)
		}
	})
}
