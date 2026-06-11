// Package database opens and migrates the subconverter module's own SQLite file.
//
// The subconverter module persists into subconverter.db under the configured
// 3X-UI database folder. Keeping the file separate means upstream merges never
// touch our schema and 3X-UI's getDb/importDB cannot accidentally overwrite or
// leak our data.
package database

import (
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/mhsanaei/3x-ui/v3/config"
	"github.com/mhsanaei/3x-ui/v3/subconverter/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const dbFileName = "subconverter.db"

var db *gorm.DB

// GetDBPath returns the absolute path to the subconverter SQLite file.
func GetDBPath() string {
	return filepath.Join(config.GetDBFolderPath(), dbFileName)
}

// InitDB opens the subconverter SQLite file and runs AutoMigrate for every
// model. Safe to call multiple times: subsequent calls reuse the existing
// connection.
func InitDB() error {
	if db != nil {
		return nil
	}

	dbPath := GetDBPath()
	if err := os.MkdirAll(filepath.Dir(dbPath), os.ModePerm); err != nil {
		return err
	}

	cfg := &gorm.Config{}
	if config.IsDebug() {
		cfg.Logger = logger.Default.LogMode(logger.Info)
	} else {
		cfg.Logger = logger.New(log.New(io.Discard, "", 0), logger.Config{})
	}

	dsn := dbPath + "?_journal_mode=WAL&_busy_timeout=10000&_synchronous=NORMAL&_txlock=immediate"
	conn, err := gorm.Open(sqlite.Open(dsn), cfg)
	if err != nil {
		return err
	}
	db = conn
	initialized := false
	defer func() {
		if !initialized {
			_ = Reset()
		}
	}()

	if err := dropObsoleteTables(); err != nil {
		return err
	}
	if err := autoMigrate(); err != nil {
		return err
	}
	initialized = true
	return nil
}

// GetDB returns the package-level GORM handle. Returns nil if InitDB has not
// been called.
func GetDB() *gorm.DB {
	return db
}

// Reset closes the package-level connection and clears it. Intended for tests
// that need to drive InitDB against a fresh XUI_DB_FOLDER.
func Reset() error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err == nil && sqlDB != nil {
		_ = sqlDB.Close()
	}
	db = nil
	return nil
}

func autoMigrate() error {
	models := []any{
		&model.Subscription{},
		&model.SubscriptionInbound{},
		&model.IpBinding{},
		&model.AccessLog{},
		&model.SubscriptionStats{},
		&model.Settings{},
	}
	for _, m := range models {
		if err := db.AutoMigrate(m); err != nil {
			log.Printf("subconverter: auto migrate failed: %v", err)
			return err
		}
	}
	return nil
}

func dropObsoleteTables() error {
	const accessSessions = "subscription_access_sessions"
	if !db.Migrator().HasTable(accessSessions) {
		return nil
	}
	return db.Migrator().DropTable(accessSessions)
}
