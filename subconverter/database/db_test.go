package database

import (
	"os"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/subconverter/model"
)

// TestInitDB verifies that InitDB creates the SQLite file under XUI_DB_FOLDER
// and that AutoMigrate produces every table the module relies on.
func TestInitDB(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", tmpDir)

	// Reset package state so the test exercises a real init.
	if err := Reset(); err != nil {
		t.Fatalf("reset db: %v", err)
	}
	t.Cleanup(func() { _ = Reset() })

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

	tables := []string{
		"subscriptions",
		"subscription_inbounds",
		"ip_bindings",
		"subscription_stats",
		"access_logs",
		"settings",
	}
	for _, name := range tables {
		var count int64
		if err := GetDB().Table(name).Count(&count).Error; err != nil {
			t.Errorf("table %q not migrated: %v", name, err)
		}
	}
	if GetDB().Migrator().HasTable("subscription_access_sessions") {
		t.Fatal("obsolete subscription_access_sessions table should not be migrated")
	}
}

// TestInitDBIdempotent verifies that calling InitDB twice in a row reuses the
// existing connection and does not error.
func TestInitDBIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", tmpDir)

	if err := Reset(); err != nil {
		t.Fatalf("reset db: %v", err)
	}
	t.Cleanup(func() { _ = Reset() })

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

func TestInitDBMigratesLegacySettingsWithoutChangingUA(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	if err := Reset(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Reset() })
	conn, err := gorm.Open(sqlite.Open(GetDBPath()), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Exec(`CREATE TABLE settings (id integer PRIMARY KEY AUTOINCREMENT, ua_filter_enabled numeric DEFAULT true, ua_keywords text NOT NULL, ua_reject_status integer DEFAULT 403, created_at datetime, updated_at datetime)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := conn.Exec(`INSERT INTO settings (id, ua_filter_enabled, ua_keywords, ua_reject_status) VALUES (1, 0, 'custom-client', 404)`).Error; err != nil {
		t.Fatal(err)
	}
	sqlDB, err := conn.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	var row model.Settings
	if err := GetDB().First(&row, 1).Error; err != nil {
		t.Fatal(err)
	}
	if row.TemplateVariant != "general" || row.UAFilterEnabled || row.UAKeywords != "custom-client" || row.UARejectStatus != 404 {
		t.Fatalf("migration changed legacy settings: %+v", row)
	}
	if !GetDB().Migrator().HasTable(&model.Template{}) {
		t.Fatal("migration did not create the custom template catalog")
	}
}
