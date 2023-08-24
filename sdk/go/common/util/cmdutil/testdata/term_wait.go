package main

import (
	"fmt"
	"os"
	"os/signal"
	"time"
)

func main() {
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, os.Interrupt)
	// os.Interrupt handles SIGINT and CTRL_BREAK_EVENT.

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
