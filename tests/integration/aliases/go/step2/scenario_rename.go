// Copyright 2026, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// renameComponent is a component resource
type renameComponent struct {
	pulumi.ResourceState
}

// Scenario #1 - rename a resource
func scenarioRename(ctx *pulumi.Context) error {
	fooComponent := &renameComponent{}
	alias := &pulumi.Alias{
		Name: pulumi.String("foo"),
	}
	opts := pulumi.Aliases([]pulumi.Alias{*alias})
	return ctx.RegisterComponentResource("foo:component", "newfoo", fooComponent, opts)
}
