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

	fmt.Println("Waiting for SIGINT...")
	select {
	case <-sigch:
		fmt.Println("SIGINT received, cleaning up...")
		time.Sleep(1 * time.Second)

	case <-time.After(3 * time.Second):
		fmt.Println("Timed out waiting for SIGINT, exiting...")
		os.Exit(1)
	}
}
