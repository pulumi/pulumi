// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.
// +build windows

package cmd

import (
	"fmt"

	"github.com/bgentry/speakeasy"
)

func readConsoleNoEcho() (string, error) {
	s, err := speakeasy.Ask("")

	fmt.Println() // echo a newline, since the user's keypress did not generate one

	return s, err
}
