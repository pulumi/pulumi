// Copyright 2016-2023, Pulumi Corporation.
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

package workspace

import (
	"errors"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	"github.com/stretchr/testify/assert"
)

func TestInstallPluginErrorText(t *testing.T) {
	t.Parallel()

	v1 := semver.MustParse("0.1.0")
	err := errors.New("some error")
	tests := []struct {
		Name          string
		Err           InstallPluginError
		ExpectedError string
	}{
		{
			Name: "Just name",
			Err: InstallPluginError{
				Err: err,
				Spec: workspace.PluginSpec{
					Name: "myplugin",
					Kind: workspace.ResourcePlugin,
				},
			},
			ExpectedError: "Could not automatically download and install resource plugin 'pulumi-resource-myplugin'," +
				" install the plugin using `pulumi plugin install resource myplugin`: some error",
		},
		{
			Name: "Different kind",
			Err: InstallPluginError{
				Err: err,
				Spec: workspace.PluginSpec{
					Name: "myplugin",
					Kind: workspace.ConverterPlugin,
				},
			},
			ExpectedError: "Could not automatically download and install converter plugin 'pulumi-converter-myplugin'," +
				" install the plugin using `pulumi plugin install converter myplugin`: some error",
		},
		{
			Name: "Name and version",
			Err: InstallPluginError{
				Err: err,
				Spec: workspace.PluginSpec{
					Name:    "myplugin",
					Kind:    workspace.ResourcePlugin,
					Version: &v1,
				},
			},
			ExpectedError: "Could not automatically download and install resource plugin 'pulumi-resource-myplugin'" +
				" at version v0.1.0, install the plugin using `pulumi plugin install resource myplugin v0.1.0`: some error",
		},
		{
			Name: "Name and version and URL",
			Err: InstallPluginError{
				Err: err,
				Spec: workspace.PluginSpec{
					Name:              "myplugin",
					Kind:              workspace.ResourcePlugin,
					Version:           &v1,
					PluginDownloadURL: "github://owner/repo",
				},
			},
			ExpectedError: "Could not automatically download and install resource plugin 'pulumi-resource-myplugin'" +
				" at version v0.1.0, install the plugin using `pulumi plugin install resource myplugin v0.1.0" +
				" --server github://owner/repo`: some error",
		},
		{
			Name: "Name and URL",
			Err: InstallPluginError{
				Err: err,
				Spec: workspace.PluginSpec{
					Name:              "myplugin",
					Kind:              workspace.ResourcePlugin,
					PluginDownloadURL: "github://owner/repo",
				},
			},
			ExpectedError: "Could not automatically download and install resource plugin 'pulumi-resource-myplugin'," +
				" install the plugin using `pulumi plugin install resource myplugin" +
				" --server github://owner/repo`: some error",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()
			assert.EqualError(t, &tt.Err, tt.ExpectedError)
		})
	}
}
