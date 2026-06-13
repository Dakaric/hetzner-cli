package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// This file holds the per-resource command handlers. main.go owns dispatch and
// the shared harness (flag parsing, output, the destructive-op guard); each
// handler here turns parsed args into one or more typed client calls and
// renders the result. Writes go through requireYes before touching state.

// showOrUnknown handles the default arm of a resource dispatcher. A "show" only
// ever takes a single positional reference, so two or more positionals mean the
// first token was almost certainly a mistyped subcommand — surface that with the
// valid verbs instead of a confusing "no <resource> named <verb>" lookup error.
func showOrUnknown(args []string, verbs string, show func()) {
	if len(parseOpts(args).pos) > 1 {
		fail(fmt.Errorf("unknown subcommand %q; expected one of: %s — or <id|name> to show one", args[0], verbs))
	}
	show()
}

// --- Servers ---------------------------------------------------------------

func cmdServers(c *Client, args []string) {
	o := parseOpts(args)
	servers, err := c.servers()
	if err != nil {
		fail(err)
	}
	if o.json() {
		printJSON(servers)
		return
	}
	for _, s := range servers {
		fmt.Println(renderServerLine(s))
	}
	if len(servers) == 0 {
		fmt.Fprintln(os.Stderr, "no servers")
	}
}

func cmdServer(c *Client, args []string) {
	if len(args) == 0 {
		fail(fmt.Errorf("usage: hetzner server <id|name> | create … | delete <id|name> | reboot|poweron|poweroff|shutdown|reset <id|name>"))
	}
	switch sub := args[0]; sub {
	case "create":
		createServerCmd(c, args[1:])
	case "delete", "rm":
		deleteServerCmd(c, args[1:])
	case "reboot", "poweron", "poweroff", "shutdown", "reset":
		serverActionCmd(c, sub, args[1:])
	default:
		showOrUnknown(args, "create, delete, reboot, poweron, poweroff, shutdown, reset", func() { showServerCmd(c, args) })
	}
}

func showServerCmd(c *Client, args []string) {
	o := parseOpts(args)
	ref := firstPos(o)
	id, err := c.lookupID("servers", ref)
	if err != nil {
		fail(err)
	}
	s, err := c.server(id)
	if err != nil {
		fail(err)
	}
	if o.json() {
		printJSON(s)
		return
	}
	fmt.Println(renderServerDetail(s))
}

func createServerCmd(c *Client, args []string) {
	o := parseOpts(args)
	name := firstNonEmpty(o.get("name"), posAt(o.pos, 0))
	serverType, image := o.get("type"), o.get("image")
	if name == "" || serverType == "" || image == "" {
		fail(fmt.Errorf("usage: hetzner server create --name <n> --type <cx22> --image <ubuntu-24.04> [--location fsn1 | --datacenter fsn1-dc14] [--ssh-key <name|id> …] [--no-start]"))
	}
	req := CreateServerRequest{
		Name:             name,
		ServerType:       serverType,
		Image:            image,
		Location:         o.get("location"),
		Datacenter:       o.get("datacenter"),
		SSHKeys:          o.sshKeys,
		StartAfterCreate: !o.bool("no-start"),
	}
	server, rootPassword, err := c.createServer(req)
	if err != nil {
		fail(err)
	}
	if o.json() {
		printJSON(map[string]any{"server": server, "root_password": rootPassword})
		return
	}
	fmt.Println(renderServerDetail(server))
	if rootPassword != "" {
		fmt.Printf("\nroot password: %s\n(no SSH key set — Hetzner shows this only once, store it now)\n", rootPassword)
	}
}

func deleteServerCmd(c *Client, args []string) {
	o := parseOpts(args)
	id, err := c.lookupID("servers", firstPos(o))
	if err != nil {
		fail(err)
	}
	if err := requireYes(o, fmt.Sprintf("delete server %d", id)); err != nil {
		fail(err)
	}
	if err := c.deleteServer(id); err != nil {
		fail(err)
	}
	fmt.Printf("deleted server %d\n", id)
}

func serverActionCmd(c *Client, action string, args []string) {
	o := parseOpts(args)
	id, err := c.lookupID("servers", firstPos(o))
	if err != nil {
		fail(err)
	}
	if action == "poweroff" || action == "reset" {
		if err := requireYes(o, fmt.Sprintf("%s server %d (hard, may lose unsaved data)", action, id)); err != nil {
			fail(err)
		}
	}
	act, err := c.serverAction(id, action)
	if err != nil {
		fail(err)
	}
	if o.json() {
		printJSON(act)
		return
	}
	fmt.Println(renderAction(action, id, act))
}

