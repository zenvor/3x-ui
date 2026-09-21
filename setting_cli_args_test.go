package main

import (
	"flag"
	"reflect"
	"testing"
)

func TestNormalizeLegacySettingBoolArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "preserves bare flags",
			args: []string{"-show", "-getCert"},
			want: []string{"-show", "-getCert"},
		},
		{
			name: "normalizes legacy values and preserves following flags",
			args: []string{"-getApiToken", "true", "-tokenName", "ci-bot"},
			want: []string{"-getApiToken=true", "-tokenName", "ci-bot"},
		},
		{
			name: "normalizes false values",
			args: []string{"-show", "false", "-port", "9261"},
			want: []string{"-show=false", "-port", "9261"},
		},
		{
			name: "does not consume unrelated positionals",
			args: []string{"-show", "true", "unexpected", "-port", "9261"},
			want: []string{"-show=true", "unexpected", "-port", "9261"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeLegacySettingBoolArgs(tt.args); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("normalizeLegacySettingBoolArgs(%q) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestLegacyBoolSyntaxKeepsFollowingSettingFlags(t *testing.T) {
	set := flag.NewFlagSet("setting", flag.ContinueOnError)
	var getApiToken bool
	var tokenName string
	set.BoolVar(&getApiToken, "getApiToken", false, "")
	set.StringVar(&tokenName, "tokenName", "", "")

	if err := set.Parse(normalizeLegacySettingBoolArgs([]string{"-getApiToken", "true", "-tokenName", "ci-bot"})); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !getApiToken || tokenName != "ci-bot" || len(set.Args()) != 0 {
		t.Fatalf("getApiToken=%t tokenName=%q leftovers=%q", getApiToken, tokenName, set.Args())
	}
}

func TestUnexpectedSettingPositionalsRemainRejected(t *testing.T) {
	set := flag.NewFlagSet("setting", flag.ContinueOnError)
	var show bool
	set.BoolVar(&show, "show", false, "")

	if err := set.Parse(normalizeLegacySettingBoolArgs([]string{"-show", "true", "unexpected", "-show"})); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !show || !hasIgnoredSettingArgs(set.Args()) {
		t.Fatalf("show=%t leftovers=%q, want enabled show and rejected positionals", show, set.Args())
	}
}
