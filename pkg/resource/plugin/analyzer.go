// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package plugin

import (
	"io"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/tokens"
)

// Analyzer provides a pluggable interface for performing arbitrary analysis of entire projects/stacks/snapshots, and/or
// individual resources, for arbitrary issues.  These might be style, policy, correctness, security, or performance
// related.  This interface hides the messiness of the underlying machinery, since providers are behind an RPC boundary.
type Analyzer interface {
	// Closer closes any underlying OS resources associated with this provider (like processes, RPC channels, etc).
	io.Closer
	// Name fetches an analyzer's qualified name.
	Name() tokens.QName
	// Analyze analyzes a single resource object, and returns any errors that it finds.
	Analyze(t tokens.Type, props resource.PropertyMap) ([]AnalyzeFailure, error)
	// GetPluginInfo returns this plugin's information.
	GetPluginInfo() (Info, error)
}

// AnalyzeFailure indicates that resource analysis failed; it contains the property and reason for the failure.
type AnalyzeFailure struct {
	Property resource.PropertyKey // the property that failed the analysis.
	Reason   string               // the reason the property failed the analysis.
}