// --- Volumes ---------------------------------------------------------------

func cmdVolumes(c *Client, args []string) {
	o := parseOpts(args)
	volumes, err := c.volumes()
	if err != nil {
		fail(err)
	}
	if o.json() {
		printJSON(volumes)
		return
	}
	for _, v := range volumes {
		fmt.Println(renderVolumeLine(v))
	}
	if len(volumes) == 0 {
		fmt.Fprintln(os.Stderr, "no volumes")
	}
}

func cmdVolume(c *Client, args []string) {
	if len(args) == 0 {
		fail(fmt.Errorf("usage: hetzner volume <id|name> | create … | delete <id|name> | attach <id|name> --server <id|name> | detach <id|name>"))
	}
	switch sub := args[0]; sub {
	case "create":
		createVolumeCmd(c, args[1:])
	case "delete", "rm":
		deleteVolumeCmd(c, args[1:])
	case "attach":
		attachVolumeCmd(c, args[1:])
	case "detach":
		detachVolumeCmd(c, args[1:])
	default:
		showOrUnknown(args, "create, delete, attach, detach", func() { showVolumeCmd(c, args) })
	}
}

func showVolumeCmd(c *Client, args []string) {
	o := parseOpts(args)
	id, err := c.lookupID("volumes", firstPos(o))
	if err != nil {
		fail(err)
	}
	v, err := c.volume(id)
	if err != nil {
		fail(err)
	}
	if o.json() {
		printJSON(v)
		return
	}
	fmt.Println(renderVolumeDetail(v))
}

func createVolumeCmd(c *Client, args []string) {
	o := parseOpts(args)
	name := firstNonEmpty(o.get("name"), posAt(o.pos, 0))
	sizeStr := o.get("size")
	size, err := strconv.Atoi(sizeStr)
	if sizeStr != "" && err != nil {
		fail(fmt.Errorf("--size must be a whole number of GB, got %q", sizeStr))
	}
	if name == "" || size <= 0 {
		fail(fmt.Errorf("usage: hetzner volume create --name <n> --size <GB> (--location fsn1 | --server <id|name>) [--format ext4] [--automount]"))
	}
	req := CreateVolumeRequest{
		Name:      name,
		Size:      size,
		Location:  o.get("location"),
		Format:    o.get("format"),
		Automount: o.bool("automount"),
	}
	if ref := o.get("server"); ref != "" {
		id, err := c.lookupID("servers", ref)
		if err != nil {
			fail(err)
		}
		req.Server = &id
	}
	if req.Location == "" && req.Server == nil {
		fail(fmt.Errorf("volume create needs either --location or --server"))
	}
	v, err := c.createVolume(req)
	if err != nil {
		fail(err)
	}
	if o.json() {
		printJSON(v)
		return
	}
	fmt.Println(renderVolumeDetail(v))
}

func deleteVolumeCmd(c *Client, args []string) {
	o := parseOpts(args)
	id, err := c.lookupID("volumes", firstPos(o))
	if err != nil {
		fail(err)
	}
	if err := requireYes(o, fmt.Sprintf("delete volume %d", id)); err != nil {
		fail(err)
	}
	if err := c.deleteVolume(id); err != nil {
		fail(err)
	}
	fmt.Printf("deleted volume %d\n", id)
}

func attachVolumeCmd(c *Client, args []string) {
	o := parseOpts(args)
	volID, err := c.lookupID("volumes", firstPos(o))
	if err != nil {
		fail(err)
	}
	serverRef := o.get("server")
	if serverRef == "" {
		fail(fmt.Errorf("usage: hetzner volume attach <id|name> --server <id|name> [--automount]"))
	}
	serverID, err := c.lookupID("servers", serverRef)
	if err != nil {
		fail(err)
	}
	payload := map[string]any{"server": serverID, "automount": o.bool("automount")}
	act, err := c.volumeAction(volID, "attach", payload)
	if err != nil {
		fail(err)
	}
	if o.json() {
		printJSON(act)
		return
	}
	fmt.Println(renderAction("attach volume", volID, act))
}

func detachVolumeCmd(c *Client, args []string) {
	o := parseOpts(args)
	volID, err := c.lookupID("volumes", firstPos(o))
	if err != nil {
		fail(err)
	}
	act, err := c.volumeAction(volID, "detach", nil)
	if err != nil {
		fail(err)
	}
	if o.json() {
		printJSON(act)
		return
	}
	fmt.Println(renderAction("detach volume", volID, act))
}

