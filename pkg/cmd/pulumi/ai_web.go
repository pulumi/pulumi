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
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/pkg/browser"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

type PulumiAILanguage string

const (
	TypeScript PulumiAILanguage = "TypeScript"
	Python     PulumiAILanguage = "Python"
	Go         PulumiAILanguage = "Go"
	CSharp     PulumiAILanguage = "C#"
	Java       PulumiAILanguage = "Java"
	YAML       PulumiAILanguage = "YAML"
)

func (l *PulumiAILanguage) String() string {
	return string(*l)
}

func (l *PulumiAILanguage) Set(v string) error {
	switch strings.ToLower(v) {
	case "typescript", "ts":
		*l = TypeScript
	case "python", "py":
		*l = Python
	case "go", "golang":
		*l = Go
	case "c#", "csharp":
		*l = CSharp
	case "java":
		*l = Java
	case "yaml":
		*l = YAML
	default:
		return fmt.Errorf("invalid language %q", v)
	}
	return nil
}

func (l *PulumiAILanguage) Type() string {
	return "string"
}

type aiWebCmd struct {
	appURL            string
	disableAutoSubmit bool
	language          PulumiAILanguage

	Stdout io.Writer // defaults to os.Stdout

	// currentBackend is a reference to the top-level currentBackend function.
	// This is used to override the default implementation for testing purposes.
	currentBackend func(context.Context, *workspace.Project, display.Options) (backend.Backend, error)
}

func (cmd *aiWebCmd) Run(ctx context.Context, args []string) error {
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
	if len(args) > 0 {
		query.Set("prompt", args[0])
	}
	if !cmd.disableAutoSubmit {
		if len(args) == 0 {
			return fmt.Errorf("prompt must be provided when auto-submit is enabled")
		}
		query.Set("autoSubmit", "true")
	}
	query.Set("language", cmd.language.String())

	requestURL.RawQuery = query.Encode()
	err = browser.OpenURL(requestURL.String())
	if err != nil {
		return err
	}

	return nil
}

func newAIWebCommand() *cobra.Command {
	var aiwebcmd aiWebCmd
	aiwebcmd.appURL = env.AIServiceEndpoint.Value()
	if aiwebcmd.appURL == "" {
		aiwebcmd.appURL = "https://www.pulumi.com/ai"
	}
	cmd := &cobra.Command{
		Use:   "web",
		Short: "Opens Pulumi AI in your local browser",
		Long: `Opens Pulumi AI in your local browser

This command opens the Pulumi AI web app in your local default browser.
It can be further initialized by providing a prompt to pre-fill in the app,
with the default behavior then automatically submitting that prompt to Pulumi AI.

If no prompt is provided, the app will be opened with no prompt pre-filled.

If you do not want to submit the prompt to Pulumi AI, you can opt-out of this
by passing the --no-auto-submit flag.
`,
		Args: cmdutil.MaximumNArgs(1),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			ctx := commandContext()
			return aiwebcmd.Run(ctx, args)
		},
		),
	}
	cmd.PersistentFlags().BoolVar(
		&aiwebcmd.disableAutoSubmit, "no-auto-submit", false,
		"Opt-out of automatically submitting the prompt to Pulumi AI",
	)
	cmd.PersistentFlags().VarP(
		&aiwebcmd.language, "language", "l",
		"Language to use for the prompt - this defaults to TypeScript. [TypeScript, Python, Go, C#, Java, YAML]",
	)
	return cmd
}
