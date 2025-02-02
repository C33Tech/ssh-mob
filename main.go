package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
)

func main() {
	host := flag.String("host", "localhost", "Host to connect to")
	port := flag.Int("port", 22, "Port to connect to")
	username := flag.String("u", "sshmob", "Username to connect with")
	password := flag.String("p", "", "Password to connect with")
	count := flag.Int("count", 1, "Number of connections to make")
	ttl := flag.Int("ttl", 60, "Time to live for each connection")
	randomMax := flag.Int("random-max", 0, "Maximum random delay in seconds before connecting")
	rate := flag.Int("rate", 6, "Commands per minute")
	useTTY := flag.Bool("tty", false, "Use TTY for the connection")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	script := flag.String("script", "", "Script to run on the remote host. If a file path is provided, the contents will be used.")
	flag.Parse()

	// If any required fields are missing, prompt the user for them
	fields := []huh.Field{}
	if host == nil || *host == "" {
		fields = append(fields, huh.NewInput().
			Title("Please enter the host to connect to.").
			Validate(func(str string) error {
				if str == "" {
					return fmt.Errorf("Host cannot be empty")
				}

				return nil
			}).
			Value(host),
		)
	}

	if username == nil || *username == "" {
		fields = append(fields, huh.NewInput().
			Title("Please enter the username to connect with.").
			Validate(func(str string) error {
				if str == "" {
					return fmt.Errorf("Username cannot be empty")
				}

				return nil
			}).
			Value(username),
		)
	}

	if password == nil || *password == "" {
		fields = append(fields, huh.NewInput().
			Title("Please enter the password for the SSH connection.").
			EchoMode(huh.EchoModePassword).
			Validate(func(str string) error {
				if str == "" {
					return fmt.Errorf("Password cannot be empty")
				}

				return nil
			}).
			Value(password),
		)
	}

	if len(fields) > 0 {
		form := huh.NewForm(huh.NewGroup(fields...))
		err := form.Run()
		if err != nil {
			log.Error("Failed to get required user input: ", err)
			return
		}
	}

	log.SetLevel(translateLogLevel(*logLevel))
	log.Info("Starting SSH Mob...", "host", *host, "port", *port, "username", *username, "count", *count, "ttl", *ttl)

	// Create the requested number of agents and connect them asynchronously
	var wg sync.WaitGroup
	agents := make([]*Agent, *count)

	log.Debug("Creating agents...")

	commandScript := []string{}
	if script != nil && *script != "" {
		var err error
		commandScript, err = parseScript(*script)
		if err != nil {
			log.Error("Failed to parse script: ", err)
			return
		}
	}

	for i := 0; i < *count; i++ {
		delay := 0
		if randomMax != nil && *randomMax > 0 {
			delay = rand.Intn(*randomMax)
		}

		agents[i] = &Agent{
			Host:            *host,
			Port:            *port,
			Username:        *username,
			Password:        *password,
			ConnectionDelay: delay,
			ConnectionTTL:   *ttl,
			UseTTY:          *useTTY,
			CommandRate:     *rate,
			CommandScript:   commandScript,
		}

		wg.Add(1)

		log.Debug(fmt.Sprintf("Starting agent #%d", i+1))
		go func(a *Agent) {
			defer wg.Done()
			agents[i].Connect()

			log.Debug(fmt.Sprintf("Agent #%d connected. Starting command loop...", i+1))
			agents[i].RunProgram()
			a.Close()
		}(agents[i])
	}

	// Wait for all agents to finish
	wg.Wait()

	log.Info("All connections closed.")
}

func translateLogLevel(logLevel string) log.Level {
	switch logLevel {
	case "debug":
		return log.DebugLevel
	case "info":
		return log.InfoLevel
	case "warn":
		return log.WarnLevel
	case "error":
		return log.ErrorLevel
	case "fatal":
		return log.FatalLevel
	default:
		return log.InfoLevel
	}
}

func parseScript(script string) ([]string, error) {
	var data []byte

	// Check if the string contains a valid path
	_, err := os.Stat(script)
	if err == nil {
		// Read the file contents
		data, err = os.ReadFile(script)
		if err != nil {
			return nil, fmt.Errorf("Failed to read script file: %s", err)
		}
	} else {
		data = []byte(script)
	}

	// Split the data by newlines or semicolons
	dataStr := string(data)
	lines := strings.Split(dataStr, "\n")
	if len(lines) == 1 {
		lines = strings.Split(dataStr, ";")
	}

	return lines, nil
}