// --- Networks --------------------------------------------------------------

func cmdNetworks(c *Client, args []string) {
	o := parseOpts(args)
	networks, err := c.networks()
	if err != nil {
		fail(err)
	}
	if o.json() {
		printJSON(networks)
		return
	}
	for _, n := range networks {
		fmt.Println(renderNetworkLine(n))
	}
	if len(networks) == 0 {
		fmt.Fprintln(os.Stderr, "no networks")
	}
}

func cmdNetwork(c *Client, args []string) {
	if len(args) == 0 {
		fail(fmt.Errorf("usage: hetzner network <id|name> | create --name <n> --ip-range <10.0.0.0/16> | delete <id|name>"))
	}
	switch sub := args[0]; sub {
	case "create":
		createNetworkCmd(c, args[1:])
	case "delete", "rm":
		deleteNetworkCmd(c, args[1:])
	default:
		showOrUnknown(args, "create, delete", func() { showNetworkCmd(c, args) })
	}
}

func showNetworkCmd(c *Client, args []string) {
	o := parseOpts(args)
	id, err := c.lookupID("networks", firstPos(o))
	if err != nil {
		fail(err)
	}
	n, err := c.network(id)
	if err != nil {
		fail(err)
	}
	if o.json() {
		printJSON(n)
		return
	}
	fmt.Println(renderNetworkDetail(n))
}

func createNetworkCmd(c *Client, args []string) {
	o := parseOpts(args)
	name := firstNonEmpty(o.get("name"), posAt(o.pos, 0))
	ipRange := o.get("ip-range")
	if name == "" || ipRange == "" {
		fail(fmt.Errorf("usage: hetzner network create --name <n> --ip-range <10.0.0.0/16>"))
	}
	n, err := c.createNetwork(name, ipRange)
	if err != nil {
		fail(err)
	}
	if o.json() {
		printJSON(n)
		return
	}
	fmt.Println(renderNetworkDetail(n))
}

func deleteNetworkCmd(c *Client, args []string) {
	o := parseOpts(args)
	id, err := c.lookupID("networks", firstPos(o))
	if err != nil {
		fail(err)
	}
	if err := requireYes(o, fmt.Sprintf("delete network %d", id)); err != nil {
		fail(err)
	}
	if err := c.deleteNetwork(id); err != nil {
		fail(err)
	}
	fmt.Printf("deleted network %d\n", id)
}

// --- Firewalls -------------------------------------------------------------

func cmdFirewalls(c *Client, args []string) {
	o := parseOpts(args)
	firewalls, err := c.firewalls()
	if err != nil {
		fail(err)
	}
	if o.json() {
		printJSON(firewalls)
		return
	}
	for _, f := range firewalls {
		fmt.Println(renderFirewallLine(f))
	}
	if len(firewalls) == 0 {
		fmt.Fprintln(os.Stderr, "no firewalls")
	}
}

func cmdFirewall(c *Client, args []string) {
	if len(args) == 0 {
		fail(fmt.Errorf(`usage: hetzner firewall <id|name> | create --name <n> [--rules '<json-array>'] | delete <id|name>`))
	}
	switch sub := args[0]; sub {
	case "create":
		createFirewallCmd(c, args[1:])
	case "delete", "rm":
		deleteFirewallCmd(c, args[1:])
	default:
		showOrUnknown(args, "create, delete", func() { showFirewallCmd(c, args) })
	}
}

func showFirewallCmd(c *Client, args []string) {
	o := parseOpts(args)
	id, err := c.lookupID("firewalls", firstPos(o))
	if err != nil {
		fail(err)
	}
	f, err := c.firewall(id)
	if err != nil {
		fail(err)
	}
	if o.json() {
		printJSON(f)
		return
	}
	fmt.Println(renderFirewallDetail(f))
}

// createFirewallCmd builds the firewall body. Rules are complex, so they are
// passed verbatim as a JSON array via --rules and parsed here; the client just
// POSTs the assembled body.
func createFirewallCmd(c *Client, args []string) {
	o := parseOpts(args)
	name := firstNonEmpty(o.get("name"), posAt(o.pos, 0))
	if name == "" {
		fail(fmt.Errorf(`usage: hetzner firewall create --name <n> [--rules '[{"direction":"in","protocol":"tcp","port":"22","source_ips":["0.0.0.0/0","::/0"]}]']`))
	}
	body := map[string]any{"name": name}
	if rules := o.get("rules"); rules != "" {
		parsed, err := parseJSONArray(rules)
		if err != nil {
			fail(fmt.Errorf("--rules is not valid JSON: %w", err))
		}
		body["rules"] = parsed
	}
	f, err := c.createFirewall(body)
	if err != nil {
		fail(err)
	}
	if o.json() {
		printJSON(f)
		return
	}
	fmt.Println(renderFirewallDetail(f))
}

