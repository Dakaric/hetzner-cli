package main

import (
	"strings"
	"testing"
	"time"
)

func TestRenderServerLineHasKeyFields(t *testing.T) {
	s := Server{
		ID:         42,
		Name:       "web",
		Status:     "running",
		ServerType: ServerType{Name: "cx22"},
		PublicNet:  PublicNet{IPv4: IPv4{IP: "1.2.3.4"}},
		Datacenter: Datacenter{Location: Location{Name: "fsn1"}},
	}
	line := renderServerLine(s)
	for _, want := range []string{"42", "web", "running", "cx22", "1.2.3.4", "fsn1"} {
		if !strings.Contains(line, want) {
			t.Errorf("server line %q missing %q", line, want)
		}
	}
}

func TestRenderServerDetailShowsRootlessFields(t *testing.T) {
	s := Server{ID: 1, Name: "db", Status: "off", ServerType: ServerType{Name: "cx22", Cores: 2, Memory: 4, Disk: 40, CPUType: "shared"}}
	detail := renderServerDetail(s)
	if !strings.Contains(detail, "off") || !strings.Contains(detail, "2 vCPU") {
		t.Errorf("detail missing status/type: %q", detail)
	}
}

func TestFmtTime(t *testing.T) {
	if got := fmtTime(""); got != "-" {
		t.Errorf("fmtTime(empty) = %q, want -", got)
	}
	if got := fmtTime("not-a-time"); got != "not-a-time" {
		t.Errorf("fmtTime(garbage) should pass through, got %q", got)
	}
	iso := "2026-06-13T08:30:00+00:00"
	want := mustParse(t, iso).Local().Format("2006-01-02 15:04")
	if got := fmtTime(iso); got != want {
		t.Errorf("fmtTime(%q) = %q, want %q", iso, got, want)
	}
}

func mustParse(t *testing.T, iso string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return parsed
}

func TestLabelsText(t *testing.T) {
	if got := labelsText(nil); got != "-" {
		t.Errorf("empty labels = %q, want -", got)
	}
	got := labelsText(map[string]string{"env": "prod", "app": "api"})
	if got != "app=api, env=prod" {
		t.Errorf("labels = %q, want sorted app=api, env=prod", got)
	}
}

func TestTrimFloat(t *testing.T) {
	cases := map[float64]string{4.0: "4", 7.5: "7.5", 16.0: "16"}
	for in, want := range cases {
		if got := trimFloat(in); got != want {
			t.Errorf("trimFloat(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestMoney(t *testing.T) {
	cases := map[string]string{
		"7.1281000000000000": "7.1281",
		"14.2681":            "14.2681",
		"0.0052000000":       "0.0052",
		"5.0000000000":       "5",
		"not-a-number":       "not-a-number",
	}
	for in, want := range cases {
		if got := money(in); got != want {
			t.Errorf("money(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRenderActionError(t *testing.T) {
	a := Action{Command: "delete_server", Status: "error", Error: &ActionError{Code: "locked", Message: "server is locked"}}
	got := renderAction("delete", 9, a)
	if !strings.Contains(got, "server is locked") || !strings.Contains(got, "locked") {
		t.Errorf("action render %q should surface the error", got)
	}
}
