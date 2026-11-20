// Copyright 2026, Pulumi Corporation. All rights reserved.
//go:build !all

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() (err error) {
	defer func() {
		err = errors.Join(err, cleanup(os.Getenv("CLEANUP_DIR")))
	}()

	return pulumi.RunErr(func(ctx *pulumi.Context) error {
		var resource pulumi.CustomResourceState
		return ctx.RegisterResource("testprovider:index:FailsOnCreate", "failure", nil, &resource)
	})
}

func cleanup(dir string) error {
	if err := os.WriteFile(filepath.Join(dir, "started"), []byte("started"), 0o600); err != nil {
		return err
	}
	time.Sleep(time.Second)
	return os.WriteFile(filepath.Join(dir, "finished"), []byte("done"), 0o600)
}
