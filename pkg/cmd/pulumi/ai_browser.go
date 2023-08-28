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

package main

import (
	"context"
	"io"
	"net/url"
	"os"

	"github.com/pkg/browser"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

type aiBrowserCmd struct {
	appURL  string
	prompt  string
	autoRun bool

	Stdout io.Writer // defaults to os.Stdout

	// currentBackend is a reference to the top-level currentBackend function.
	// This is used to override the default implementation for testing purposes.
	currentBackend func(context.Context, *workspace.Project, display.Options) (backend.Backend, error)
}

func (cmd *aiBrowserCmd) Run(ctx context.Context, args []string) error {
	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}

	if cmd.currentBackend == nil {
		cmd.currentBackend = currentBackend
	}
	requestURL, err := url.Parse(cmd.appURL)
	if err != nil {
		return err
	}
	query := requestURL.Query()
	if cmd.prompt != "" {
		query.Set("prompt", cmd.prompt)
	}
	if cmd.autoRun {
		query.Set("autoRun", "true")
	}

	requestURL.RawQuery = query.Encode()
	err = browser.OpenURL(requestURL.String())
	if err != nil {
		return err
	}

	return nil
}

func newAIBrowserCommand() *cobra.Command {
	var aibcmd aiBrowserCmd
	aibcmd.appURL = env.AIServiceEndpoint.Value()
	if aibcmd.appURL == "" {
		aibcmd.appURL = "https://www.pulumi.com/ai"
	}
	cmd := &cobra.Command{
		Use:   "browser",
		Short: "Opens Pulumi AI in your local browser",
		Long:  "Opens Pulumi AI in your local browser",
		Args:  cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			ctx := commandContext()
			return aibcmd.Run(ctx, args)
		},
		),
	}
	cmd.PersistentFlags().StringVar(
		&aibcmd.prompt, "prompt", "",
		"Initial prompt to populate the app with",
	)
	cmd.PersistentFlags().BoolVar(
		&aibcmd.autoRun, "auto-run", false,
		"Automatically send the prompt",
	)
	return cmd
}
