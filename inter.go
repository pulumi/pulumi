package main

import (
	"fmt"
	"golang.org/x/crypto/ssh/terminal"
	"os"
)

func main() {
	fmt.Printf("STDIN: %v\n", terminal.IsTerminal(int(os.Stdin.Fd())))
	fmt.Printf("STDOUT: %v\n", terminal.IsTerminal(int(os.Stdout.Fd())))
	fmt.Printf("STDERR: %v\n", terminal.IsTerminal(int(os.Stderr.Fd())))
}
