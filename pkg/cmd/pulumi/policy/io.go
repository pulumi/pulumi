// Copyright 2024, Pulumi Corporation.
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

package policy

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/opentracing/opentracing-go"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// ReadPolicyProject attempts to detect and read a Pulumi PolicyPack project for the current
// workspace. If the project is successfully detected and read, it is returned along with the path
// to its containing directory, which will be used as the root of the project's Pulumi program.
func ReadPolicyProject(pwd string) (*workspace.PolicyPackProject, string, string, error) {
	// Now that we got here, we have a path, so we will try to load it.
	path, err := workspace.DetectPolicyPackPathFrom(pwd)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to find current Pulumi project because of "+
			"an error when searching for the PulumiPolicy.yaml file (searching upwards from %s)"+": %w", pwd, err)
	} else if path == "" {
		return nil, "", "", fmt.Errorf("no PulumiPolicy.yaml project file found (searching upwards from %s)", pwd)
	}
	proj, err := workspace.LoadPolicyPack(path)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to load Pulumi policy project located at %q: %w", path, err)
	}

	return proj, path, filepath.Dir(path), nil
}

func InstallPolicyPackDependencies(ctx context.Context, root string, proj *workspace.PolicyPackProject) error {
	span := opentracing.SpanFromContext(ctx)
	// Bit of a hack here. Creating a plugin context requires a "program project", but we've only got a
	// policy project. Ideally we should be able to make a plugin context without any related project. But
	// fow now this works.
	projinfo := &engine.Projinfo{Proj: &workspace.Project{
		Main:    proj.Main,
		Runtime: proj.Runtime,
	}, Root: root}
	_, main, pluginCtx, err := engine.ProjectInfoContext(
		projinfo,
		nil,
		cmdutil.Diag(),
		cmdutil.Diag(),
		nil,
		false,
		span,
		nil,
	)
	if err != nil {
		return err
	}
	defer pluginCtx.Close()

	programInfo := plugin.NewProgramInfo(pluginCtx.Root, pluginCtx.Pwd, main, proj.Runtime.Options())
	lang, err := pluginCtx.Host.LanguageRuntime(proj.Runtime.Name(), programInfo)
	if err != nil {
		return fmt.Errorf("failed to load language plugin %s: %w", proj.Runtime.Name(), err)
	}

	if err = lang.InstallDependencies(plugin.InstallDependenciesRequest{Info: programInfo}); err != nil {
		return fmt.Errorf("installing dependencies failed: %w", err)
	}

	return nil
}
