package main

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// defaultBaseURL is the Hetzner Cloud REST API v1 root. It rarely changes, so
// HETZNER_BASE_URL is optional and only needed to point the CLI elsewhere.
const defaultBaseURL = "https://api.hetzner.cloud/v1"

// Config holds the resolved connection settings for the Hetzner Cloud API.
type Config struct {
	BaseURL string
	Token   string
}

// loadConfig resolves the base URL and the project API token. A real
// HETZNER_API_KEY environment variable always wins; otherwise the token is read
// from the first dotenv candidate that has it (see envFileCandidates). The token
// lookup accepts a few names so HCLOUD_TOKEN (the official hcloud CLI's var)
// works too.
func loadConfig() (Config, error) {
	fileVals, _ := loadEnvFiles(envFileCandidates())

	base := firstNonEmpty(
		os.Getenv("HETZNER_BASE_URL"), fileVals["HETZNER_BASE_URL"], defaultBaseURL)
	token := resolveToken(fileVals)

	if token == "" {
		return Config{}, fmt.Errorf("no Hetzner token found — run `hetzner login`, or set HETZNER_API_KEY (checked: %s)",
			strings.Join(envFileCandidates(), ", "))
	}
	return Config{BaseURL: strings.TrimRight(base, "/"), Token: token}, nil
}

// resolveToken finds the Cloud API token across the supported names, preferring
// real environment variables over the dotenv values for each candidate.
func resolveToken(fileVals map[string]string) string {
	return firstNonEmpty(
		os.Getenv("HETZNER_API_KEY"), fileVals["HETZNER_API_KEY"],
		os.Getenv("HETZNER_TOKEN"), fileVals["HETZNER_TOKEN"],
		os.Getenv("HCLOUD_TOKEN"), fileVals["HCLOUD_TOKEN"],
	)
}

// envFileCandidates lists the dotenv files consulted, most specific first. A
// real HETZNER_API_KEY env var still overrides all of them. When HETZNER_ENV_FILE
// is set it is used exclusively, so a caller can pin one file.
func envFileCandidates() []string {
	if p := os.Getenv("HETZNER_ENV_FILE"); p != "" {
		return []string{p}
	}
	var paths []string
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		paths = append(paths, filepath.Join(xdg, "hetzner", "env"))
	}
	if appdata := os.Getenv("APPDATA"); appdata != "" { // Windows
		paths = append(paths, filepath.Join(appdata, "hetzner", "env"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths,
			filepath.Join(home, ".config", "hetzner", "env"),
			filepath.Join(home, ".claude", ".env"),
		)
	}
	return dedupe(paths)
}

// dedupe removes duplicate paths while preserving order, so a doctor listing or
// lookup does not show the same file twice (e.g. when XDG_CONFIG_HOME already
// points at ~/.config).
func dedupe(paths []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

// configWritePath is where `hetzner login` stores the token: this tool's
// per-user config file, OS-appropriate (APPDATA on Windows, XDG/.config on
// Unix). HETZNER_ENV_FILE pins it explicitly.
func configWritePath() string {
	if p := os.Getenv("HETZNER_ENV_FILE"); p != "" {
		return p
	}
	if runtime.GOOS == "windows" {
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "hetzner", "env")
		}
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "hetzner", "env")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "hetzner", "env")
	}
	return "hetzner.env"
}

// loadEnvFiles parses each candidate, merging values with earlier files winning,
// and reports the first file that contributed any value (for the doctor output).
func loadEnvFiles(paths []string) (map[string]string, string) {
	merged := map[string]string{}
	used := ""
	for _, p := range paths {
		vals := parseEnvFile(p)
		if len(vals) > 0 && used == "" {
			used = p
		}
		for k, v := range vals {
			if _, ok := merged[k]; !ok {
				merged[k] = v
			}
		}
	}
	return merged, used
}

// parseEnvFile reads a dotenv-style file into a map. A missing or unreadable
// file is not an error: it yields an empty map so env vars can still satisfy
// the config. Lines may use an optional "export " prefix and quoted values.
func parseEnvFile(path string) map[string]string {
	out := map[string]string{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		out[key] = val
	}
	return out
}

// warnInsecureBaseURL prints a one-line stderr warning when the resolved base
// URL would send the bearer token over plaintext http to a non-loopback host.
// It never blocks — a local proxy or a test server on http is occasionally
// legitimate — but a token leaving over the wire in the clear must not be
// silent. Both doJSON and raw inherit the base URL, so warning once here covers
// the typed commands and the `api` passthrough alike.
func warnInsecureBaseURL(base string) {
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "http" {
		return
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1", "":
		return
	}
	fmt.Fprintf(os.Stderr, "warning: HETZNER_BASE_URL is http:// (%s) — your API token will be sent in plaintext\n", base)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
