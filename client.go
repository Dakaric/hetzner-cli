package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client is the sole owner of the Hetzner Cloud API: it knows the base URL, the
// bearer auth scheme, the resource shapes, and the {resource}/{collection}
// envelopes. No other part of the program constructs requests or talks HTTP to
// Hetzner — commands call typed methods, never raw paths (except `api`).
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func newClient(cfg Config) *Client {
	return &Client{
		baseURL: cfg.BaseURL,
		token:   cfg.Token,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// --- Resource shapes -------------------------------------------------------

// Server is one Cloud server. The nested PublicNet/ServerType/Datacenter carry
// the fields the CLI renders; many more exist and are reachable via `api`.
type Server struct {
	ID         int               `json:"id"`
	Name       string            `json:"name"`
	Status     string            `json:"status"`
	PublicNet  PublicNet         `json:"public_net"`
	PrivateNet []PrivateNet      `json:"private_net"`
	ServerType ServerType        `json:"server_type"`
	Datacenter Datacenter        `json:"datacenter"`
	Image      *Image            `json:"image"`
	Volumes    []int             `json:"volumes"`
	Protection Protection        `json:"protection"`
	Locked     bool              `json:"locked"`
	Created    string            `json:"created"`
	Labels     map[string]string `json:"labels"`
}

type PublicNet struct {
	IPv4 IPv4 `json:"ipv4"`
	IPv6 IPv6 `json:"ipv6"`
}

type IPv4 struct {
	IP      string `json:"ip"`
	DNSPtr  string `json:"dns_ptr"`
	Blocked bool   `json:"blocked"`
}

type IPv6 struct {
	IP      string `json:"ip"`
	Blocked bool   `json:"blocked"`
}

type PrivateNet struct {
	Network  int      `json:"network"`
	IP       string   `json:"ip"`
	AliasIPs []string `json:"alias_ips"`
}

type Protection struct {
	Delete  bool `json:"delete"`
	Rebuild bool `json:"rebuild"`
}

// ServerType is a machine size (cx22, ...). Memory is GB; Disk is GB.
type ServerType struct {
	ID           int         `json:"id"`
	Name         string      `json:"name"`
	Description  string      `json:"description"`
	Cores        int         `json:"cores"`
	Memory       float64     `json:"memory"`
	Disk         int         `json:"disk"`
	CPUType      string      `json:"cpu_type"`
	Architecture string      `json:"architecture"`
	Deprecated   bool        `json:"deprecated"`
	Prices       []TypePrice `json:"prices"`
}

type TypePrice struct {
	Location     string      `json:"location"`
	PriceHourly  PriceAmount `json:"price_hourly"`
	PriceMonthly PriceAmount `json:"price_monthly"`
}

type PriceAmount struct {
	Net   string `json:"net"`
	Gross string `json:"gross"`
}

type Datacenter struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Location    Location `json:"location"`
}

type Location struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	City        string `json:"city"`
	Country     string `json:"country"`
	NetworkZone string `json:"network_zone"`
}

type Image struct {
	ID           int     `json:"id"`
	Type         string  `json:"type"`
	Status       string  `json:"status"`
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	OSFlavor     string  `json:"os_flavor"`
	OSVersion    string  `json:"os_version"`
	Architecture string  `json:"architecture"`
	ImageSize    float64 `json:"image_size"`
	DiskSize     float64 `json:"disk_size"`
	Created      string  `json:"created"`
}

type Volume struct {
	ID          int               `json:"id"`
	Name        string            `json:"name"`
	Status      string            `json:"status"`
	Server      *int              `json:"server"`
	Location    Location          `json:"location"`
	Size        int               `json:"size"`
	LinuxDevice string            `json:"linux_device"`
	Format      *string           `json:"format"`
	Protection  Protection        `json:"protection"`
	Created     string            `json:"created"`
	Labels      map[string]string `json:"labels"`
}

type Network struct {
	ID         int               `json:"id"`
	Name       string            `json:"name"`
	IPRange    string            `json:"ip_range"`
	Subnets    []Subnet          `json:"subnets"`
	Routes     []Route           `json:"routes"`
	Servers    []int             `json:"servers"`
	Protection Protection        `json:"protection"`
	Created    string            `json:"created"`
	Labels     map[string]string `json:"labels"`
}

type Subnet struct {
	Type        string `json:"type"`
	IPRange     string `json:"ip_range"`
	NetworkZone string `json:"network_zone"`
	Gateway     string `json:"gateway"`
}

