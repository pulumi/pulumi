// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmdutil

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ReadConsole reads the console with the given prompt text.
func ReadConsole(prompt string) (string, error) {
	if prompt != "" {
		fmt.Printf("%s: ", prompt)
	}

	reader := bufio.NewReader(os.Stdin)
	raw, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(raw), nil
}

// IsTruthy returns true if the given string represents a CLI input interpreted as "true".
func IsTruthy(s string) bool {
	return s == "1" || strings.EqualFold(s, "true")
}
