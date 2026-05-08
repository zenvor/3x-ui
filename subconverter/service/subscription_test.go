package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v2/subconverter/database"
)

// setupTestDB points InitDB at a fresh per-test temp directory so the
// package-level GORM handle in subconverter/database is bound to a clean
// SQLite file with all migrations applied.
func setupTestDB(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", tmpDir)
	if err := database.Reset(); err != nil {
		t.Fatalf("reset db: %v", err)
	}
	if err := database.InitDB(); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = database.Reset() })
}

func TestSubscriptionCRUD(t *testing.T) {
	setupTestDB(t)
	svc := NewSubscriptionService()

	enabled := true
	created, err := svc.Create(SubscriptionInput{
		Remark:  "family",
		MaxIps:  3,
		Enabled: &enabled,
		Inbounds: []InboundInput{
			{InboundId: 1, ClientEmail: ""},
			{InboundId: 2, ClientEmail: "alice@x"},
		},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Id == 0 {
		t.Fatal("created subscription has zero id")
	}
	if len(created.Token) != 32 {
		t.Fatalf("token length = %d, want 32", len(created.Token))
	}
	if len(created.Inbounds) != 2 {
		t.Fatalf("created inbounds len = %d, want 2", len(created.Inbounds))
	}

	subs, err := svc.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(subs) != 1 || subs[0].Token != created.Token {
		t.Fatalf("unexpected list result: %+v", subs)
	}

	got, err := svc.Get(created.Id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Remark != "family" || got.MaxIps != 3 || !got.Enabled {
		t.Fatalf("get returned unexpected data: %+v", got)
	}
	if len(got.Inbounds) != 2 {
		t.Fatalf("get inbounds len = %d, want 2", len(got.Inbounds))
	}

	disabled := false
	updated, err := svc.Update(created.Id, SubscriptionInput{
		Remark:  "team",
		MaxIps:  5,
		Enabled: &disabled,
		Inbounds: []InboundInput{
			{InboundId: 7, ClientEmail: ""},
		},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Remark != "team" || updated.MaxIps != 5 || updated.Enabled {
		t.Fatalf("update did not persist mutable fields: %+v", updated)
	}
	if len(updated.Inbounds) != 1 || updated.Inbounds[0].InboundId != 7 {
		t.Fatalf("update did not replace inbound list: %+v", updated.Inbounds)
	}
	if updated.Token != created.Token {
		t.Fatal("update should not change token")
	}

	if err := svc.Delete(created.Id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	subs, _ = svc.List()
	if len(subs) != 0 {
		t.Fatalf("expected empty list after delete, got %d", len(subs))
	}
}

func TestCreateRequiresInbounds(t *testing.T) {
	setupTestDB(t)
	svc := NewSubscriptionService()

	if _, err := svc.Create(SubscriptionInput{Remark: "x"}); err == nil {
		t.Fatal("expected error when input has no inbounds")
	}
}

func TestGetMissingReturnsNotFound(t *testing.T) {
	setupTestDB(t)
	svc := NewSubscriptionService()

	if _, err := svc.Get(9999); err != ErrSubscriptionNotFound {
		t.Fatalf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

func TestTokensAreUnique(t *testing.T) {
	setupTestDB(t)
	svc := NewSubscriptionService()

	seen := make(map[string]struct{}, 20)
	for i := 0; i < 20; i++ {
		sub, err := svc.Create(SubscriptionInput{
			Remark:   "n",
			Inbounds: []InboundInput{{InboundId: 1}},
		})
		if err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
		if _, dup := seen[sub.Token]; dup {
			t.Fatalf("duplicate token at iteration %d: %s", i, sub.Token)
		}
		seen[sub.Token] = struct{}{}
	}
}