type Route struct {
	Destination string `json:"destination"`
	Gateway     string `json:"gateway"`
}

type Firewall struct {
	ID        int               `json:"id"`
	Name      string            `json:"name"`
	Rules     []FirewallRule    `json:"rules"`
	AppliedTo []FirewallApplied `json:"applied_to"`
	Created   string            `json:"created"`
	Labels    map[string]string `json:"labels"`
}

type FirewallRule struct {
	Direction      string   `json:"direction"`
	Protocol       string   `json:"protocol"`
	Port           string   `json:"port"`
	SourceIPs      []string `json:"source_ips"`
	DestinationIPs []string `json:"destination_ips"`
	Description    string   `json:"description"`
}

type FirewallApplied struct {
	Type   string  `json:"type"`
	Server *idOnly `json:"server"`
}

type idOnly struct {
	ID int `json:"id"`
}

type SSHKey struct {
	ID          int               `json:"id"`
	Name        string            `json:"name"`
	Fingerprint string            `json:"fingerprint"`
	PublicKey   string            `json:"public_key"`
	Created     string            `json:"created"`
	Labels      map[string]string `json:"labels"`
}

type FloatingIP struct {
	ID           int      `json:"id"`
	Type         string   `json:"type"`
	IP           string   `json:"ip"`
	Server       *int     `json:"server"`
	Description  string   `json:"description"`
	HomeLocation Location `json:"home_location"`
	Created      string   `json:"created"`
}

type PrimaryIP struct {
	ID           int        `json:"id"`
	Type         string     `json:"type"`
	IP           string     `json:"ip"`
	AssigneeID   *int       `json:"assignee_id"`
	AssigneeType string     `json:"assignee_type"`
	Datacenter   Datacenter `json:"datacenter"`
	Created      string     `json:"created"`
}

type LoadBalancer struct {
	ID        int         `json:"id"`
	Name      string      `json:"name"`
	PublicNet LBPublicNet `json:"public_net"`
	Location  Location    `json:"location"`
	Type      LBType      `json:"load_balancer_type"`
	Services  []LBService `json:"services"`
	Targets   []LBTarget  `json:"targets"`
	Created   string      `json:"created"`
}

type LBPublicNet struct {
	IPv4 IPv4 `json:"ipv4"`
	IPv6 IPv6 `json:"ipv6"`
}

type LBType struct {
	Name             string `json:"name"`
	MaxServices      int    `json:"max_services"`
	MaxConnections   int    `json:"max_connections"`
	MaxTargets       int    `json:"max_targets"`
	MaxAssignedCerts int    `json:"max_assigned_certificates"`
}

type LBService struct {
	Protocol        string `json:"protocol"`
	ListenPort      int    `json:"listen_port"`
	DestinationPort int    `json:"destination_port"`
}

type LBTarget struct {
	Type   string  `json:"type"`
	Server *idOnly `json:"server"`
}

// Action is the asynchronous result Hetzner returns for create/delete and the
// server/volume power actions. Status is running|success|error.
type Action struct {
	ID        int              `json:"id"`
	Command   string           `json:"command"`
	Status    string           `json:"status"`
	Progress  int              `json:"progress"`
	Started   string           `json:"started"`
	Finished  *string          `json:"finished"`
	Error     *ActionError     `json:"error"`
	Resources []ActionResource `json:"resources"`
}

type ActionError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ActionResource struct {
	ID   int    `json:"id"`
	Type string `json:"type"`
}

// Pricing is the subset of /pricing the CLI renders. The full payload (every
// server type in every location) is available via `hetzner pricing --json`.
type Pricing struct {
	Currency    string              `json:"currency"`
	VATRate     string              `json:"vat_rate"`
	Image       perGBMonth          `json:"image"`
	Volume      perGBMonth          `json:"volume"`
	Traffic     trafficPrice        `json:"traffic"`
	ServerTypes []pricingServerType `json:"server_types"`
}

type perGBMonth struct {
	PricePerGBMonth PriceAmount `json:"price_per_gb_month"`
}

type trafficPrice struct {
	PricePerTB PriceAmount `json:"price_per_tb"`
}

type pricingServerType struct {
	Name   string      `json:"name"`
	Prices []TypePrice `json:"prices"`
}

// --- Create request bodies -------------------------------------------------

