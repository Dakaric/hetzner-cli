package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// main parses the command, loads config, and dispatches. It orchestrates only:
// the per-resource work lives in commands.go, the shell bridge in ssh.go, and
// every HTTP call in client.go.
func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	cmd, args := os.Args[1], os.Args[2:]
	switch cmd {
	case "help", "-h", "--help":
		usage()
		return
	case "config":
		cmdConfig()
		return
	case "login":
		cmdLogin(args)
		return
	}

	cfg, err := loadConfig()
	if err != nil {
		fail(err)
	}
	client := newClient(cfg)

	switch cmd {
	case "servers":
		cmdServers(client, args)
	case "server":
		cmdServer(client, args)
	case "ssh":
		cmdSSH(client, args)
	case "exec", "run":
		cmdExec(client, args)
	case "volumes":
		cmdVolumes(client, args)
	case "volume":
		cmdVolume(client, args)
	case "networks":
		cmdNetworks(client, args)
	case "network":
		cmdNetwork(client, args)
	case "firewalls":
		cmdFirewalls(client, args)
	case "firewall":
		cmdFirewall(client, args)
	case "ssh-keys", "sshkeys", "keys":
		cmdSSHKeys(client, args)
	case "ssh-key", "sshkey", "key":
		cmdSSHKey(client, args)
	case "images", "image":
		cmdImages(client, args)
	case "server-types", "types":
		cmdServerTypes(client, args)
	case "locations":
		cmdLocations(client, args)
	case "datacenters":
		cmdDatacenters(client, args)
	case "floating-ips", "floating-ip", "fips":
		cmdFloatingIPs(client, args)
	case "primary-ips", "primary-ip":
		cmdPrimaryIPs(client, args)
	case "load-balancers", "load-balancer", "lbs", "lb":
		cmdLoadBalancers(client, args)
	case "pricing":
		cmdPricing(client, args)
	case "status":
		cmdStatus(client, args)
	case "api":
		cmdAPI(client, args)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		usage()
		os.Exit(2)
	}
}

// cmdConfig is a non-secret doctor: it reports the resolved env file, the base
// URL, and whether the token is present plus its length. It never prints the
// token value itself.
func cmdConfig() {
	candidates := envFileCandidates()
	fileVals, used := loadEnvFiles(candidates)

	base := firstNonEmpty(os.Getenv("HETZNER_BASE_URL"), fileVals["HETZNER_BASE_URL"], defaultBaseURL)
	token := resolveToken(fileVals)

	fmt.Printf("base url:     %s\n", base)
	fmt.Printf("env files:    %s\n", strings.Join(candidates, ", "))
	fmt.Printf("loaded from:  %s\n", orNone(used))
	if token == "" {
		fmt.Println("api key:      MISSING — run `hetzner login` or set HETZNER_API_KEY")
	} else {
		fmt.Printf("api key:      set (%d chars)\n", len(token))
	}
}

// cmdLogin is the onboarding step: it takes a token (argument or prompt),
// validates it against the live API, and writes it to the per-user config file.
func cmdLogin(args []string) {
	o := parseOpts(args)
	token := strings.TrimSpace(firstNonEmpty(posAt(o.pos, 0), promptToken()))
	if token == "" {
		fail(fmt.Errorf("no token provided"))
	}

	client := newClient(Config{BaseURL: defaultBaseURL, Token: token})
	status, _, err := client.raw(http.MethodGet, "/servers?per_page=1", nil)
	switch {
	case err != nil:
		fail(fmt.Errorf("could not reach Hetzner: %w", err))
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		fail(fmt.Errorf("token rejected by Hetzner (HTTP %d) — make sure it is a Cloud API token with read & write", status))
	case status < 200 || status >= 300:
		fail(fmt.Errorf("unexpected response validating token: HTTP %d", status))
	}

	path := configWritePath()
	if err := writeToken(path, token); err != nil {
		fail(err)
	}
	fmt.Printf("token validated and saved to %s\n", path)
	fmt.Println("run `hetzner status` to see your project.")
}

