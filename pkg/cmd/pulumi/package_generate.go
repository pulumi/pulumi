// Copyright 2016-2018, Pulumi Corporation.
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

package main

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
)

type Action int

const (
	Generate Action = iota
	Build
	Publish
	Install
	New
)

func (a Action) String() string {
	return [...]string{"generate", "build", "publish", "install", "new"}[a]
}

func doPkgUp(opts backend.UpdateOptions, action Action, version string) result.Result {
	s, err := requireStack("", true, opts.Display, false /*setCurrent*/)
	if err != nil {
		return result.FromError(err)
	}

	ps, err := loadProjectStack(s)
	if err != nil {
		return result.FromError(err)
	}

	key, err := config.ParseKey(fmt.Sprintf("pulumi-package:action"))
	if err != nil {
		return result.FromError(err)
	}

	v := config.NewValue(action.String())

	err = ps.Config.Set(key, v, false)
	if err != nil {
		return result.FromError(err)
	}

	if version != "" {
		key, err := config.ParseKey(fmt.Sprintf("pulumi-package:version"))
		if err != nil {
			return result.FromError(err)
		}

		v := config.NewValue(version)

		err = ps.Config.Set(key, v, false)
		if err != nil {
			return result.FromError(err)
		}
	}

	proj, root, err := readProjectForUpdate("")
	if err != nil {
		return result.FromError(err)
	}

	m, err := getUpdateMetadata("", root, "", "")
	if err != nil {
		return result.FromError(errors.Wrap(err, "gathering environment metadata"))
	}

	sm, err := getStackSecretsManager(s)
	if err != nil {
		return result.FromError(errors.Wrap(err, "getting secrets manager"))
	}

	cfg, err := getStackConfiguration(s, sm)
	if err != nil {
		return result.FromError(errors.Wrap(err, "getting stack configuration"))
	}

	opts.Engine = engine.UpdateOptions{
		UseLegacyDiff:             useLegacyDiff(),
		DisableProviderPreview:    disableProviderPreview(),
		DisableResourceReferences: disableResourceReferences(),
	}

	_, res := s.Update(commandContext(), backend.UpdateOperation{
		Proj:               proj,
		Root:               root,
		M:                  m,
		Opts:               opts,
		StackConfiguration: cfg,
		SecretsManager:     sm,
		Scopes:             cancellationScopes,
	})
	switch {
	case res != nil && res.Error() == context.Canceled:
		return result.FromError(errors.New("update cancelled"))
	case res != nil:
		return PrintEngineResult(res)
	default:
		return nil
	}
}

func newPackageGenerateCmd() *cobra.Command {

	var cmd = &cobra.Command{
		Use:   "generate",
		Args:  cmdutil.MaximumNArgs(3),
		Short: "Generate SDKs for your pulumi package.",
		Long:  "Generate SDKs for your pulumi package.",
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, args []string) result.Result {
			yes := true
			skipPreview := true
			interactive := cmdutil.Interactive()

			opts, err := updateFlagsToOptions(interactive, skipPreview, yes)
			if err != nil {
				return result.FromError(err)
			}

			var displayType = display.DisplayProgress

			opts.Display = display.Options{
				Color:         cmdutil.GetGlobalColorization(),
				IsInteractive: interactive,
				Type:          displayType,
			}
			opts.Display.SuppressPermaLink = false

			return doPkgUp(opts, Generate, "")
		}),
	}
	return cmd
}