func deleteFirewallCmd(c *Client, args []string) {
	o := parseOpts(args)
	id, err := c.lookupID("firewalls", firstPos(o))
	if err != nil {
		fail(err)
	}
	if err := requireYes(o, fmt.Sprintf("delete firewall %d", id)); err != nil {
		fail(err)
	}
	if err := c.deleteFirewall(id); err != nil {
		fail(err)
	}
	fmt.Printf("deleted firewall %d\n", id)
}

// --- SSH keys --------------------------------------------------------------

func cmdSSHKeys(c *Client, args []string) {
	o := parseOpts(args)
	keys, err := c.sshKeys()
	if err != nil {
		fail(err)
	}
	if o.json() {
		printJSON(keys)
		return
	}
	for _, k := range keys {
		fmt.Println(renderSSHKeyLine(k))
	}
	if len(keys) == 0 {
		fmt.Fprintln(os.Stderr, "no ssh keys")
	}
}

func cmdSSHKey(c *Client, args []string) {
	if len(args) == 0 {
		fail(fmt.Errorf("usage: hetzner ssh-key <id|name> | create --name <n> (--public-key '<key>' | --public-key-file <path>) | delete <id|name>"))
	}
	switch sub := args[0]; sub {
	case "create":
		createSSHKeyCmd(c, args[1:])
	case "delete", "rm":
		deleteSSHKeyCmd(c, args[1:])
	default:
		showOrUnknown(args, "create, delete", func() { showSSHKeyCmd(c, args) })
	}
}

func showSSHKeyCmd(c *Client, args []string) {
	o := parseOpts(args)
	id, err := c.lookupID("ssh_keys", firstPos(o))
	if err != nil {
		fail(err)
	}
	k, err := c.sshKey(id)
	if err != nil {
		fail(err)
	}
	if o.json() {
		printJSON(k)
		return
	}
	fmt.Println(renderSSHKeyDetail(k))
}

func createSSHKeyCmd(c *Client, args []string) {
	o := parseOpts(args)
	name := firstNonEmpty(o.get("name"), posAt(o.pos, 0))
	publicKey := o.get("public-key")
	if path := o.get("public-key-file"); path != "" {
		data, err := os.ReadFile(expandHome(path))
		if err != nil {
			fail(fmt.Errorf("read public key file: %w", err))
		}
		publicKey = string(data)
	}
	// Normalize at the source so the API, --json output, and the text renderer
	// all see the same canonical key — a key file carries a trailing newline.
	publicKey = strings.TrimSpace(publicKey)
	if name == "" || publicKey == "" {
		fail(fmt.Errorf("usage: hetzner ssh-key create --name <n> (--public-key '<key>' | --public-key-file <path>)"))
	}
	k, err := c.createSSHKey(name, publicKey)
	if err != nil {
		fail(err)
	}
	if o.json() {
		printJSON(k)
		return
	}
	fmt.Println(renderSSHKeyDetail(k))
}

func deleteSSHKeyCmd(c *Client, args []string) {
	o := parseOpts(args)
	id, err := c.lookupID("ssh_keys", firstPos(o))
	if err != nil {
		fail(err)
	}
	if err := requireYes(o, fmt.Sprintf("delete ssh key %d", id)); err != nil {
		fail(err)
	}
	if err := c.deleteSSHKey(id); err != nil {
		fail(err)
	}
	fmt.Printf("deleted ssh key %d\n", id)
}

// --- Catalogs (read-only) --------------------------------------------------

func cmdImages(c *Client, args []string) {
	o := parseOpts(args)
	images, err := c.images(o.get("type"))
	if err != nil {
		fail(err)
	}
	if o.json() {
		printJSON(images)
		return
	}
	for _, i := range images {
		fmt.Println(renderImageLine(i))
	}
	if len(images) == 0 {
		fmt.Fprintln(os.Stderr, "no images")
	}
}

