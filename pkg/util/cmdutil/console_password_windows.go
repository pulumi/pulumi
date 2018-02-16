// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.
// +build windows

package cmdutil

import (
	"fmt"

	"github.com/bgentry/speakeasy"

	"github.com/pulumi/pulumi/pkg/diag/colors"
)

// ReadConsoleNoEcho reads from the console without echoing.  This is useful for reading passwords.
func ReadConsoleNoEcho(prompt string) (string, error) {
	if prompt != "" {
		prompt = colors.ColorizeText(
			fmt.Sprintf("%s%s:%s ", colors.BrightCyan, prompt, colors.Reset))
		fmt.Print(prompt)
	}

	s, err := speakeasy.Ask("")

	fmt.Println() // echo a newline, since the user's keypress did not generate one

	return s, err
}
