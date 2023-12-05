// Copyright 2016-2021, Pulumi Corporation.
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

package providers

import (
	"fmt"
	"strings"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// A ProviderRequest is a tuple of an optional semantic version, download server url and a package name. Whenever
// the engine receives a registration for a resource that doesn't explicitly specify a provider, the engine creates
// a ProviderRequest for that resource's provider, using the version passed to the engine as part of RegisterResource
// and the package derived from the resource's token.
//
// The source evaluator (source_eval.go) is responsible for servicing provider requests. It does this by interpreting
// these provider requests and sending resource registrations to the engine for the providers themselves. These are
// called "default providers".
//
// ProviderRequest is useful as a hash key. The engine is free to instantiate any number of provider requests, but it
// is free to cache requests for a provider request that is equal to one that has already been serviced. If you do use
// ProviderRequest as a hash key, you should call String() to get a usable key for string-based hash maps.
type ProviderRequest struct {
	version           *semver.Version
	pkg               tokens.Package
	pluginDownloadURL string
	pluginChecksums   map[string][]byte
}

// NewProviderRequest constructs a new provider request from an optional version, optional
// pluginDownloadURL and package.
func NewProviderRequest(
	version *semver.Version, pkg tokens.Package,
	pluginDownloadURL string, checksums map[string][]byte,
) ProviderRequest {
	return ProviderRequest{
		version:           version,
		pkg:               pkg,
		pluginDownloadURL: strings.TrimSuffix(pluginDownloadURL, "/"),
		pluginChecksums:   checksums,
	}
}

// Version returns this provider request's version. May be nil if no version was provided.
func (p ProviderRequest) Version() *semver.Version {
	return p.version
}

// Package returns this provider request's package.
func (p ProviderRequest) Package() tokens.Package {
	return p.pkg
}

// PluginDownloadURL returns this providers server url. May be "" if no pluginDownloadURL was
// provided.
func (p ProviderRequest) PluginDownloadURL() string {
	return p.pluginDownloadURL
}

// PluginChecksums returns this providers checksums. May be nil if no checksums were provided.
func (p ProviderRequest) PluginChecksums() map[string][]byte {
	return p.pluginChecksums
}

// Name returns a QName that is an appropriate name for a default provider constructed from this provider request. The
// name is intended to be unique; as such, the name is derived from the version associated with this request.
//
// If a version is not provided, "default" is returned. Otherwise, Name returns a name starting with "default" and
// followed by a QName-legal representation of the semantic version of the requested provider.
func (p ProviderRequest) Name() string {
	base := "default"
	if v := p.version; v != nil {
		// QNames are forbidden to contain dashes, so we construct a string here using the semantic
		// version's component parts.
		base += fmt.Sprintf("_%d_%d_%d", v.Major, v.Minor, v.Patch)
		for _, pre := range v.Pre {
			base += "_" + pre.String()
		}
		for _, build := range v.Build {
			base += "_" + build
		}
	}

	if url := p.pluginDownloadURL; url != "" {
		base += "_" + tokens.IntoQName(url).String()
	}

	// This thing that we generated must be a QName, the engine doesn't actually care but it probably helps
	// down the line if we keep these names simple.
	contract.Assertf(tokens.IsQName(base), "generated provider name %q is not a QName", base)
	return base
}

// String returns a string representation of this request. This string is suitable for use as a hash key.
func (p ProviderRequest) String() string {
	var version string
	if p.version != nil {
		version = "-" + p.version.String()
	}
	var url string
	if p.pluginDownloadURL != "" {
		url = "-" + p.pluginDownloadURL
	}
	return p.pkg.String() + version + url
}
