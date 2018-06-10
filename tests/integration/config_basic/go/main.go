// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package main

import (
	"fmt"
	"github.com/pulumi/pulumi/sdk/go/pulumi"
	"github.com/pulumi/pulumi/sdk/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Just test that basic config works.
		cfg := config.New(ctx, "config_basic_go")

		// This value is plaintext and doesn't require encryption.
		value := cfg.Require("aConfigValue")
		if value != "this value is a value" {
			return fmt.Errorf("aConfigValue not the expected value; got %s", value)
		}

		// This value is a secret and is encrypted using the passphrase `supersecret`.
		secret := cfg.Require("bEncryptedSecret")
		if secret != "this super secret is encrypted" {
			return fmt.Errorf("bEncryptedSecret not the expected value; got %s", secret)
		}

		return nil
	})
}
