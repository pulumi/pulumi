// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package plugin

import (
	"io"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/workspace"
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
	// Is called before the resource is modified.
	Analyze(r AnalyzerResource) ([]AnalyzeDiagnostic, error)
	// AnalyzeStack analyzes all resources after a successful preview or update.
	// Is called after all resources have been processed, and all changes applied.
	AnalyzeStack(resources []AnalyzerResource) ([]AnalyzeDiagnostic, error)
	// GetAnalyzerInfo returns metadata about the analyzer (e.g., list of policies contained).
	GetAnalyzerInfo() (AnalyzerInfo, error)
	// GetPluginInfo returns this plugin's information.
	GetPluginInfo() (workspace.PluginInfo, error)
}

// AnalyzerResource mirrors a resource that is sent to the analyzer.
type AnalyzerResource struct {
	URN        resource.URN
	Type       tokens.Type
	Name       tokens.QName
	Properties resource.PropertyMap
	Options    AnalyzerResourceOptions
}

// AnalyzerResourceOptions mirrors resource options sent to the analyzer.
type AnalyzerResourceOptions struct {
	Parent                  resource.URN            // an optional parent URN for this resource.
	Protect                 bool                    // true to protect this resource from deletion.
	Dependencies            []resource.URN          // dependencies of this resource object.
	Provider                string                  // the provider to use for this resource.
	AdditionalSecretOutputs []resource.PropertyKey  // outputs that should always be treated as secrets.
	Aliases                 []resource.URN          // additional URNs that should be aliased to this resource.
	CustomTimeouts          resource.CustomTimeouts // an optional config object for resource options
}

// AnalyzeDiagnostic indicates that resource analysis failed; it contains the property and reason
// for the failure.
type AnalyzeDiagnostic struct {
	PolicyName        string
	PolicyPackName    string
	PolicyPackVersion string
	Description       string
	Message           string
	Tags              []string
	EnforcementLevel  apitype.EnforcementLevel
	URN               resource.URN
}

// AnalyzerInfo provides metadata about a PolicyPack inside an analyzer.
type AnalyzerInfo struct {
	Name        string
	DisplayName string
	Policies    []apitype.Policy
}
