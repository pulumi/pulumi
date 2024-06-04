// Copyright 2023, Pulumi Corporation.

package pager

import (
	"context"
	"fmt"
	"io"
)

func runNoopPager(stdout io.Writer, f func(context.Context, io.Writer) error) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, stdin := io.Pipe()
	defer r.Close()

	done := make(chan error)
	go func() {
		done <- func() error {
			defer stdin.Close()
			return f(ctx, stdin)
		}()
	}()

	if _, err := io.Copy(stdout, r); err != nil {
		return fmt.Errorf("running pager: %w", err)
	}
	cancel()

	return <-done
}
