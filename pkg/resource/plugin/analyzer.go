// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
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

	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/tokens"
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
}

// AnalyzeFailure indicates that resource analysis failed; it contains the property and reason for the failure.
type AnalyzeFailure struct {
	Property resource.PropertyKey // the property that failed the analysis.
	Reason   string               // the reason the property failed the analysis.
}
