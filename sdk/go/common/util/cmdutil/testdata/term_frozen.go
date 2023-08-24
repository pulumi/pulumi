//go:build ignore
// +build ignore

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
		time.Sleep(3 * time.Second)
		fmt.Fprintln(os.Stderr, "error: was not forced to exit")
		os.Exit(2)

	case <-time.After(3 * time.Second):
		fmt.Fprintln(os.Stderr, "error: did not receive signal")
		os.Exit(1)
	}
}
