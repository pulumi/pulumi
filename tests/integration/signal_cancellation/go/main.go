// Copyright 2026, Pulumi Corporation.

package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		sentinelDir := os.Getenv("SENTINEL_DIR")
		if sentinelDir == "" {
			return fmt.Errorf("SENTINEL_DIR not set")
		}
		os.WriteFile(filepath.Join(sentinelDir, "started"), []byte("ok"), 0o600)

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt)

		<-sig
		os.WriteFile(filepath.Join(sentinelDir, "graceful-shutdown"), []byte("ok"), 0o600)

		return nil
	})
}
