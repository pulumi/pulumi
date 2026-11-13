// Copyright 2026, Pulumi Corporation.
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
	"context"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// CheckConfigFailure indicates that a call to CheckConfig2 failed; it contains the property and reason for the
// failure.
type CheckConfigFailure struct {
	Property resource.PropertyKey // the provider configuration property that failed checking.
	Reason   string               // the reason the property failed to check.
}

// CheckConfig2Response is the response to [ProviderWithCheckConfig2.CheckConfig2].
type CheckConfig2Response struct {
	Properties resource.PropertyMap
	Failures   []CheckConfigFailure
}

// ProviderWithCheckConfig2 is an optional interface for a [Provider] that answers CheckConfig2, the successor to
// CheckConfig that reports failures as [CheckConfigFailure]. Such a provider must keep implementing CheckConfig, for
// callers that only know about the latter.
type ProviderWithCheckConfig2 interface {
	// CheckConfig2 validates the configuration for this resource provider.
	CheckConfig2(ctx context.Context, req CheckConfigRequest) (CheckConfig2Response, error)
}

// CheckConfig2 validates the configuration for prov with its CheckConfig2 method if it implements
// [ProviderWithCheckConfig2], and with its CheckConfig method otherwise.
func CheckConfig2(ctx context.Context, prov Provider, req CheckConfigRequest) (CheckConfig2Response, error) {
	if p, ok := prov.(ProviderWithCheckConfig2); ok {
		return p.CheckConfig2(ctx, req)
	}
	resp, err := prov.CheckConfig(ctx, req)
	if err != nil {
		return CheckConfig2Response{}, err
	}
	return CheckConfig2Response{Properties: resp.Properties, Failures: checkConfigFailures(resp.Failures)}, nil
}

// checkConfigFailures converts the failures of a CheckConfig response to those of a CheckConfig2 response.
func checkConfigFailures(failures []CheckFailure) []CheckConfigFailure {
	if failures == nil {
		return nil
	}
	result := make([]CheckConfigFailure, len(failures))
	for i, f := range failures {
		result[i] = CheckConfigFailure(f)
	}
	return result
}

// checkFailures converts the failures of a CheckConfig2 response to those of a CheckConfig response, dropping
// anything that only CheckConfig2 can carry.
func checkFailures(failures []CheckConfigFailure) []CheckFailure {
	if failures == nil {
		return nil
	}
	result := make([]CheckFailure, len(failures))
	for i, f := range failures {
		result[i] = CheckFailure(f)
	}
	return result
}
