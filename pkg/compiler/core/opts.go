// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package core

import (
	"github.com/pulumi/pulumi-fabric/pkg/diag"
)

// Options contains all of the settings a user can use to control the compiler's behavior.
type Options struct {
	Diag diag.Sink // a sink to use for all diagnostics.
}

// DefaultOptions returns the default set of compiler options.
func DefaultOptions() *Options {
	return &Options{}
}

// DefaultSink returns the default preconfigured diagnostics sink.
func DefaultSink(path string) diag.Sink {
	return diag.DefaultSink(diag.FormatOptions{
		Pwd:    path, // ensure output paths are relative to the current path.
		Colors: true, // turn on colorization of warnings/errors.
	})
}
