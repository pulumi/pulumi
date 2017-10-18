// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.
// +build !windows

package cmd

import (
	"fmt"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"
)

func readConsoleNoEcho() (string, error) {
	b, err := terminal.ReadPassword(syscall.Stdin)

	fmt.Println() // echo a newline, since the user's keypress did not generate one

	return string(b), err
}
