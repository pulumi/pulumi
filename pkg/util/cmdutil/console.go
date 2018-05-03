// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cmdutil

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/pulumi/pulumi/pkg/diag/colors"
)

// Emoji controls whether emojis will by default be printed in the output.
// While some Linux systems can display Emoji's in the terminal by default, we restrict this to just macOS, like Yarn.
var Emoji = (runtime.GOOS == "darwin")

// EmojiOr returns the emoji string e if emojis are enabled, or the string or if emojis are disabled.
func EmojiOr(e, or string) string {
	if Emoji {
		return e
	}
	return or
}

// Interactive returns true if we're in an interactive terminal session.
func Interactive() bool {
	return terminal.IsTerminal(int(os.Stdin.Fd()))
}

// ReadConsole reads the console with the given prompt text.
func ReadConsole(prompt string) (string, error) {
	if prompt != "" {
		prompt = colors.ColorizeText(
			fmt.Sprintf("%s%s:%s ", colors.BrightCyan, prompt, colors.Reset))
		fmt.Print(prompt)
	}

	reader := bufio.NewReader(os.Stdin)
	raw, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return RemoveTralingNewline(raw), nil
}

// IsTruthy returns true if the given string represents a CLI input interpreted as "true".
func IsTruthy(s string) bool {
	return s == "1" || strings.EqualFold(s, "true")
}

// RemoveTralingNewline removes a trailing newline from a string. On windows, we'll remove either \r\n or \n, on other
// platforms, we just remove \n.
func RemoveTralingNewline(s string) string {
	s = strings.TrimSuffix(s, "\n")

	if runtime.GOOS == "windows" {
		s = strings.TrimSuffix(s, "\r")
	}

	return s
}
