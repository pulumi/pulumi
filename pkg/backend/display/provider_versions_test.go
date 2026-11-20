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
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

func TestRecordProviderVersion(t *testing.T) {
	t.Parallel()

	pre := func(typ tokens.Type, version string) engine.Event {
		return engine.NewEvent(engine.ResourcePreEventPayload{Metadata: engine.StepEventMetadata{
			Type: typ,
			Res: &engine.StepEventStateMetadata{
				Inputs: property.NewMap(map[string]property.Value{"version": property.New(version)}),
			},
		}})
	}

	recordProviderVersion(pre("pulumi:providers:recordtest", "1.2.3"))
	recordProviderVersion(pre("pulumi:providers:recordtest", "1.2.3"))
	recordProviderVersion(pre("pulumi:providers:recordtest", "2.0.0"))
	recordProviderVersion(pre("recordtest:index:Thing", "9.9.9"))

	assert.Equal(t,
		[]semver.Version{semver.MustParse("1.2.3"), semver.MustParse("2.0.0")},
		ProviderVersions()["recordtest"])
}
