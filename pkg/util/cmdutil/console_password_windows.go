// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.
// +build windows

package cmdutil

import (
	"fmt"

	"github.com/bgentry/speakeasy"
)

// ReadConsoleNoEcho reads from the console without echoing.  This is useful for reading passwords.
func ReadConsoleNoEcho(prompt string) (string, error) {
	if prompt != "" {
		fmt.Printf("%s: ", prompt)
	}

	s, err := speakeasy.Ask("")

	fmt.Println() // echo a newline, since the user's keypress did not generate one

	return s, err
}
