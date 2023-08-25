//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"
)

func main() {
	log.SetFlags(0)

	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, os.Interrupt)
	// os.Interrupt handles SIGINT and CTRL_BREAK_EVENT.

	fmt.Println("ready")
	select {
	case <-sigch:
		log.Println("exiting cleanly")
		os.Exit(0)

	case <-time.After(3 * time.Second):
		log.Fatal("error: did not receive signal")
	}
}
