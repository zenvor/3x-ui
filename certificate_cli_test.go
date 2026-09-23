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

func TestShowSettingFailsWhenSettingsCannotBeRead(t *testing.T) {
	newTokenCLIEnv(t)

	if err := database.CloseDB(); err != nil {
		t.Fatalf("close db: %v", err)
	}
	if err := showSetting(true); err == nil {
		t.Fatal("showSetting succeeded after the settings database was closed")
	}
}

func TestUpdateSettingPropagatesCredentialErrors(t *testing.T) {
	newTokenCLIEnv(t)

	if err := updateSetting(0, "", "password", "", "", false); err == nil {
		t.Fatal("updateSetting succeeded with an empty username")
	}
}

func TestUpdateCertRejectsIncompletePair(t *testing.T) {
	newTokenCLIEnv(t)

	if err := updateCert("/root/cert/fullchain.pem", ""); err == nil {
		t.Fatal("updateCert succeeded with an incomplete certificate pair")
	}
}
