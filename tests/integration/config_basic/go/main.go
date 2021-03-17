// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package main

import (
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Just test that basic config works.
		cfg := config.New(ctx, "config_basic_go")

		tests := []struct {
			Key      string
			Expected string
		}{
			{
				Key:      "aConfigValue",
				Expected: `this value is a value`,
			},
			{
				Key:      "bEncryptedSecret",
				Expected: `this super secret is encrypted`,
			},
			{
				Key:      "outer",
				Expected: `{"inner":"value"}`,
			},
			{
				Key:      "names",
				Expected: `["a","b","c","super secret name"]`,
			},
			{
				Key:      "servers",
				Expected: `[{"host":"example","port":80}]`,
			},
			{
				Key:      "a",
				Expected: `{"b":[{"c":true},{"c":false}]}`,
			},
			{
				Key:      "tokens",
				Expected: `["shh"]`,
			},
			{
				Key:      "foo",
				Expected: `{"bar":"don't tell"}`,
			},
		}

		for _, test := range tests {
			value := cfg.Require(test.Key)
			if value != test.Expected {
				return fmt.Errorf("%q not the expected value; got %q", test.Key, value)
			}
			// config-less form
			value = config.Require(ctx, test.Key)
			if value != test.Expected {
				return fmt.Errorf("%q not the expected value; got %q", test.Key, value)
			}
		}

		return nil
	})
}