func cmdServerTypes(c *Client, args []string) {
	o := parseOpts(args)
	types, err := c.serverTypes()
	if err != nil {
		fail(err)
	}
	if o.json() {
		printJSON(types)
		return
	}
	for _, t := range types {
		fmt.Println(renderServerTypeLine(t))
	}
	if len(types) == 0 {
		fmt.Fprintln(os.Stderr, "no server types")
	}
}

func cmdLocations(c *Client, args []string) {
	o := parseOpts(args)
	locations, err := c.locations()
	if err != nil {
		fail(err)
	}
	if o.json() {
		printJSON(locations)
		return
	}
	for _, l := range locations {
		fmt.Println(renderLocationLine(l))
	}
	if len(locations) == 0 {
		fmt.Fprintln(os.Stderr, "no locations")
	}
}

func cmdDatacenters(c *Client, args []string) {
	o := parseOpts(args)
	datacenters, err := c.datacenters()
	if err != nil {
		fail(err)
	}
	if o.json() {
		printJSON(datacenters)
		return
	}
	for _, d := range datacenters {
		fmt.Println(renderDatacenterLine(d))
	}
	if len(datacenters) == 0 {
		fmt.Fprintln(os.Stderr, "no datacenters")
	}
}

func cmdFloatingIPs(c *Client, args []string) {
	o := parseOpts(args)
	ips, err := c.floatingIPs()
	if err != nil {
		fail(err)
	}
	if o.json() {
		printJSON(ips)
		return
	}
	for _, ip := range ips {
		fmt.Println(renderFloatingIPLine(ip))
	}
	if len(ips) == 0 {
		fmt.Fprintln(os.Stderr, "no floating ips")
	}
}

func cmdPrimaryIPs(c *Client, args []string) {
	o := parseOpts(args)
	ips, err := c.primaryIPs()
	if err != nil {
		fail(err)
	}
	if o.json() {
		printJSON(ips)
		return
	}
	for _, ip := range ips {
		fmt.Println(renderPrimaryIPLine(ip))
	}
	if len(ips) == 0 {
		fmt.Fprintln(os.Stderr, "no primary ips")
	}
}

func cmdLoadBalancers(c *Client, args []string) {
	o := parseOpts(args)
	if len(o.pos) > 0 {
		id, err := c.lookupID("load_balancers", o.pos[0])
		if err != nil {
			fail(err)
		}
		lb, err := c.loadBalancer(id)
		if err != nil {
			fail(err)
		}
		if o.json() {
			printJSON(lb)
			return
		}
		fmt.Println(renderLoadBalancerDetail(lb))
		return
	}
	lbs, err := c.loadBalancers()
	if err != nil {
		fail(err)
	}
	if o.json() {
		printJSON(lbs)
		return
	}
	for _, lb := range lbs {
		fmt.Println(renderLoadBalancerLine(lb))
	}
	if len(lbs) == 0 {
		fmt.Fprintln(os.Stderr, "no load balancers")
	}
}

func cmdPricing(c *Client, args []string) {
	o := parseOpts(args)
	p, err := c.pricing()
	if err != nil {
		fail(err)
	}
	if o.json() {
		printJSON(p)
		return
	}
	fmt.Println(renderPricing(p))
}

// cmdStatus is a quick connection test and inventory summary: it counts the
// main resources, which also proves the token works against the live API.
func cmdStatus(c *Client, args []string) {
	o := parseOpts(args)
	// Every count is fatal on error, like every other command: a status whose
	// job is to prove the token works must never coerce a failed call into a
	// misleading "0" — empty and failed have to look different.
	servers, err := c.servers()
	if err != nil {
		fail(err)
	}
	volumes, err := c.volumes()
	if err != nil {
		fail(err)
	}
	networks, err := c.networks()
	if err != nil {
		fail(err)
	}
	firewalls, err := c.firewalls()
	if err != nil {
		fail(err)
	}
	floating, err := c.floatingIPs()
	if err != nil {
		fail(err)
	}

	running := 0
	for _, s := range servers {
		if s.Status == "running" {
			running++
		}
	}
	summary := map[string]int{
		"servers": len(servers), "servers_running": running,
		"volumes": len(volumes), "networks": len(networks),
		"firewalls": len(firewalls), "floating_ips": len(floating),
	}
	if o.json() {
		printJSON(summary)
		return
	}
	fmt.Printf("servers:      %d (%d running)\n", len(servers), running)
	fmt.Printf("volumes:      %d\n", len(volumes))
	fmt.Printf("networks:     %d\n", len(networks))
	fmt.Printf("firewalls:    %d\n", len(firewalls))
	fmt.Printf("floating ips: %d\n", len(floating))
}
