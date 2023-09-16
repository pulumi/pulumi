// Copyright 2023, Pulumi Corporation.  All rights reserved.

package esc

import (
	"context"

	"github.com/pulumi/esc/schema"
)

// A Provider provides environments access to dynamic secrets. These secrets may be generated at runtime, fetched from
// other services, etc.
type Provider interface {
	// Schema returns the provider's input and output schemata.
	Schema() (inputs, outputs *schema.Schema)

	// Open retrieves the provider's secrets.
	Open(ctx context.Context, inputs map[string]Value) (Value, error)
}
