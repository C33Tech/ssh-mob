package main

import (
	"bufio"
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"golang.org/x/crypto/ssh"
)

type Agent struct {
	Host            string
	Port            int
	Username        string
	Password        string
	PrivateKeyPath  string
	UseTTY          bool
	Connection      *ssh.Client
	ConnectionDelay int
	ConnectionTTL   int
	ConnectionStart time.Time
	CommandRate     int
	CommandScript   []string
	MaxRetries      int
}

func (a *Agent) Connect(ctx context.Context) error {
	if a.Connection != nil {
		return nil
	}

	// Apply connection delay only on first attempt (context-aware)
	if a.ConnectionDelay > 0 {
		select {
		case <-time.After(time.Duration(a.ConnectionDelay) * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	config := ssh.ClientConfig{
		User: a.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(a.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	var lastErr error
	maxAttempts := a.MaxRetries + 1 // MaxRetries=0 means 1 attempt

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Check for cancellation before each attempt
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", a.Host, a.Port), &config)
		if err == nil {
			a.ConnectionStart = time.Now()
			a.Connection = conn
			log.Debug("Connected to SSH server.", "time", time.Now(), "host", a.Host, "port", a.Port)
			return nil
		}

		lastErr = err

		// If we have retries left, log and backoff
		if attempt < maxAttempts-1 {
			backoff := a.getBackoffDuration(attempt)
			log.Warn("Connection failed, retrying...", "attempt", attempt+1, "backoff", backoff, "error", err)

			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	log.Error("Connection failed after retries", "attempts", maxAttempts, "error", lastErr)
	return lastErr
}

func (a *Agent) getBackoffDuration(attempt int) time.Duration {
	// Exponential backoff: 1s, 2s, 4s, 8s, 16s (capped)
	backoff := time.Duration(1<<attempt) * time.Second
	if backoff > 16*time.Second {
		backoff = 16 * time.Second
	}
	return backoff
}

func (a *Agent) RunProgram(ctx context.Context) error {
	if a.UseTTY {
		return a.RunTTYProgram(ctx)
	}

	return a.RunStandardProgram(ctx)
}

func (a *Agent) RunTTYProgram(ctx context.Context) error {
	if a.Connection == nil {
		return fmt.Errorf("Connection is nil")
	}

	sess, err := a.Connection.NewSession()
	if err != nil {
		return err
	}

	defer sess.Close()

	modes := ssh.TerminalModes{
		ssh.ECHO:          0,     // disable echoing
		ssh.ECHOCTL:       0,     // disable echoing control characters
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}

	if err := sess.RequestPty("xterm-256color", 100, 30, modes); err != nil {
		a.Close()
		return err
	}

	stdin, err := sess.StdinPipe()
	if err != nil {
		a.Close()
		return err
	}

	stdout, err := sess.StdoutPipe()
	if err != nil {
		a.Close()
		return err
	}

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			scanner.Text() // Do nothing with the output
		}
	}()

	stderr, err := sess.StderrPipe()
	if err != nil {
		a.Close()
		return err
	}

	go func() {
		scanner := bufio.NewScanner(stderr)

		for scanner.Scan() {
			scanner.Text() // Do nothing with the output
		}
	}()

	if err := sess.Shell(); err != nil {
		a.Close()
		return err
	}

	log.Debug("Waiting for shell to start...")
	select {
	case <-time.After(time.Second * 10):
	case <-ctx.Done():
		return ctx.Err()
	}

	idx := 0
	for {
		// Check for cancellation
		select {
		case <-ctx.Done():
			log.Info("Context cancelled. Closing agent...")
			return nil
		default:
		}

		if a.ConnectionStart.Add(time.Duration(a.ConnectionTTL) * time.Second).Before(time.Now()) {
			log.Info("Connection TTL reached. Closing agent...")
			return nil
		}

		command := a.getCommand(idx)

		log.Debug("Running", "CMD", command)
		log.Debug("Writing command to stdin...")
		if _, err := stdin.Write([]byte(command + "\r")); err != nil {
			log.Error("Failed to write command to stdin: ", err)
			return err
		}

		log.Debug("Sleeping", "duration", a.getSleepDuration())

		// Context-aware sleep
		select {
		case <-time.After(a.getSleepDuration()):
		case <-ctx.Done():
			return nil
		}
		idx++
	}
}

func (a *Agent) RunStandardProgram(ctx context.Context) error {
	if a.Connection == nil {
		return fmt.Errorf("Connection is nil")
	}

	idx := 0
	for {
		// Check for cancellation
		select {
		case <-ctx.Done():
			log.Info("Context cancelled. Closing agent...")
			return nil
		default:
		}

		if a.ConnectionStart.Add(time.Duration(a.ConnectionTTL) * time.Second).Before(time.Now()) {
			log.Info("Connection TTL reached. Closing agent...")
			return nil
		}

		command := a.getCommand(idx)

		sess, err := a.Connection.NewSession()
		if err != nil {
			return err
		}

		modes := ssh.TerminalModes{
			ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
			ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
		}

		if err := sess.RequestPty("xterm-256color", 100, 30, modes); err != nil {
			sess.Close()
			a.Close()
			return err
		}

		log.Debug("Running", "CMD", command)
		out, err := sess.Output(command)
		if err != nil {
			log.Error("Failed to run command: ", err)
			sess.Close()
			a.Close()
			return err
		}

		log.Info(string(out))
		sess.Close()

		// Context-aware sleep
		select {
		case <-time.After(a.getSleepDuration()):
		case <-ctx.Done():
			return nil
		}
		idx++
	}
}

func (a *Agent) getCommand(idx int) string {
	if len(a.CommandScript) > 0 {
		if idx >= len(a.CommandScript) {
			return ""
		}

		return a.CommandScript[idx]
	}

	return "echo 'Hello, world!'"
}

func (a *Agent) getSleepDuration() time.Duration {
	return time.Duration(60/a.CommandRate) * time.Second
}

func (a *Agent) Close() {
	log.Debug("Closing connection...")
	if a.Connection != nil {
		a.Connection.Close()
		a.Connection = nil
	}
}
