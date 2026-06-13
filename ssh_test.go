package main

import (
	"strings"
	"testing"
)

func TestSSHArgsDefaults(t *testing.T) {
	got := strings.Join(sshArgs(parseOpts(nil), "1.2.3.4", false), " ")
	if !strings.Contains(got, "root@1.2.3.4") {
		t.Errorf("args %q should target root@1.2.3.4 by default", got)
	}
	if !strings.Contains(got, "StrictHostKeyChecking=accept-new") {
		t.Errorf("args %q should trust the host key on first connect", got)
	}
	if strings.Contains(got, "BatchMode") {
		t.Errorf("interactive args %q must not set BatchMode", got)
	}
}

func TestSSHArgsExecIsBatch(t *testing.T) {
	got := strings.Join(sshArgs(parseOpts(nil), "1.2.3.4", true), " ")
	if !strings.Contains(got, "BatchMode=yes") {
		t.Errorf("exec args %q must set BatchMode so it fails fast", got)
	}
}

func TestSSHArgsUserAndPortOverride(t *testing.T) {
	o := parseOpts([]string{"--user", "deploy", "--port", "2222"})
	got := strings.Join(sshArgs(o, "10.0.0.9", false), " ")
	if !strings.Contains(got, "deploy@10.0.0.9") {
		t.Errorf("args %q should honor --user", got)
	}
	if !strings.Contains(got, "-p 2222") {
		t.Errorf("args %q should honor --port", got)
	}
}
