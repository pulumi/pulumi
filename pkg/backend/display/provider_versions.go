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

package display

import (
	"maps"
	"slices"
	"sync"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	deployproviders "github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

var (
	providerVersionsLock sync.Mutex
	providerVersions     = map[string][]semver.Version{}
)

// ProviderVersions returns the versions of each provider that appeared in the engine events this
// process has displayed, keyed by package name.
func ProviderVersions() map[string][]semver.Version {
	providerVersionsLock.Lock()
	defer providerVersionsLock.Unlock()
	return maps.Clone(providerVersions)
}

func recordProviderVersion(e engine.Event) {
	p, ok := e.Payload().(engine.ResourcePreEventPayload)
	if !ok || p.Metadata.Res == nil || !providers.IsProviderType(p.Metadata.Type) {
		return
	}
	inputs := resource.ToResourcePropertyMap(p.Metadata.Res.Inputs)
	name, err := deployproviders.GetProviderName(providers.GetProviderPackage(p.Metadata.Type), inputs)
	if err != nil {
		return
	}
	version, err := deployproviders.GetProviderVersion(inputs)
	if err != nil || version == nil {
		return
	}

	providerVersionsLock.Lock()
	defer providerVersionsLock.Unlock()
	if !slices.ContainsFunc(providerVersions[string(name)], version.Equals) {
		providerVersions[string(name)] = append(providerVersions[string(name)], *version)
	}
}