type CreateServerRequest struct {
	Name             string   `json:"name"`
	ServerType       string   `json:"server_type"`
	Image            string   `json:"image"`
	Location         string   `json:"location,omitempty"`
	Datacenter       string   `json:"datacenter,omitempty"`
	SSHKeys          []string `json:"ssh_keys,omitempty"`
	StartAfterCreate bool     `json:"start_after_create"`
}

type CreateVolumeRequest struct {
	Name      string `json:"name"`
	Size      int    `json:"size"`
	Location  string `json:"location,omitempty"`
	Server    *int   `json:"server,omitempty"`
	Format    string `json:"format,omitempty"`
	Automount bool   `json:"automount"`
}

// --- Pagination ------------------------------------------------------------

// perPage is the page size requested for every list endpoint. Hetzner caps it
// at 50; listAll walks all pages, so this only trades round-trips, never
// completeness.
const perPage = 50

// listAll is the single owner of list pagination: it walks every page of a
// Hetzner collection endpoint and concatenates the named array. Each typed list
// method delegates here, so page-walking and the per-page size live in one
// place instead of being re-decided (or forgotten) per resource. basePath may
// already carry a query (e.g. an image type filter); page/per_page are appended.
func listAll[T any](c *Client, basePath, key string) ([]T, error) {
	var all []T
	for page := 1; page > 0; {
		var env map[string]json.RawMessage
		if err := c.doJSON(http.MethodGet, pagedPath(basePath, page), nil, &env); err != nil {
			return nil, err
		}
		if raw, ok := env[key]; ok {
			var items []T
			if err := json.Unmarshal(raw, &items); err != nil {
				return nil, fmt.Errorf("decode %s list: %w", key, err)
			}
			all = append(all, items...)
		}
		page = nextPage(env["meta"])
	}
	return all, nil
}

// pagedPath appends page and per_page to a list path, preserving any query
// string the caller already set.
func pagedPath(basePath string, page int) string {
	sep := "?"
	if strings.Contains(basePath, "?") {
		sep = "&"
	}
	return fmt.Sprintf("%s%spage=%d&per_page=%d", basePath, sep, page, perPage)
}

// nextPage reads meta.pagination.next_page from a list response. Hetzner sets it
// to null on the final page, which decodes to a nil pointer here and returns 0,
// stopping the walk.
func nextPage(meta json.RawMessage) int {
	if len(meta) == 0 {
		return 0
	}
	var m struct {
		Pagination struct {
			NextPage *int `json:"next_page"`
		} `json:"pagination"`
	}
	if json.Unmarshal(meta, &m) == nil && m.Pagination.NextPage != nil {
		return *m.Pagination.NextPage
	}
	return 0
}

// --- Typed endpoints -------------------------------------------------------

func (c *Client) servers() ([]Server, error) {
	return listAll[Server](c, "/servers", "servers")
}

func (c *Client) server(id int) (Server, error) {
	var env struct {
		Server Server `json:"server"`
	}
	err := c.doJSON(http.MethodGet, fmt.Sprintf("/servers/%d", id), nil, &env)
	return env.Server, err
}

// serverIPv4 resolves a server reference (id or name) to its public IPv4 and
// name. It is the single bridge the ssh/exec commands use to turn a friendly
// reference into a connectable host. An error is returned when the server has
// no public IPv4 (e.g. private-only servers).
func (c *Client) serverIPv4(ref string) (ip, name string, err error) {
	id, err := c.lookupID("servers", ref)
	if err != nil {
		return "", "", err
	}
	s, err := c.server(id)
	if err != nil {
		return "", "", err
	}
	if s.PublicNet.IPv4.IP == "" {
		return "", "", fmt.Errorf("server %s (#%d) has no public IPv4 to connect to", s.Name, s.ID)
	}
	return s.PublicNet.IPv4.IP, s.Name, nil
}

// createServer provisions a server and returns it plus a root password, which
// Hetzner only sets (and only returns once) when no SSH key was supplied.
func (c *Client) createServer(req CreateServerRequest) (Server, string, error) {
	var env struct {
		Server       Server `json:"server"`
		RootPassword string `json:"root_password"`
	}
	err := c.doJSON(http.MethodPost, "/servers", req, &env)
	return env.Server, env.RootPassword, err
}

func (c *Client) deleteServer(id int) error {
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/servers/%d", id), nil, nil)
}

