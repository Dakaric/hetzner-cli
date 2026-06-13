package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// fmtTime renders an RFC3339 timestamp in local time, falling back to the raw
// string if it cannot be parsed.
func fmtTime(iso string) string {
	if iso == "" {
		return "-"
	}
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return iso
	}
	return t.Local().Format("2006-01-02 15:04")
}

// labelsText renders a label map as sorted k=v pairs, or "-" when empty.
func labelsText(labels map[string]string) string {
	if len(labels) == 0 {
		return "-"
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+labels[k])
	}
	return strings.Join(parts, ", ")
}

func boolText(b bool, yes, no string) string {
	if b {
		return yes
	}
	return no
}

// --- Servers ---------------------------------------------------------------

func renderServerLine(s Server) string {
	return fmt.Sprintf("#%-9d %-22s %-9s %-7s %-15s %s",
		s.ID, s.Name, s.Status, s.ServerType.Name, orDash(s.PublicNet.IPv4.IP), s.Datacenter.Location.Name)
}

func renderServerDetail(s Server) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#%d  %s\n", s.ID, s.Name)
	fmt.Fprintf(&b, "  status:     %s%s\n", s.Status, boolText(s.Locked, " (locked)", ""))
	fmt.Fprintf(&b, "  type:       %s — %d vCPU, %s GB RAM, %d GB disk (%s)\n",
		s.ServerType.Name, s.ServerType.Cores, trimFloat(s.ServerType.Memory), s.ServerType.Disk, s.ServerType.CPUType)
	if s.Image != nil {
		fmt.Fprintf(&b, "  image:      %s\n", orDash(s.Image.Name))
	}
	fmt.Fprintf(&b, "  ipv4:       %s\n", orDash(s.PublicNet.IPv4.IP))
	fmt.Fprintf(&b, "  ipv6:       %s\n", orDash(s.PublicNet.IPv6.IP))
	for _, p := range s.PrivateNet {
		fmt.Fprintf(&b, "  private:    %s (network %d)\n", p.IP, p.Network)
	}
	fmt.Fprintf(&b, "  location:   %s / %s (%s, %s)\n",
		s.Datacenter.Location.Name, s.Datacenter.Name, s.Datacenter.Location.City, s.Datacenter.Location.Country)
	if len(s.Volumes) > 0 {
		fmt.Fprintf(&b, "  volumes:    %s\n", joinInts(s.Volumes))
	}
	fmt.Fprintf(&b, "  protection: delete=%s rebuild=%s\n",
		boolText(s.Protection.Delete, "on", "off"), boolText(s.Protection.Rebuild, "on", "off"))
	fmt.Fprintf(&b, "  labels:     %s\n", labelsText(s.Labels))
	fmt.Fprintf(&b, "  created:    %s", fmtTime(s.Created))
	return b.String()
}

// --- Volumes ---------------------------------------------------------------

func renderVolumeLine(v Volume) string {
	attached := "unattached"
	if v.Server != nil {
		attached = fmt.Sprintf("server %d", *v.Server)
	}
	return fmt.Sprintf("#%-9d %-22s %-8s %4d GB  %-12s %s",
		v.ID, v.Name, v.Status, v.Size, v.Location.Name, attached)
}

func renderVolumeDetail(v Volume) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#%d  %s\n", v.ID, v.Name)
	fmt.Fprintf(&b, "  status:     %s\n", v.Status)
	fmt.Fprintf(&b, "  size:       %d GB\n", v.Size)
	fmt.Fprintf(&b, "  location:   %s\n", v.Location.Name)
	if v.Server != nil {
		fmt.Fprintf(&b, "  attached:   server %d at %s\n", *v.Server, orDash(v.LinuxDevice))
	} else {
		fmt.Fprintf(&b, "  attached:   no\n")
	}
	if v.Format != nil {
		fmt.Fprintf(&b, "  format:     %s\n", *v.Format)
	}
	fmt.Fprintf(&b, "  protection: delete=%s\n", boolText(v.Protection.Delete, "on", "off"))
	fmt.Fprintf(&b, "  labels:     %s\n", labelsText(v.Labels))
	fmt.Fprintf(&b, "  created:    %s", fmtTime(v.Created))
	return b.String()
}

