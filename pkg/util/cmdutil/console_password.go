// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.
// +build !windows

package cmdutil

import (
	"fmt"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"
)

// ReadConsoleNoEcho reads from the console without echoing.  This is useful for reading passwords.
func ReadConsoleNoEcho(prompt string) (string, error) {
	if prompt != "" {
		fmt.Printf("%s: ", prompt)
	}

	b, err := terminal.ReadPassword(syscall.Stdin)

	fmt.Println() // echo a newline, since the user's keypress did not generate one

	return string(b), err
}