// serverAction runs a power action (reboot|poweron|poweroff|shutdown|reset) and
// returns the asynchronous Action handle.
func (c *Client) serverAction(id int, action string) (Action, error) {
	var env struct {
		Action Action `json:"action"`
	}
	err := c.doJSON(http.MethodPost, fmt.Sprintf("/servers/%d/actions/%s", id, action), nil, &env)
	return env.Action, err
}

func (c *Client) volumes() ([]Volume, error) {
	return listAll[Volume](c, "/volumes", "volumes")
}

func (c *Client) volume(id int) (Volume, error) {
	var env struct {
		Volume Volume `json:"volume"`
	}
	err := c.doJSON(http.MethodGet, fmt.Sprintf("/volumes/%d", id), nil, &env)
	return env.Volume, err
}

func (c *Client) createVolume(req CreateVolumeRequest) (Volume, error) {
	var env struct {
		Volume Volume `json:"volume"`
	}
	err := c.doJSON(http.MethodPost, "/volumes", req, &env)
	return env.Volume, err
}

func (c *Client) deleteVolume(id int) error {
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/volumes/%d", id), nil, nil)
}

// volumeAction attaches or detaches a volume. payload is nil for detach.
func (c *Client) volumeAction(id int, action string, payload any) (Action, error) {
	var env struct {
		Action Action `json:"action"`
	}
	err := c.doJSON(http.MethodPost, fmt.Sprintf("/volumes/%d/actions/%s", id, action), payload, &env)
	return env.Action, err
}

func (c *Client) networks() ([]Network, error) {
	return listAll[Network](c, "/networks", "networks")
}

func (c *Client) network(id int) (Network, error) {
	var env struct {
		Network Network `json:"network"`
	}
	err := c.doJSON(http.MethodGet, fmt.Sprintf("/networks/%d", id), nil, &env)
	return env.Network, err
}

func (c *Client) createNetwork(name, ipRange string) (Network, error) {
	var env struct {
		Network Network `json:"network"`
	}
	body := map[string]string{"name": name, "ip_range": ipRange}
	err := c.doJSON(http.MethodPost, "/networks", body, &env)
	return env.Network, err
}

func (c *Client) deleteNetwork(id int) error {
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/networks/%d", id), nil, nil)
}

func (c *Client) firewalls() ([]Firewall, error) {
	return listAll[Firewall](c, "/firewalls", "firewalls")
}

func (c *Client) firewall(id int) (Firewall, error) {
	var env struct {
		Firewall Firewall `json:"firewall"`
	}
	err := c.doJSON(http.MethodGet, fmt.Sprintf("/firewalls/%d", id), nil, &env)
	return env.Firewall, err
}

// createFirewall accepts an already-assembled body (name plus optional rules)
// so the command layer owns the rule parsing and this method stays a thin POST.
func (c *Client) createFirewall(body map[string]any) (Firewall, error) {
	var env struct {
		Firewall Firewall `json:"firewall"`
	}
	err := c.doJSON(http.MethodPost, "/firewalls", body, &env)
	return env.Firewall, err
}

func (c *Client) deleteFirewall(id int) error {
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/firewalls/%d", id), nil, nil)
}

func (c *Client) sshKeys() ([]SSHKey, error) {
	return listAll[SSHKey](c, "/ssh_keys", "ssh_keys")
}

func (c *Client) sshKey(id int) (SSHKey, error) {
	var env struct {
		SSHKey SSHKey `json:"ssh_key"`
	}
	err := c.doJSON(http.MethodGet, fmt.Sprintf("/ssh_keys/%d", id), nil, &env)
	return env.SSHKey, err
}

func (c *Client) createSSHKey(name, publicKey string) (SSHKey, error) {
	var env struct {
		SSHKey SSHKey `json:"ssh_key"`
	}
	body := map[string]string{"name": name, "public_key": publicKey}
	err := c.doJSON(http.MethodPost, "/ssh_keys", body, &env)
	return env.SSHKey, err
}

func (c *Client) deleteSSHKey(id int) error {
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/ssh_keys/%d", id), nil, nil)
}

// images lists images, optionally filtered by type (system|snapshot|backup|app).
func (c *Client) images(imageType string) ([]Image, error) {
	path := "/images"
	if imageType != "" {
		path += "?type=" + url.QueryEscape(imageType)
	}
	return listAll[Image](c, path, "images")
}

func (c *Client) serverTypes() ([]ServerType, error) {
	return listAll[ServerType](c, "/server_types", "server_types")
}