// --- Networks --------------------------------------------------------------

func renderNetworkLine(n Network) string {
	return fmt.Sprintf("#%-9d %-22s %-18s %d subnet(s)  %d server(s)",
		n.ID, n.Name, n.IPRange, len(n.Subnets), len(n.Servers))
}

func renderNetworkDetail(n Network) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#%d  %s\n", n.ID, n.Name)
	fmt.Fprintf(&b, "  ip range:   %s\n", n.IPRange)
	for _, s := range n.Subnets {
		fmt.Fprintf(&b, "  subnet:     %s (%s, zone %s, gw %s)\n", s.IPRange, s.Type, s.NetworkZone, orDash(s.Gateway))
	}
	for _, r := range n.Routes {
		fmt.Fprintf(&b, "  route:      %s -> %s\n", r.Destination, r.Gateway)
	}
	if len(n.Servers) > 0 {
		fmt.Fprintf(&b, "  servers:    %s\n", joinInts(n.Servers))
	}
	fmt.Fprintf(&b, "  labels:     %s\n", labelsText(n.Labels))
	fmt.Fprintf(&b, "  created:    %s", fmtTime(n.Created))
	return b.String()
}

// --- Firewalls -------------------------------------------------------------

func renderFirewallLine(f Firewall) string {
	return fmt.Sprintf("#%-9d %-22s %d rule(s)  applied to %d resource(s)",
		f.ID, f.Name, len(f.Rules), len(f.AppliedTo))
}

func renderFirewallDetail(f Firewall) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#%d  %s\n", f.ID, f.Name)
	for _, r := range f.Rules {
		peers := strings.Join(append(r.SourceIPs, r.DestinationIPs...), ", ")
		fmt.Fprintf(&b, "  rule:       %s %s port %s  [%s]\n",
			r.Direction, orDash(r.Protocol), orDash(r.Port), orDash(peers))
	}
	fmt.Fprintf(&b, "  applied to: %d resource(s)\n", len(f.AppliedTo))
	fmt.Fprintf(&b, "  labels:     %s\n", labelsText(f.Labels))
	fmt.Fprintf(&b, "  created:    %s", fmtTime(f.Created))
	return b.String()
}

// --- SSH keys --------------------------------------------------------------

func renderSSHKeyLine(k SSHKey) string {
	return fmt.Sprintf("#%-9d %-24s %s", k.ID, k.Name, k.Fingerprint)
}

func renderSSHKeyDetail(k SSHKey) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#%d  %s\n", k.ID, k.Name)
	fmt.Fprintf(&b, "  fingerprint: %s\n", k.Fingerprint)
	fmt.Fprintf(&b, "  labels:      %s\n", labelsText(k.Labels))
	fmt.Fprintf(&b, "  created:     %s\n", fmtTime(k.Created))
	fmt.Fprintf(&b, "  public key:  %s", strings.TrimSpace(k.PublicKey))
	return b.String()
}

// --- Catalogs --------------------------------------------------------------

func renderImageLine(i Image) string {
	name := orDash(firstNonEmpty(i.Name, i.Description))
	return fmt.Sprintf("#%-9d %-26s %-9s %-12s %s",
		i.ID, name, i.Type, i.Architecture, orDash(i.Description))
}

func renderServerTypeLine(t ServerType) string {
	price := "-"
	if len(t.Prices) > 0 {
		price = money(t.Prices[0].PriceMonthly.Gross) + "/mo (" + t.Prices[0].Location + ")"
	}
	dep := ""
	if t.Deprecated {
		dep = " [deprecated]"
	}
	return fmt.Sprintf("%-10s %d vCPU  %s GB  %3d GB  %-9s %s%s",
		t.Name, t.Cores, trimFloat(t.Memory), t.Disk, t.CPUType, price, dep)
}

func renderLocationLine(l Location) string {
	return fmt.Sprintf("%-6s %-14s %-3s zone=%s  (%s)", l.Name, l.City, l.Country, l.NetworkZone, l.Description)
}

func renderDatacenterLine(d Datacenter) string {
	return fmt.Sprintf("%-12s %-6s %s", d.Name, d.Location.Name, d.Description)
}

