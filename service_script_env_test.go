package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func serviceScriptFunction(t *testing.T, source, name string) string {
	t.Helper()
	start := strings.Index(source, "\n"+name+"() {")
	if start < 0 {
		t.Fatalf("missing %s", name)
	}
	start++
	end := strings.Index(source[start:], "\n}\n")
	if end < 0 {
		t.Fatalf("unterminated %s", name)
	}
	return source[start : start+end+3]
}

func runServiceScript(t *testing.T, script, input, action string, args ...string) []byte {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is needed for installer tests")
	}
	body, err := os.ReadFile(script)
	if err != nil {
		t.Fatal(err)
	}
	envPath := filepath.Join(t.TempDir(), "service.env")
	if err := os.WriteFile(envPath, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	program := serviceScriptFunction(t, string(body), "_xui_load_service_env") + action
	cmd := exec.Command("bash", append([]string{"-eu", "-c", program, "test", envPath}, args...)...)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "WORD=expanded", "TOKEN=expanded"}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s: %v\n%s", script, err, stderr.String())
	}
	return out
}

func TestServiceScriptEnvironmentValues(t *testing.T) {
	cases := []struct {
		name, input, want string
	}{
		{"unquoted", "XUI_DB_DSN=host=db password=pa$WORD application_name=x-ui&sslmode=disable\n", "host=db password=pa$WORD application_name=x-ui&sslmode=disable"},
		{"single quoted", "XUI_DB_DSN='host=db password=pa\\\\WORD$TOKEN'\n", "host=db password=pa\\\\WORD$TOKEN"},
		{"double quoted", " XUI_DB_DSN = \"host=db password=pa\\\\WORD\\$TOKEN\\q\" \r\n", "host=db password=pa\\WORD$TOKEN\\q"},
		{"unquoted escapes", "XUI_DB_DSN=host=db password=pa\\\\WORD\\$TOKEN", "host=db password=pa\\WORD$TOKEN"},
		{"empty", "XUI_DB_DSN=old\n# comment\n; comment\nnot-an-assignment\nXUI_DB_DSN=\"\"\n", ""},
	}
	for _, script := range []string{"install.sh", "update.sh"} {
		for _, tc := range cases {
			t.Run(script+"/"+tc.name, func(t *testing.T) {
				out := runServiceScript(t, script, tc.input,
					"\n_xui_load_service_env \"$1\"\nprintf '%s' \"$XUI_DB_DSN\"\n")
				if string(out) != tc.want {
					t.Fatalf("DSN = %q, want %q", out, tc.want)
				}
			})
		}
	}
}

func TestServiceScriptDropinDoesNotExecuteEnvironment(t *testing.T) {
	for _, script := range []string{"install.sh", "update.sh"} {
		t.Run(script, func(t *testing.T) {
			dir := t.TempDir()
			marker := filepath.Join(dir, "executed")
			input := "XUI_DB_DSN=host=db password=pa$WORD application_name=x-ui&$(touch '" + marker + "')\n" +
				"XUI_DB_FOLDER='/etc/xui-$TOKEN&literal'\nXUI_LOG_FOLDER=/var/log/xui-%i\n" +
				"XUI_BIN_FOLDER=\"custom-bin\"\nXUI_ENABLE_FAIL2BAN=false\n"
			body, err := os.ReadFile(script)
			if err != nil {
				t.Fatal(err)
			}
			action := "\n" + serviceScriptFunction(t, string(body), "_xui_service_write_paths_dropin") + `
_xui_systemd_major_version() { printf '255\n'; }
systemctl() { return 0; }
xui_folder=/usr/local/x-ui
xui_service="$2"
_xui_service_write_paths_dropin "$1"
printf '%s\0' "$XUI_DB_DSN" "$XUI_ENABLE_FAIL2BAN"
`
			out := runServiceScript(t, script, input, action, dir)
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("EnvironmentFile was executed: marker stat error = %v", err)
			}
			parts := bytes.Split(out, []byte{0})
			wantDSN := strings.TrimPrefix(strings.SplitN(input, "\n", 2)[0], "XUI_DB_DSN=")
			if len(parts) != 3 || string(parts[0]) != wantDSN || string(parts[1]) != "false" {
				t.Fatalf("loaded values = %q", out)
			}
			dropin, err := os.ReadFile(filepath.Join(dir, "x-ui.service.d", "10-xui-sandbox.conf"))
			if err != nil {
				t.Fatal(err)
			}
			want := "ReadWritePaths=-/etc/xui-$TOKEN&literal -/var/log/xui-%%i -/usr/local/x-ui/custom-bin -/usr/local/x-ui\n"
			if !strings.Contains(string(dropin), want) {
				t.Fatalf("drop-in paths differ:\n%s", dropin)
			}
			if strings.Contains(string(dropin), "password=") {
				t.Fatal("drop-in leaked the DSN")
			}
		})
	}
}

func TestUpdaterLoadsEnvironmentWithoutExecutingIt(t *testing.T) {
	body, err := os.ReadFile("update.sh")
	if err != nil {
		t.Fatal(err)
	}
	action := "\n" + serviceScriptFunction(t, string(body), "load_xui_env") + `
service_env="$1"
xui_env_file_path() { printf '%s\n' "$service_env"; }
load_xui_env
printf '%s\0' "$XUI_DB_DSN" "$XUI_ENABLE_FAIL2BAN"
`
	input := "XUI_DB_DSN='host=db password=pa$WORD&literal'\nXUI_ENABLE_FAIL2BAN=false\n"
	out := runServiceScript(t, "update.sh", input, action)
	if string(out) != "host=db password=pa$WORD&literal\x00false\x00" {
		t.Fatalf("loaded values = %q", out)
	}
}
