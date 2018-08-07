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
	Analyze(urn resource.URN, id resource.ID, props resource.PropertyMap) ([]AnalyzerDiagnostic, error)
	// GetPluginInfo returns this plugin's information.
	GetPluginInfo() (workspace.PluginInfo, error)
}

// AnalyzerDiagnostic reports a potential issue discovered by an analyzer along with metadata about that issue.
type AnalyzerDiagnostic struct {
	ID         string  // an optional identifier unique to this diagnostic.
	Message    string  // a freeform message describing the issue discovered by the analyzer.
	Severity   string  // the severity of this diagnostic, including how seriously it is to be taken.
	Category   string  // a category classifying this diagnostic for purposes of aggregation.
	Confidence float32 // a score from 0.0 to 1.0 indicating how confident the analyzer is about this issue.
}
