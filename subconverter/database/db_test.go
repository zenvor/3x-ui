package database

import (
	"os"
	"path/filepath"
	"testing"
)

// TestInitDB verifies that InitDB creates the SQLite file under XUI_DB_FOLDER
// and that AutoMigrate produces every table the module relies on.
func TestInitDB(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", tmpDir)

	// Reset package state so the test exercises a real init.
	db = nil
	t.Cleanup(func() { db = nil })

	if err := InitDB(); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "subconverter.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("expected db at %s: %v", dbPath, err)
	}

	if GetDB() == nil {
		t.Fatal("GetDB returned nil after InitDB")
	}

	tables := []string{"subscriptions", "subscription_inbounds", "ip_bindings"}
	for _, name := range tables {
		var count int64
		if err := GetDB().Table(name).Count(&count).Error; err != nil {
			t.Errorf("table %q not migrated: %v", name, err)
		}
	}
}

// TestInitDBIdempotent verifies that calling InitDB twice in a row reuses the
// existing connection and does not error.
func TestInitDBIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", tmpDir)

	db = nil
	t.Cleanup(func() { db = nil })

	if err := InitDB(); err != nil {
		t.Fatalf("first InitDB failed: %v", err)
	}
	first := GetDB()

	if err := InitDB(); err != nil {
		t.Fatalf("second InitDB failed: %v", err)
	}
	second := GetDB()

	if first != second {
		t.Fatal("InitDB should reuse the same *gorm.DB across calls")
	}
}
