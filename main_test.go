package main

import (
	"reflect"
	"testing"
)

// TestParseOpts locks the behavior of the forgiving, order-independent flag
// parser every command depends on: the value-vs-bool disambiguation, the two
// flag spellings, repeatable --ssh-key, and the bare "--" passthrough that lets
// an exec command keep its own --flags.
func TestParseOpts(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		flags   map[string]string
		bools   map[string]bool
		sshKeys []string
		pos     []string
		rest    []string
	}{
		{
			name:  "valued flag, space form",
			args:  []string{"--type", "cx22"},
			flags: map[string]string{"type": "cx22"},
		},
		{
			name:  "valued flag, equals form",
			args:  []string{"--type=cx22"},
			flags: map[string]string{"type": "cx22"},
		},
		{
			name:  "json and yes aliases",
			args:  []string{"-j", "-y"},
			bools: map[string]bool{"json": true, "yes": true},
		},
		{
			name:  "force is an alias for yes",
			args:  []string{"--force"},
			bools: map[string]bool{"yes": true},
		},
		{
			name:  "trailing flag with no value becomes a bool",
			args:  []string{"--automount"},
			bools: map[string]bool{"automount": true},
		},
		{
			name:  "a flag is not consumed as the previous flag's value",
			args:  []string{"--type", "--json"},
			bools: map[string]bool{"type": true, "json": true},
		},
		{
			name:    "repeatable ssh-key in both spellings",
			args:    []string{"--ssh-key", "alpha", "--ssh-key=beta"},
			sshKeys: []string{"alpha", "beta"},
		},
		{
			name: "positionals are collected in order",
			args: []string{"my-web", "docker", "ps"},
			pos:  []string{"my-web", "docker", "ps"},
		},
		{
			name:  "double dash passes the rest through verbatim",
			args:  []string{"my-web", "--", "docker", "ps", "--all"},
			pos:   []string{"my-web"},
			rest:  []string{"docker", "ps", "--all"},
			flags: map[string]string{},
		},
		{
			name:  "flags before the double dash are still parsed",
			args:  []string{"--user", "deploy", "my-web", "--", "ls", "-la"},
			flags: map[string]string{"user": "deploy"},
			pos:   []string{"my-web"},
			rest:  []string{"ls", "-la"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := parseOpts(tt.args)

			wantFlags := tt.flags
			if wantFlags == nil {
				wantFlags = map[string]string{}
			}
			wantBools := tt.bools
			if wantBools == nil {
				wantBools = map[string]bool{}
			}
			if !reflect.DeepEqual(o.flags, wantFlags) {
				t.Errorf("flags = %v, want %v", o.flags, wantFlags)
			}
			if !reflect.DeepEqual(o.bools, wantBools) {
				t.Errorf("bools = %v, want %v", o.bools, wantBools)
			}
			if !reflect.DeepEqual(o.sshKeys, tt.sshKeys) {
				t.Errorf("sshKeys = %v, want %v", o.sshKeys, tt.sshKeys)
			}
			if !reflect.DeepEqual(o.pos, tt.pos) {
				t.Errorf("pos = %v, want %v", o.pos, tt.pos)
			}
			if !reflect.DeepEqual(o.rest, tt.rest) {
				t.Errorf("rest = %v, want %v", o.rest, tt.rest)
			}
		})
	}
}

// TestRequireYes pins the central safety contract: a destructive operation is
// refused unless --yes (or its -y/--force aliases) is present.
func TestRequireYes(t *testing.T) {
	if err := requireYes(parseOpts(nil), "delete server 1"); err == nil {
		t.Error("requireYes without --yes must return an error so the caller aborts")
	}
	for _, flag := range []string{"--yes", "-y", "--force"} {
		if err := requireYes(parseOpts([]string{flag}), "delete server 1"); err != nil {
			t.Errorf("requireYes with %s must succeed, got %v", flag, err)
		}
	}
}
