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

package pulumi

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func TestStackTraceModuleSourceFile(t *testing.T) {
	t.Parallel()
	const modulePath = "example.com/infra"
	root := t.TempDir()
	programDir := filepath.Join(root, "cmd", "deploy")
	require.NoError(t, os.MkdirAll(programDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module "+modulePath+"\n"), 0o600))

	module := findStackTraceModule(programDir, modulePath)
	assert.Equal(t, filepath.Join(programDir, "main.go"), module.sourceFile(modulePath+"/cmd/deploy/main.go"))
	for _, file := range []string{
		filepath.Join(programDir, "main.go"),
		"example.com/infra-extra/main.go",
		"example.com/infra@v1.2.3/main.go",
		"example.com/infra/../main.go",
	} {
		assert.Equal(t, file, module.sourceFile(file))
	}

	otherModule := findStackTraceModule(programDir, "example.com/other")
	assert.Equal(t, "example.com/other/main.go", otherModule.sourceFile("example.com/other/main.go"))
}

func TestStackFrameIdentity(t *testing.T) {
	t.Parallel()
	modules := []string{
		pulumiSDKModule, "github.com/pulumi/pulumi-aws/sdk/v7",
		"example.com/app/plugin", "example.com/app", "gopkg.in/yaml.v3",
	}
	for _, test := range []struct {
		name, function, file, module, origin string
	}{
		{"SDK", pulumiSDKModule + "/go/pulumi.(*Context).RegisterResource", "", pulumiSDKModule, "pulumi"},
		{
			"provider SDK", "github.com/pulumi/pulumi-aws/sdk/v7/go/aws/s3.NewBucket", "",
			"github.com/pulumi/pulumi-aws/sdk/v7", "",
		},
		{"similar module", pulumiSDKModule + "0/go/pulumi.RegisterResource", "", pulumiSDKModule + "0/go/pulumi", ""},
		{"application", "example.com/app/components.NewSite", "", "example.com/app", ""},
		{"nested module", "example.com/app/plugin/handler.Handle", "", "example.com/app/plugin", ""},
		{
			"generic receiver", "example.com/app/components.(*Builder[example.com/other.Type]).Build", "",
			"example.com/app", "",
		},
		{"escaped package", "gopkg.in/yaml%2ev3.Marshal", "", "gopkg.in/yaml.v3", ""},
		{"runtime", "runtime.goexit", filepath.Join(stackTraceRuntimeSource(), "runtime/asm_arm64.s"), "runtime", "runtime"},
		{"trimpath runtime", "net/http.(*Server).Serve", "net/http/server.go", "net/http", "runtime"},
		{"unknown", "", "", "", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			module, origin := stackFrameIdentity(runtime.Frame{Function: test.function, File: test.file}, modules)
			assert.Equal(t, test.module, module)
			assert.Equal(t, test.origin, origin)
		})
	}
}

func TestStackTraceRetainsDeepFramesAndMetadata(t *testing.T) {
	t.Parallel()
	ctx := &Context{}
	var capture func(int) *pulumirpc.StackTrace
	capture = func(depth int) *pulumirpc.StackTrace {
		if depth == 0 {
			return ctx.getStackTrace(0)
		}
		return capture(depth - 1)
	}
	trace := capture(300)
	require.Greater(t, len(trace.Frames), 300)
	assert.Equal(t, pulumiSDKModule, trace.Frames[0].Module)
	assert.Equal(t, "pulumi", trace.Frames[0].Origin)
	assert.Equal(t, "runtime", trace.Frames[len(trace.Frames)-1].Module)
	assert.Equal(t, "runtime", trace.Frames[len(trace.Frames)-1].Origin)
}
