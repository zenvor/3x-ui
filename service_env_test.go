package main

import "testing"

func TestParseServiceEnvFilePreservesSystemdDSNValues(t *testing.T) {
	values := parseServiceEnvFile("# panel database\nXUI_DB_TYPE=postgres\nXUI_DB_DSN=host=127.0.0.1 password=pa$WORD application_name=x-ui\n")
	if got, want := values["XUI_DB_TYPE"], "postgres"; got != want {
		t.Fatalf("XUI_DB_TYPE = %q, want %q", got, want)
	}
	if got, want := values["XUI_DB_DSN"], "host=127.0.0.1 password=pa$WORD application_name=x-ui"; got != want {
		t.Fatalf("XUI_DB_DSN = %q, want %q", got, want)
	}
}

func TestParseServiceEnvFileKeepsQuotedLiteralValue(t *testing.T) {
	values := parseServiceEnvFile("XUI_DB_DSN='postgres://user:pa$WORD@db/xui?application_name=panel'\n")
	if got, want := values["XUI_DB_DSN"], "postgres://user:pa$WORD@db/xui?application_name=panel"; got != want {
		t.Fatalf("XUI_DB_DSN = %q, want %q", got, want)
	}
}

func TestParseServiceEnvFileUnescapesSystemdValue(t *testing.T) {
	values := parseServiceEnvFile(`XUI_DB_DSN=host=db password=pa\\WORD\$TOKEN`)
	if got, want := values["XUI_DB_DSN"], `host=db password=pa\WORD$TOKEN`; got != want {
		t.Fatalf("XUI_DB_DSN = %q, want %q", got, want)
	}
}

func TestParseServiceEnvFilePreservesSingleQuotedBackslash(t *testing.T) {
	values := parseServiceEnvFile(`XUI_DB_DSN='host=db password=pa\\WORD'`)
	if got, want := values["XUI_DB_DSN"], `host=db password=pa\\WORD`; got != want {
		t.Fatalf("XUI_DB_DSN = %q, want %q", got, want)
	}
}
