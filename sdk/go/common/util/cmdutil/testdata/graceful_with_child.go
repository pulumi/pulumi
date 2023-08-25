//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"time"
)

func main() {
	log.SetFlags(0)

	// If the process name is "child", then we are the child process.
	switch filepath.Base(os.Args[0]) {
	case "child":
		log.SetPrefix("child: ")
		childMain()
	default:
		log.SetPrefix("parent: ")
		parentMain()
	}
}

func parentMain() {
	exe, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}

	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, os.Interrupt)

	cmd := exec.Command(exe)
	cmd.Args[0] = "child" // process dispatches on args[0]
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	select {
	case <-sigch:
		log.Println("waiting for child to exit")
		if err := cmd.Wait(); err != nil {
			log.Fatal(err)
		}
		log.Println("exiting cleanly")
		os.Exit(0)

	case <-time.After(5 * time.Second):
		log.Println("did not receive signal, killing child")
		if err := cmd.Process.Kill(); err != nil {
			log.Fatal(err)
		}
		os.Exit(1)
	}
}

func childMain() {
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, os.Interrupt)
	fmt.Println("child: ready")

	select {
	case <-sigch:
		fmt.Println("child: exiting cleanly")
		os.Exit(0)

	case <-time.After(3 * time.Second):
		log.Println("error: did not receive signal")
		os.Exit(1)
	}
}
