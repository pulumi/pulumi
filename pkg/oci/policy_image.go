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

package oci

import (
	"fmt"
	"os"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
)

// AnalyzerInfoFromImage runs a policy-pack image as a one-shot analyzer container
// and returns the identity and policy metadata the pack reports (GetAnalyzerInfo) —
// the analyzer half of "verify by running" (`pulumi package publish <policy dir>`
// checks this report against the manifest's claim before pushing). It is the policy
// twin of ProviderFromImage: the pod machinery comes from the environment (so it
// must run inside the engine container), the pack joins the engine's netns, and
// only this call's container is stopped afterward — via the fresh host's
// ReleaseContext, which tracks exactly the containers this boot started.
func AnalyzerInfoFromImage(ctx *plugin.Context, ref string) (plugin.AnalyzerInfo, error) {
	if os.Getenv("PULUMI_POD_MODE") != "true" {
		return plugin.AnalyzerInfo{}, fmt.Errorf(
			"oci: verifying a policy pack image requires pod mode (run inside the engine "+
				"container); PULUMI_POD_MODE is not set for %q", ref)
	}
	host, err := NewContainerHostFromEnv(ctx.Host)
	if err != nil {
		return plugin.AnalyzerInfo{}, err
	}
	h := host.(*containerHost)
	analyzer, err := h.PolicyAnalyzer(ctx, "publish-verify", ref, nil)
	if err != nil {
		return plugin.AnalyzerInfo{}, err
	}
	defer func() { _ = h.ReleaseContext(ctx) }()
	return analyzer.GetAnalyzerInfo(ctx.Base())
}