// promptToken reads a token from stdin. The prompt goes to stderr so stdout
// stays clean if the command is ever piped.
func promptToken() string {
	fmt.Fprint(os.Stderr, "Paste your Hetzner Cloud API token: ")
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return line
}

// writeToken stores the token as HETZNER_API_KEY in a dotenv file, creating the
// directory with private permissions. Existing keys in the file are preserved;
// only HETZNER_API_KEY is set or replaced.
func writeToken(path, token string) error {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create config dir: %w", err)
		}
	}
	vals := parseEnvFile(path)
	vals["HETZNER_API_KEY"] = token

	var b strings.Builder
	for _, k := range sortedKeys(vals) {
		fmt.Fprintf(&b, "%s=%s\n", k, vals[k])
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func orNone(s string) string {
	if s == "" {
		return "(none found — token must come from the environment)"
	}
	return s
}

// cmdAPI is a low-level passthrough to any Cloud API endpoint, signing the
// request with the configured token. It prints the HTTP status to stderr and
// the pretty-printed body to stdout, exiting non-zero on a non-2xx status while
// still showing the body — handy for endpoints the typed commands do not cover.
func cmdAPI(c *Client, args []string) {
	if len(args) < 2 {
		fail(fmt.Errorf("usage: hetzner api <METHOD> <path> [json-body]   (e.g. api GET /servers, api POST /ssh_keys '{...}')"))
	}
	method, path := strings.ToUpper(args[0]), args[1]
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	var body []byte
	if len(args) >= 3 {
		body = []byte(args[2])
	}

	status, respBody, err := c.raw(method, path, body)
	if err != nil {
		fail(err)
	}
	fmt.Fprintf(os.Stderr, "HTTP %d\n", status)

	var pretty any
	if len(respBody) > 0 && json.Unmarshal(respBody, &pretty) == nil {
		printJSON(pretty)
	} else if len(respBody) > 0 {
		fmt.Println(string(respBody))
	}
	if status < 200 || status >= 300 {
		os.Exit(1)
	}
}

// --- Flag parsing ----------------------------------------------------------

// opts is the forgiving, order-independent flag set shared by all commands.
// Boolean flags land in bools, valued flags in flags, repeatable --ssh-key in
// sshKeys, and everything else is a positional argument.
type opts struct {
	flags   map[string]string
	bools   map[string]bool
	sshKeys []string
	pos     []string
}

// parseOpts splits args into flags, bools, repeatable ssh keys, and
// positionals. Both `--flag value` and `--flag=value` are accepted; a `--flag`
// with no following value (or followed by another flag) is treated as a bool.
func parseOpts(args []string) opts {
	o := opts{flags: map[string]string{}, bools: map[string]bool{}}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json" || arg == "-j":
			o.bools["json"] = true
		case arg == "--yes" || arg == "-y" || arg == "--force":
			o.bools["yes"] = true
		case arg == "--ssh-key":
			if i+1 < len(args) {
				o.sshKeys = append(o.sshKeys, args[i+1])
				i++
			}
		case strings.HasPrefix(arg, "--ssh-key="):
			o.sshKeys = append(o.sshKeys, strings.TrimPrefix(arg, "--ssh-key="))
		case strings.HasPrefix(arg, "--") && strings.Contains(arg, "="):
			key, val, _ := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
			o.flags[key] = val
		case strings.HasPrefix(arg, "--"):
			key := strings.TrimPrefix(arg, "--")
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				o.flags[key] = args[i+1]
				i++
			} else {
				o.bools[key] = true
			}
		default:
			o.pos = append(o.pos, arg)
		}
	}
	return o
}

func (o opts) json() bool          { return o.bools["json"] }
func (o opts) yes() bool           { return o.bools["yes"] }
func (o opts) bool(k string) bool  { return o.bools[k] }
func (o opts) get(k string) string { return o.flags[k] }

// firstPos returns the first positional argument or aborts with a usage hint —
// the addressing commands all need exactly one id-or-name reference.
func firstPos(o opts) string {
	if len(o.pos) == 0 {
		fail(fmt.Errorf("missing <id|name> argument"))
	}
	return o.pos[0]
}