func renderFloatingIPLine(f FloatingIP) string {
	assigned := "unassigned"
	if f.Server != nil {
		assigned = fmt.Sprintf("server %d", *f.Server)
	}
	return fmt.Sprintf("#%-9d %-6s %-22s %-12s %s",
		f.ID, f.Type, f.IP, f.HomeLocation.Name, assigned)
}

func renderPrimaryIPLine(p PrimaryIP) string {
	assigned := "unassigned"
	if p.AssigneeID != nil {
		assigned = fmt.Sprintf("%s %d", orDash(p.AssigneeType), *p.AssigneeID)
	}
	return fmt.Sprintf("#%-9d %-6s %-22s %-12s %s",
		p.ID, p.Type, p.IP, p.Datacenter.Name, assigned)
}

func renderLoadBalancerLine(lb LoadBalancer) string {
	return fmt.Sprintf("#%-9d %-22s %-15s %-8s %d service(s)  %d target(s)",
		lb.ID, lb.Name, orDash(lb.PublicNet.IPv4.IP), lb.Type.Name, len(lb.Services), len(lb.Targets))
}

func renderLoadBalancerDetail(lb LoadBalancer) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#%d  %s\n", lb.ID, lb.Name)
	fmt.Fprintf(&b, "  type:       %s\n", lb.Type.Name)
	fmt.Fprintf(&b, "  ipv4:       %s\n", orDash(lb.PublicNet.IPv4.IP))
	fmt.Fprintf(&b, "  ipv6:       %s\n", orDash(lb.PublicNet.IPv6.IP))
	fmt.Fprintf(&b, "  location:   %s\n", lb.Location.Name)
	for _, s := range lb.Services {
		fmt.Fprintf(&b, "  service:    %s %d -> %d\n", s.Protocol, s.ListenPort, s.DestinationPort)
	}
	fmt.Fprintf(&b, "  targets:    %d\n", len(lb.Targets))
	fmt.Fprintf(&b, "  created:    %s", fmtTime(lb.Created))
	return b.String()
}

func renderPricing(p Pricing) string {
	var b strings.Builder
	fmt.Fprintf(&b, "currency:   %s  (VAT %s%%)\n", p.Currency, p.VATRate)
	fmt.Fprintf(&b, "volume:     %s gross / GB·month\n", money(p.Volume.PricePerGBMonth.Gross))
	fmt.Fprintf(&b, "image:      %s gross / GB·month\n", money(p.Image.PricePerGBMonth.Gross))
	fmt.Fprintf(&b, "traffic:    %s gross / TB\n", money(p.Traffic.PricePerTB.Gross))
	fmt.Fprintln(&b, "server types (first listed location, gross/month):")
	for _, st := range p.ServerTypes {
		if len(st.Prices) == 0 {
			continue
		}
		fmt.Fprintf(&b, "  %-10s %s\n", st.Name, money(st.Prices[0].PriceMonthly.Gross))
	}
	return strings.TrimRight(b.String(), "\n")
}

// --- shared helpers --------------------------------------------------------

// trimFloat renders a GB float without a trailing ".0" so 4.0 reads as "4".
func trimFloat(f float64) string {
	s := fmt.Sprintf("%.1f", f)
	return strings.TrimSuffix(s, ".0")
}

// money trims Hetzner's 16-decimal price strings to their significant digits
// (at most four places), so "7.1281000000000000" reads as "7.1281". The raw
// value is preserved on parse failure and always available via --json.
func money(s string) string {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return s
	}
	out := strconv.FormatFloat(f, 'f', 4, 64)
	out = strings.TrimRight(out, "0")
	return strings.TrimRight(out, ".")
}

func joinInts(ids []int) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("%d", id)
	}
	return strings.Join(parts, ", ")
}

// renderAction summarizes an asynchronous action for the text output.
func renderAction(verb string, id int, a Action) string {
	if a.Error != nil {
		return fmt.Sprintf("%s %d: %s — error: %s (%s)", verb, id, a.Command, a.Error.Message, a.Error.Code)
	}
	return fmt.Sprintf("%s %d: %s (%s)", verb, id, a.Command, a.Status)
}
