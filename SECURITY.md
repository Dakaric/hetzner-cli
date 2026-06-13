# Security Policy

## Reporting a vulnerability

Please **do not open a public issue** for security problems.

Use GitHub's private vulnerability reporting instead: go to the
[**Security** tab](https://github.com/Dakaric/hetzner-cli/security) of this
repository and click **Report a vulnerability**. That opens a private advisory
visible only to the maintainers.

Include enough to reproduce: the command, the affected version
(`hetzner version`), your OS/arch, and the impact you see. You'll get an
acknowledgement, and a fix or mitigation will be coordinated with you before any
public disclosure.

## What this tool touches

`hetzner` is a thin client; its security surface is small and worth stating
plainly:

- **Your API token never leaves your machine** except in the `Authorization`
  header of requests to the Hetzner Cloud API (`api.hetzner.cloud`, or a
  `HETZNER_BASE_URL` you set). There is no telemetry and no other network call.
- **The token is stored locally** in a `0600` dotenv file
  (`~/.config/hetzner/env`, or `%APPDATA%\hetzner\env` on Windows). `hetzner
  config` reports whether a token is present but **never prints its value**.
- **Tokens are project-scoped** — a token only ever sees the one Hetzner Cloud
  project it was created in. Prefer a read-only token where write access isn't
  needed.
- **SSH** (`hetzner ssh` / `exec`) hands off to your system's OpenSSH client;
  nothing about the SSH protocol or your keys is reimplemented or stored.
- **Destructive operations require an explicit `--yes`**, so nothing irreversible
  happens from a bare command.

## Supported versions

This is a single-binary tool with no long-term support branches: fixes land on
`main` and ship in the next tagged release. Always run the
[latest release](https://github.com/Dakaric/hetzner-cli/releases/latest).