func posAt(pos []string, i int) string {
	if i < len(pos) {
		return pos[i]
	}
	return ""
}

// requireYes refuses a destructive operation unless --yes was passed. The CLI
// is driven by an agent without a TTY, so the guard is an explicit flag rather
// than an interactive prompt.
func requireYes(o opts, what string) {
	if !o.yes() {
		fail(fmt.Errorf("refusing to %s without --yes (re-run with --yes to confirm)", what))
	}
}

// parseJSONArray validates that s is a JSON array and returns it decoded, so the
// firewall rules a caller supplies are well-formed before they reach the API.
func parseJSONArray(s string) ([]any, error) {
	var arr []any
	if err := json.Unmarshal([]byte(s), &arr); err != nil {
		return nil, err
	}
	return arr, nil
}

// expandHome resolves a leading ~ to the user's home directory.
func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	return path
}

func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

func usage() {
	fmt.Fprint(os.Stderr, `hetzner - Hetzner Cloud CLI

Setup & status:
  hetzner login [token]                Validate a Cloud API token and save it (onboarding)
  hetzner config                       Doctor: base URL, env files, key presence (never the token)
  hetzner status                       Resource counts + connection test

Servers:
  hetzner servers [--json]             List all servers
  hetzner server <id|name> [--json]    Show one server
  hetzner server create --name <n> --type <cx22> --image <ubuntu-24.04>
                 [--location fsn1 | --datacenter fsn1-dc14] [--ssh-key <name|id> …] [--no-start]
  hetzner server delete <id|name> --yes
  hetzner server reboot|poweron|shutdown <id|name>
  hetzner server poweroff|reset <id|name> --yes      (hard, needs --yes)

Shell into a server (control plane → data plane, via the system ssh client):
  hetzner ssh  <id|name> [--user root] [--key <path>] [--port 22]   Interactive shell
  hetzner exec <id|name> '<command>' [--user root] [--key <path>]   Run one command, stream output

Volumes:
  hetzner volumes [--json]
  hetzner volume <id|name> [--json]
  hetzner volume create --name <n> --size <GB> (--location fsn1 | --server <id|name>) [--format ext4] [--automount]
  hetzner volume attach <id|name> --server <id|name> [--automount]
  hetzner volume detach <id|name>
  hetzner volume delete <id|name> --yes

Networks:
  hetzner networks [--json]
  hetzner network <id|name> [--json]
  hetzner network create --name <n> --ip-range <10.0.0.0/16>
  hetzner network delete <id|name> --yes

Firewalls:
  hetzner firewalls [--json]
  hetzner firewall <id|name> [--json]
  hetzner firewall create --name <n> [--rules '<json-array>']
  hetzner firewall delete <id|name> --yes

SSH keys:
  hetzner ssh-keys [--json]
  hetzner ssh-key <id|name> [--json]
  hetzner ssh-key create --name <n> (--public-key '<key>' | --public-key-file <path>)
  hetzner ssh-key delete <id|name> --yes

Catalogs (read-only):
  hetzner images [--type system|snapshot|backup|app] [--json]
  hetzner server-types [--json]        Machine sizes incl. monthly price
  hetzner locations [--json]
  hetzner datacenters [--json]
  hetzner floating-ips [--json]
  hetzner primary-ips [--json]
  hetzner load-balancers [<id|name>] [--json]
  hetzner pricing [--json]

Escape hatch:
  hetzner api <METHOD> <path> [json-body]   Signed request to any endpoint (e.g. api GET /servers)

Config (env vars override the dotenv files):
  HETZNER_API_KEY     Hetzner Cloud project API token (also accepts HETZNER_TOKEN / HCLOUD_TOKEN)
  HETZNER_BASE_URL    optional, defaults to https://api.hetzner.cloud/v1
  HETZNER_ENV_FILE    optional, pin a single dotenv file

Destructive commands (delete, hard poweroff/reset) require --yes.
`)
}
