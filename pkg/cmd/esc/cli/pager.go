// Copyright 2023, Pulumi Corporation.

package cli

import (
	"context"
	"io"

	esc_pager "github.com/pulumi/esc/cmd/esc/cli/pager"
)

type pager interface {
	Run(pager string, stdout, stderr io.Writer, f func(context.Context, io.Writer) error) error
}

type defaultPager int

func newPager() pager {
	return defaultPager(0)
}

func (defaultPager) Run(pager string, stdout, stderr io.Writer, f func(context.Context, io.Writer) error) error {
	return esc_pager.Run(pager, stdout, stderr, f)
}
