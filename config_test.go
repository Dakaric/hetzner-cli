package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writeEnv(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "env")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}
	return path
}

// pinEnvFile points the config loader at exactly one dotenv file and clears the
// token env vars, so a test never picks up the developer's real ~/.claude/.env.
func pinEnvFile(t *testing.T, path string) {
	t.Helper()
	t.Setenv("HETZNER_ENV_FILE", path)
	t.Setenv("HETZNER_API_KEY", "")
	t.Setenv("HETZNER_TOKEN", "")
	t.Setenv("HCLOUD_TOKEN", "")
	t.Setenv("HETZNER_BASE_URL", "")
}

func TestParseEnvFile(t *testing.T) {
	path := writeEnv(t, `
# a comment
export HETZNER_API_KEY="secret-token"
HETZNER_BASE_URL = 'https://example.test/v1'
EMPTY=
`)
	vals := parseEnvFile(path)

	if vals["HETZNER_API_KEY"] != "secret-token" {
		t.Errorf("token = %q, want secret-token (export prefix + quotes not stripped)", vals["HETZNER_API_KEY"])
	}
	if vals["HETZNER_BASE_URL"] != "https://example.test/v1" {
		t.Errorf("base url = %q, want trimmed single-quoted value", vals["HETZNER_BASE_URL"])
	}
	if _, ok := vals["EMPTY"]; !ok {
		t.Error("EMPTY key should be present even with empty value")
	}
}

func TestParseEnvFileMissing(t *testing.T) {
	if got := parseEnvFile(filepath.Join(t.TempDir(), "nope.env")); len(got) != 0 {
		t.Errorf("missing file should yield empty map, got %v", got)
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	pinEnvFile(t, writeEnv(t, "HETZNER_API_KEY=file-token\n"))

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.Token != "file-token" {
		t.Errorf("token = %q, want file-token", cfg.Token)
	}
	if cfg.BaseURL != defaultBaseURL {
		t.Errorf("base url = %q, want default %q", cfg.BaseURL, defaultBaseURL)
	}
}

func TestLoadConfigEnvOverridesFile(t *testing.T) {
	pinEnvFile(t, writeEnv(t, "HETZNER_API_KEY=file-token\n"))
	t.Setenv("HETZNER_API_KEY", "env-token")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.Token != "env-token" {
		t.Errorf("token = %q, want env-token (env must win over file)", cfg.Token)
	}
}

func TestLoadConfigHCloudAlias(t *testing.T) {
	pinEnvFile(t, writeEnv(t, "# no token here\n"))
	t.Setenv("HCLOUD_TOKEN", "hcloud-token")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.Token != "hcloud-token" {
		t.Errorf("token = %q, want hcloud-token (HCLOUD_TOKEN alias)", cfg.Token)
	}
}

func TestLoadConfigMissingToken(t *testing.T) {
	pinEnvFile(t, writeEnv(t, "HETZNER_BASE_URL=https://example.test\n"))

	if _, err := loadConfig(); err == nil {
		t.Error("expected an error when no token is configured")
	}
}

func TestLoadConfigTrimsTrailingSlash(t *testing.T) {
	pinEnvFile(t, writeEnv(t, ""))
	t.Setenv("HETZNER_API_KEY", "t")
	t.Setenv("HETZNER_BASE_URL", "https://example.test/v1/")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.BaseURL != "https://example.test/v1" {
		t.Errorf("base url = %q, want trailing slash trimmed", cfg.BaseURL)
	}
}

func TestEnvFileCandidatesPinned(t *testing.T) {
	t.Setenv("HETZNER_ENV_FILE", "/tmp/pinned/env")
	got := envFileCandidates()
	if len(got) != 1 || got[0] != "/tmp/pinned/env" {
		t.Errorf("pinned candidates = %v, want exactly [/tmp/pinned/env]", got)
	}
}

func TestLoadEnvFilesEarlierWins(t *testing.T) {
	first := writeEnv(t, "HETZNER_API_KEY=first\nA=1\n")
	second := writeEnv(t, "HETZNER_API_KEY=second\nB=2\n")

	merged, used := loadEnvFiles([]string{first, second})
	if merged["HETZNER_API_KEY"] != "first" {
		t.Errorf("token = %q, want first (earlier file wins)", merged["HETZNER_API_KEY"])
	}
	if merged["B"] != "2" {
		t.Errorf("B = %q, want 2 (later file still contributes new keys)", merged["B"])
	}
	if used != first {
		t.Errorf("used = %q, want the first contributing file %q", used, first)
	}
}

func TestWriteTokenRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "env")
	if err := writeToken(path, "abc123"); err != nil {
		t.Fatalf("writeToken: %v", err)
	}
	if got := parseEnvFile(path)["HETZNER_API_KEY"]; got != "abc123" {
		t.Errorf("written token = %q, want abc123", got)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	// Unix file permissions don't carry meaning on Windows, where Mode().Perm()
	// reflects the read-only attribute rather than a 0600 bitmask.
	if runtime.GOOS != "windows" {
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("file perm = %o, want 600 (token file must be private)", perm)
		}
	}
}

func TestWriteTokenPreservesOtherKeys(t *testing.T) {
	path := writeEnv(t, "OTHER=keep\nHETZNER_API_KEY=old\n")
	if err := writeToken(path, "new"); err != nil {
		t.Fatalf("writeToken: %v", err)
	}
	vals := parseEnvFile(path)
	if vals["HETZNER_API_KEY"] != "new" {
		t.Errorf("token = %q, want new", vals["HETZNER_API_KEY"])
	}
	if vals["OTHER"] != "keep" {
		t.Errorf("OTHER = %q, want keep (existing keys preserved)", vals["OTHER"])
	}
}
