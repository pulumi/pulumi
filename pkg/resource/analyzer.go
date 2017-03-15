// Copyright 2017 Pulumi, Inc. All rights reserved.

package resource

import (
	"io"

	"github.com/pulumi/coconut/pkg/pack"
	"github.com/pulumi/coconut/pkg/tokens"
)

// Analyzer provides a pluggable interface for performing arbitrary analysis of entire projects/stacks/snapshots, and/or
// individual resources, for arbitrary issues.  These might be style, policy, correctness, security, or performance
// related.  This interface hides the messiness of the underlying machinery, since providers are behind an RPC boundary.
type Analyzer interface {
	// Closer closes any underlying OS resources associated with this provider (like processes, RPC channels, etc).
	io.Closer
	// Analyze analyzes an entire project/stack/snapshot, and returns any errors that it finds.
	Analyze(url pack.PackageURL) ([]AnalyzeFailure, error)
	// AnalyzeResource analyzes a single resource object, and returns any errors that it finds.
	AnalyzeResource(t tokens.Type, props PropertyMap) ([]AnalyzeResourceFailure, error)
}

// AnalyzeFailure indicates that overall analysis failed; it contains the property and reason for the failure.
type AnalyzeFailure struct {
	Reason string // the reason the analysis failed.
}

// AnalyzeResourceFailure indicates that resource analysis failed; it contains the property and reason for the failure.
type AnalyzeResourceFailure struct {
	Property PropertyKey // the property that failed the analysis.
	Reason   string      // the reason the property failed the analysis.
}
