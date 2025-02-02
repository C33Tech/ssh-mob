package main

import (
	"bufio"
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
}

func (a *Agent) Connect() {
	if a.Connection != nil {
		return
	}

	if a.ConnectionDelay > 0 {
		time.Sleep(time.Duration(a.ConnectionDelay) * time.Second)
	}

	// Connect to the SSH server
	config := ssh.ClientConfig{
		User: a.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(a.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", a.Host, a.Port), &config)
	if err != nil {
		log.Error("Failed to dial: ", err)
		return
	}

	a.ConnectionStart = time.Now()
	a.Connection = conn

	log.Debug("Connected to SSH server.", "time", time.Now(), "host", a.Host, "port", a.Port)

	if a.UseTTY {

	}
}

func (a *Agent) RunProgram() error {
	if a.UseTTY {
		return a.RunTTYProgram()
	}

	return a.RunStandardProgram()
}

func (a *Agent) RunTTYProgram() error {
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
	time.Sleep(time.Second * 10) // Wait for the shell to start

	idx := 0
	for {
		if a.ConnectionStart.Add(time.Duration(a.ConnectionTTL) * time.Second).Before(time.Now()) {
			log.Info("Connection TTL reached. Closing agent...")
			return nil
		}

		command := a.getCommand(idx)

		log.Debug("Running", "CMD", command)

		if _, err := stdin.Write([]byte(command + "\r")); err != nil {
			return err
		}

		time.Sleep(a.getSleepDuration())
		idx++
	}
}

func (a *Agent) RunStandardProgram() error {
	if a.Connection == nil {
		return fmt.Errorf("Connection is nil")
	}

	idx := 0
	for {
		if a.ConnectionStart.Add(time.Duration(a.ConnectionTTL) * time.Second).Before(time.Now()) {
			log.Info("Connection TTL reached. Closing agent...")
			return nil
		}

		command := a.getCommand(idx)

		sess, err := a.Connection.NewSession()
		if err != nil {
			return err
		}

		defer sess.Close()

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
			sess.Close()
			a.Close()
			return err
		}

		log.Info(string(out))
		sess.Close()

		time.Sleep(a.getSleepDuration())
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
