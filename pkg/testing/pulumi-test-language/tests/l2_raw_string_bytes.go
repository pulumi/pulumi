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

package tests

import (
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// A string property containing bytes that are not valid UTF-8 must flow losslessly from one
	// provider's output through the program into another provider's input. The sink provider verifies
	// byte-exactness at Create time, so a corrupted value fails the update itself, not just these
	// assertions.
	const rawBytes = "\x00hello \x80\xfe\xff world\xf0\x28"
	const rawBytesBase64 = "AGhlbGxvIID+/yB3b3JsZPAo"

	LanguageTests["l2-raw-string-bytes"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.ByteSourceProvider{} },
			func() plugin.Provider { return &providers.ByteSinkProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					require.Len(l, res.Snap.Resources, 5, "expected 5 resources in snapshot")
					RequireSingleResource(l, res.Snap.Resources, "pulumi:providers:bytesource")
					RequireSingleResource(l, res.Snap.Resources, "pulumi:providers:bytesink")
					source := RequireSingleResource(l, res.Snap.Resources, "bytesource:index:Resource")
					sink := RequireSingleResource(l, res.Snap.Resources, "bytesink:index:Resource")

					assert.Equal(l, resource.PropertyMap{
						"base64": resource.NewProperty(rawBytesBase64),
					}, source.Inputs)
					assert.Equal(l, resource.PropertyMap{
						"base64": resource.NewProperty(rawBytesBase64),
						"bytes":  resource.NewProperty(rawBytes),
					}, source.Outputs)

					wantSink := resource.PropertyMap{
						"bytes":        resource.NewProperty(rawBytes),
						"expectBase64": resource.NewProperty(rawBytesBase64),
					}
					assert.Equal(l, wantSink, sink.Inputs)
					assert.Equal(l, wantSink, sink.Outputs)
				},
			},
		},
	}
}
