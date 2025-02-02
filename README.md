# SSH Mob

A tool to stress test SSH servers by opening a configurable number of connections and executing commands.

**NOTE: Still in active development, and was developed to stress test a specific SSH server. It may not work as expected on all servers.**

## Usage

```bash
$ ssh-mob -h
Usage of ssh-mob:
  -count int
        Number of connections to make (default 1)
  -host string
        Host to connect to (default "localhost")
  -log-level string
        Log level (debug, info, warn, error) (default "info")
  -p string
        Password to connect with
  -port int
        Port to connect to (default 22)
  -random-max int
        Maximum random delay in seconds before connecting
  -rate int
        Commands per minute (default 6)
  -script string
        Script to run on the remote host. If a file path is provided, the contents will be used.
  -ttl int
        Time to live for each connection (default 60)
  -tty
        Use TTY for the connection
  -u string
        Username to connect with (default "sshmob")
```

## Installation

```bash
go install github.com/c33tech/ssh-mob
```
