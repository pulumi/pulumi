package main

import (
	"fmt"
	"net"
)

func main() {
	addrs, err := net.LookupHost("api.pulumi-staging.io")
	fmt.Printf("%v %v\n", addrs, err)
}
