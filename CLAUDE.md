# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

SSH Mob is a stress testing tool for SSH servers. It spawns multiple concurrent SSH connections ("agents") that execute commands at a configurable rate until a TTL expires.

**Note:** This tool is in active development and was built to test a specific SSH server - behavior may vary across different servers.

## Build & Run Commands

```bash
# Build
go build

# Install globally
go install github.com/c33tech/ssh-mob

# Run with basic options
./ssh-mob -host example.com -u username -p password -count 10 -ttl 120

# Run with script file
./ssh-mob -host example.com -u user -p pass -script commands.txt

# Run with inline commands (semicolon-separated)
./ssh-mob -host example.com -u user -p pass -script "ls;pwd;whoami"
```

## Architecture

The codebase consists of two files with clear separation:

- **main.go**: CLI parsing, user input prompts (via charmbracelet/huh), agent orchestration, script parsing
- **ssh.go**: `Agent` struct and all SSH connection/command execution logic

### Execution Flow

1. Parse CLI flags and prompt for missing required fields (host, username, password)
2. Parse script (file path or inline string, split by newlines or semicolons)
3. Create N `Agent` instances with shared config
4. Launch each agent in a goroutine (all run concurrently)
5. Each agent: optional random delay → connect → execute commands in loop → close at TTL
6. `sync.WaitGroup` coordinates shutdown

### Agent Modes

- **Standard mode** (default): Creates a new SSH session per command, executes via `sess.Output()`
- **TTY mode** (`-tty` flag): Opens a persistent interactive shell, sends commands via stdin

### Key Behaviors

- Agents are isolated - connection failures don't affect other agents
- Rate limiting: `-rate N` means N commands/minute (sleep = 60/N seconds between commands)
- TTL enforcement: Each agent tracks elapsed time and closes after `-ttl` seconds
- Host key verification is disabled (`ssh.InsecureIgnoreHostKey()`) - intended for testing environments only

## Dependencies

- `github.com/charmbracelet/huh` - Interactive CLI forms
- `github.com/charmbracelet/log` - Structured logging
- `golang.org/x/crypto/ssh` - SSH client
