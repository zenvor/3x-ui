package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDockerInitPlatformArtifacts(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh is needed for Docker packaging tests")
	}
	script, err := filepath.Abs("DockerInit.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ arch, variant, binary, mtg string }{
		{"amd64", "", "amd64", "amd64"},
		{"arm64", "v8", "arm64", "arm64"},
		{"386", "", "386", "386"},
		{"arm", "v7", "arm32", "armv7"},
		{"arm", "v6", "arm32", "armv6"},
		{"armv6", "", "arm32", "armv6"},
	} {
		t.Run(tc.arch+tc.variant, func(t *testing.T) {
			dir := t.TempDir()
			write := func(name, body string) {
				t.Helper()
				path := filepath.Join(dir, name)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			write(".github/scripts/build-xray.sh", "#!/bin/sh\nprintf xray > \"$1\"\n")
			write("xray-module/LICENSE", "Xray license")
			write("stubs/go", "#!/bin/sh\nprintf '%s\\n' \"$TEST_XRAY_MODULE\"\n")
			write("stubs/curl", `#!/bin/sh
printf '%s\n' "$*" >> "$TEST_DOWNLOADS"
case "$*" in
  *api.github.com*) printf '{"tag_name":"v1.2.3"}\n' ;;
esac
`)
			write("stubs/tar", `#!/bin/sh
dir=${2%.tar.gz}
mkdir -p "$dir"
printf mtg > "$dir/mtg-multi"
`)
			cmd := exec.Command("sh", script, tc.arch, tc.variant)
			cmd.Dir = dir
			cmd.Env = []string{
				"PATH=" + filepath.Join(dir, "stubs") + string(os.PathListSeparator) + os.Getenv("PATH"),
				"TEST_XRAY_MODULE=" + filepath.Join(dir, "xray-module"),
				"TEST_DOWNLOADS=" + filepath.Join(dir, "downloads.log"),
			}
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("DockerInit: %v\n%s", err, out)
			}
			for _, name := range []string{"xray-linux-" + tc.binary, "mtg-linux-" + tc.binary, "LICENSE"} {
				if _, err := os.Stat(filepath.Join(dir, "build/bin", name)); err != nil {
					t.Errorf("missing runtime artifact %s: %v", name, err)
				}
			}
			downloads, err := os.ReadFile(filepath.Join(dir, "downloads.log"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(downloads), "mtg-multi-1.2.3-linux-"+tc.mtg+".tar.gz") {
				t.Errorf("wrong MTProto platform:\n%s", downloads)
			}
			if strings.Contains(string(downloads), "XTLS/Xray-core/releases/download/") {
				t.Fatal("Xray must come from the pinned source build")
			}
		})
	}
}
