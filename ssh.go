package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// This file bridges the CLI from the control plane (the Cloud API) to the data
// plane (a shell on the server). It resolves a server reference to its public
// IPv4 via the same lookup every other command uses, then shells out to the
// system `ssh` client. No SSH is reimplemented — the OpenSSH client owns the
// connection, host-key handling, and the user's agent/keys.

// defaultKeyPath is ~/.ssh/id_ed25519, the key the CLI offers by default.
// Override per call with --key. The path is OS-correct via UserHomeDir.
func defaultKeyPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "id_ed25519"
	}
	return filepath.Join(home, ".ssh", "id_ed25519")
}

// sshArgs assembles the ssh invocation for a resolved host. batch=true adds
// BatchMode so a non-interactive `exec` fails fast instead of hanging on a
// password prompt; interactive `ssh` leaves it off so a passphrase can be typed.
func sshArgs(o opts, ip string, batch bool) []string {
	user := firstNonEmpty(o.get("user"), "root")
	args := []string{
		"-i", expandHome(firstNonEmpty(o.get("key"), defaultKeyPath())),
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=10",
	}
	if batch {
		args = append(args, "-o", "BatchMode=yes")
	}
	if port := o.get("port"); port != "" {
		args = append(args, "-p", port)
	}
	return append(args, fmt.Sprintf("%s@%s", user, ip))
}

// cmdSSH opens an interactive shell on a server. It wires the real terminal
// through to ssh, so `hetzner ssh <name>` behaves like a normal ssh login.
func cmdSSH(c *Client, args []string) {
	o := parseOpts(args)
	ip, name, err := c.serverIPv4(firstPos(o))
	if err != nil {
		fail(err)
	}
	fmt.Fprintf(os.Stderr, "connecting to %s (%s)…\n", name, ip)
	runSSH(sshArgs(o, ip, false))
}

// cmdExec runs a single command on a server and streams its output back. This
// is the non-interactive form an agent (or a script) uses. The remote command
// can be a single quoted argument ('docker ps') or, to keep its own --flags
// intact, everything after a bare "--" (exec my-web -- docker ps --all).
func cmdExec(c *Client, args []string) {
	o := parseOpts(args)
	if len(o.pos) == 0 {
		fail(execUsage())
	}
	remoteCommand := strings.Join(o.rest, " ")
	if remoteCommand == "" {
		remoteCommand = strings.Join(o.pos[1:], " ")
	}
	if remoteCommand == "" {
		fail(execUsage())
	}
	ip, _, err := c.serverIPv4(o.pos[0])
	if err != nil {
		fail(err)
	}
	runSSH(append(sshArgs(o, ip, true), remoteCommand))
}

func execUsage() error {
	return fmt.Errorf("usage: hetzner exec <id|name> '<command>'   (e.g. hetzner exec ai-test 'docker ps'; for a command with its own flags: hetzner exec ai-test -- docker ps --all)")
}

// runSSH executes the system ssh client with the given args, wiring the current
// process's stdio, and exits with ssh's own exit code so the remote command's
// status propagates. A missing ssh client yields an actionable error.
func runSSH(args []string) {
	sshBin, err := exec.LookPath("ssh")
	if err != nil {
		fail(fmt.Errorf("no ssh client found in PATH (Windows: enable the OpenSSH Client optional feature, or install Git for Windows)"))
	}
	cmd := exec.Command(sshBin, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		fail(err)
	}
}
