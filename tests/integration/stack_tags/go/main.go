// Copyright 2016-2019, Pulumi Corporation.  All rights reserved.

package main

import (
	"fmt"

	"github.com/pulumi/pulumi/sdk/go/pulumi"
	"github.com/pulumi/pulumi/sdk/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "stack_tags_go")
		customtag := cfg.GetBool("customtag")

		expected := map[string]string{
			"pulumi:project":     "stack_tags_go",
			"pulumi:runtime":     "go",
			"pulumi:description": "A simple Go program that uses stack tags",
		}
		if customtag {
			expected["foo"] = "bar"
		}

		for k, v := range expected {
			actualValue, ok := ctx.GetStackTag(k)
			if !ok {
				return fmt.Errorf("%s not found from GetStackTag", k)
			}
			if actualValue != v {
				return fmt.Errorf("%s not the expected value from GetStackTag; got %s", k, actualValue)
			}
		}

		tags := ctx.GetStackTags()
		for k, v := range expected {
			actualValue, ok := tags[k]
			if !ok {
				return fmt.Errorf("%s not found from GetStackTags", k)
			}
			if actualValue != v {
				return fmt.Errorf("%s not the expected value from GetStackTags; got %s", k, actualValue)
			}
		}

		return nil
	})
}
