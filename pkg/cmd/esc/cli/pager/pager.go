// Copyright 2023, Pulumi Corporation.

package pager

import (
	"context"
	"io"
	"os"
	"os/exec"

	"github.com/google/shlex"
)

func Run(pager string, stdout, stderr io.Writer, f func(context.Context, io.Writer) error) error {
	if pager == "" {
		pager = os.Getenv("PAGER")
		if pager == "" {
			pager = "less -R -F"
		}
	}

	// In the case of an error, just use the builtin pager.
	pagerCommand, err := shlex.Split(pager)
	if err == nil {
		pager, err := exec.LookPath(pagerCommand[0])
		if err == nil {
			pagerCommand[0] = pager
			return runSystemPager(pagerCommand, stdout, stderr, f)
		}
	}

	return runNoopPager(stdout, f)
}
