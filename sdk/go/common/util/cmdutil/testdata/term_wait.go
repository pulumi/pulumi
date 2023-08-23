package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, syscall.SIGTERM, syscall.SIGINT)
	// On Linux, we send SIGTERM.
	// On Windows, CTRL_BREAK_EVENT is treated as SIGINT.

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
