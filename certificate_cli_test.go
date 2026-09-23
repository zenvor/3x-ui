package main

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
)

func TestGetCertificateFailsWhenSettingsCannotBeRead(t *testing.T) {
	newTokenCLIEnv(t)

	if err := database.CloseDB(); err != nil {
		t.Fatalf("close db: %v", err)
	}
	if err := GetCertificate(true); err == nil {
		t.Fatal("GetCertificate succeeded after the settings database was closed")
	}
}
