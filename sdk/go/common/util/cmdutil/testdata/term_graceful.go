//go:build ignore
// +build ignore

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"
)

var _exitCode = flag.Int("exit-code", 0, "exit code to use when the signal is received")

func main() {
	flag.Parse()
	log.SetFlags(0)

	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, os.Interrupt)
	// os.Interrupt handles SIGINT and CTRL_BREAK_EVENT.

	fmt.Println("ready")
	select {
	case <-sigch:
		log.Println("exiting cleanly")
		os.Exit(*_exitCode)

	case <-time.After(3 * time.Second):
		log.Fatal("error: did not receive signal")
	}
}
