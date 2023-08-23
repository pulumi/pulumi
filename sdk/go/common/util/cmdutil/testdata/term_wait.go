package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	sigch := make(chan os.Signal, 2)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("ready")
	select {
	case <-sigch:
		fmt.Println("exiting cleanly")
		os.Exit(0)

	case <-time.After(3 * time.Second):
		fmt.Fprintln(os.Stderr, "error: did not receive signal")
		os.Exit(1)
	}
}
