package service

import "testing"

func TestSettingsDefaultsAndUpdate(t *testing.T) {
	setupTestDB(t)
	svc := NewSettingsService()

	defaults, err := svc.Get()
	if err != nil {
		t.Fatalf("get defaults: %v", err)
	}
	if !defaults.UAFilterEnabled {
		t.Fatal("UA filter should be enabled by default")
	}
	if defaults.UARejectStatus != 403 {
		t.Fatalf("reject status = %d, want 403", defaults.UARejectStatus)
	}
	for _, keyword := range []string{"clash", "mihomo", "shadowrocket"} {
		if !containsString(defaults.UAKeywords, keyword) {
			t.Fatalf("default keywords missing %q: %#v", keyword, defaults.UAKeywords)
		}
	}

	updated, err := svc.Update(SettingsInput{
		UAFilterEnabled: true,
		UAKeywords:      []string{" Clash ", "MIHOMO", "clash", "shadowrocket,stash"},
		UARejectStatus:  404,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	want := []string{"clash", "mihomo", "shadowrocket", "stash"}
	for i := range want {
		if updated.UAKeywords[i] != want[i] {
			t.Fatalf("keywords = %#v, want %#v", updated.UAKeywords, want)
		}
	}
	if updated.UARejectStatus != 404 {
		t.Fatalf("reject status = %d, want 404", updated.UARejectStatus)
	}
	if !IsUserAgentAllowed("ClashMetaForAndroid/2.11.23.Meta", updated) {
		t.Fatal("expected ClashMetaForAndroid UA to be allowed")
	}
	if IsUserAgentAllowed("Mozilla/5.0", updated) {
		t.Fatal("expected browser UA to be rejected")
	}
}

func TestSettingsRequiresKeywordsWhenEnabled(t *testing.T) {
	setupTestDB(t)
	_, err := NewSettingsService().Update(SettingsInput{UAFilterEnabled: true})
	if err != ErrUAKeywordsRequired {
		t.Fatalf("err = %v, want ErrUAKeywordsRequired", err)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
