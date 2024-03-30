// Copyright 2023, Pulumi Corporation.

package pager

import (
	"context"
	"io"
	"os"
	"os/exec"
)

func Run(pager string, stdout, stderr io.Writer, f func(context.Context, io.Writer) error) error {
	if pager == "" {
		pager = os.Getenv("PAGER")
		if pager == "" {
			pager = "less"
		}
	}

	pager, err := exec.LookPath(pager)
	if err == nil {
		return runSystemPager(pager, stdout, stderr, f)
	}
	return runTeaPager(stdout, f)
}
