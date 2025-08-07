//go:build !all
// +build !all

package main

import (
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

func main() {
	args := os.Args[1:]                       // Drop the wrapper from the command line ...
	cmd := exec.Command(args[0], args[1:]...) // ... and run the remaining args as a command

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Foreground: false, // False is the default, highlighted here because it's the trigger for the hang.
	}

	log.Printf("Starting %q in %q", cmd.String(), cmd.Dir)
	err := cmd.Start()
	if err != nil {
		log.Fatalf("start: %s", err)
	}

	// Forward signals to the child process
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		for sig := range sigChan {
			log.Printf("Got signal %s\n", sig)
			if cmd.Process != nil {
				cmd.Process.Signal(sig)
			}
		}
	}()

	// Wait for the child process to complete
	if err = cmd.Wait(); err != nil {
		// Handle exit status
		if exitError, ok := err.(*exec.ExitError); ok {
			os.Exit(exitError.ExitCode())
			return
		}
		log.Fatalf("wait: %s", err)
	}

	log.Printf("Command completed successfully")
}