func (c *Client) locations() ([]Location, error) {
	return listAll[Location](c, "/locations", "locations")
}

func (c *Client) datacenters() ([]Datacenter, error) {
	return listAll[Datacenter](c, "/datacenters", "datacenters")
}

func (c *Client) floatingIPs() ([]FloatingIP, error) {
	return listAll[FloatingIP](c, "/floating_ips", "floating_ips")
}

func (c *Client) primaryIPs() ([]PrimaryIP, error) {
	return listAll[PrimaryIP](c, "/primary_ips", "primary_ips")
}

func (c *Client) loadBalancers() ([]LoadBalancer, error) {
	return listAll[LoadBalancer](c, "/load_balancers", "load_balancers")
}

func (c *Client) loadBalancer(id int) (LoadBalancer, error) {
	var env struct {
		LoadBalancer LoadBalancer `json:"load_balancer"`
	}
	err := c.doJSON(http.MethodGet, fmt.Sprintf("/load_balancers/%d", id), nil, &env)
	return env.LoadBalancer, err
}

func (c *Client) pricing() (Pricing, error) {
	var env struct {
		Pricing Pricing `json:"pricing"`
	}
	err := c.doJSON(http.MethodGet, "/pricing", nil, &env)
	return env.Pricing, err
}

// lookupID resolves a CLI reference to a numeric id: a numeric string passes
// through untouched, otherwise it is matched by exact name against the
// collection (servers, volumes, networks, firewalls, ssh_keys). It is the
// single owner of name→id resolution shared by every addressing command.
func (c *Client) lookupID(collection, ref string) (int, error) {
	if id, err := strconv.Atoi(ref); err == nil {
		return id, nil
	}
	// The list response also carries a "meta" object, so decode into raw
	// messages and unmarshal only the collection array — never the whole map.
	var env map[string]json.RawMessage
	path := "/" + collection + "?name=" + url.QueryEscape(ref)
	if err := c.doJSON(http.MethodGet, path, nil, &env); err != nil {
		return 0, err
	}
	var items []idName
	if raw, ok := env[collection]; ok {
		if err := json.Unmarshal(raw, &items); err != nil {
			return 0, fmt.Errorf("decode %s list: %w", collection, err)
		}
	}
	if len(items) == 0 {
		return 0, fmt.Errorf("no %s named %q", strings.TrimSuffix(collection, "s"), ref)
	}
	return items[0].ID, nil
}

type idName struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// --- HTTP plumbing ---------------------------------------------------------

// newRequest builds a signed request against baseURL+path. It is the single
// owner of the bearer auth scheme and the JSON Accept/Content-Type headers, so
// both the typed doJSON path and the raw `api` passthrough sign identically and
// the scheme lives in exactly one place.
func (c *Client) newRequest(method, path string, body io.Reader, hasBody bool) (*http.Request, error) {
	req, err := http.NewRequest(method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	if hasBody {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	return req, nil
}

// doJSON executes a request against baseURL+path. A non-nil body is JSON-encoded
// and sent; a non-nil out receives the decoded JSON response. A non-2xx status
// surfaces Hetzner's error envelope so failures are never silent.
func (c *Client) doJSON(method, path string, body any, out any) error {
	var reader io.Reader
	hasBody := body != nil
	if hasBody {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := c.newRequest(method, path, reader, hasBody)
	if err != nil {
		return err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("hetzner %s: %s", resp.Status, hetznerError(respBody))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// raw executes an arbitrary signed request and returns the status and body
// without treating a non-2xx status as an error. It backs the low-level `api`
// passthrough so callers can inspect error bodies while exploring endpoints the
// typed methods do not cover.
func (c *Client) raw(method, path string, body []byte) (int, []byte, error) {
	var reader io.Reader
	hasBody := len(body) > 0
	if hasBody {
		reader = bytes.NewReader(body)
	}
	req, err := c.newRequest(method, path, reader, hasBody)
	if err != nil {
		return 0, nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, respBody, nil
}

// hetznerError pulls the human message out of Hetzner's {"error":{...}}
// envelope, falling back to the trimmed raw body when it does not match.
func hetznerError(body []byte) string {
	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &env) == nil && env.Error.Message != "" {
		if env.Error.Code != "" {
			return fmt.Sprintf("%s (%s)", env.Error.Message, env.Error.Code)
		}
		return env.Error.Message
	}
	return strings.TrimSpace(string(body))
}
