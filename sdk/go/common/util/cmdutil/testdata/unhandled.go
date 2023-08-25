//go:build ignore
// +build ignore

package main

import (
	"flag"
	"fmt"
	"log"
	"time"
)

func main() {
	flag.Parse()
	log.SetFlags(0)

	fmt.Println("ready")
	<-time.After(3 * time.Second)
	log.Fatal("error: was not terminated")
}
