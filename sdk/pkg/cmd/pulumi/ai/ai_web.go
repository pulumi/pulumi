// Copyright 2016-2024, Pulumi Corporation.
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

package ai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/pkg/browser"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
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
	currentBackend func(
		context.Context, pkgWorkspace.Context, cmdBackend.LoginManager, *workspace.Project, display.Options,
	) (backend.Backend, error)
}

func (cmd *aiWebCmd) Run(ctx context.Context, args []string) error {
	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}

	if cmd.currentBackend == nil {
		cmd.currentBackend = cmdBackend.CurrentBackend
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
			return errors.New(
				"prompt must be provided when auto-submit is enabled.\n" +
					"Example: 'pulumi ai web \"Create an S3 bucket in Python\"'\n" +
					"Alternatively, use --no-auto-submit to open the app without a prompt",
			)
		}
		query.Set("autoSubmit", "true")
	}
	if cmd.language == "" {
		cmd.language = TypeScript // TODO: default to the language of the current project if one is present
	}
	query.Set("language", cmd.language.String())

	requestURL.RawQuery = query.Encode()
	if err = browser.OpenURL(requestURL.String()); err != nil {
		fmt.Printf("We couldn't launch your web browser for some reason. Please visit:\n\n%s\n\n"+
			"to continue your Pulumi AI session.\n", requestURL)
		return errors.Join(err, fmt.Errorf("failed to open URL: %s", requestURL.String()))
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
		Use:   "web <prompt|--no-auto-submit>",
		Short: "Opens Pulumi AI in your local browser",
		Long: `Opens Pulumi AI in your local browser

This command opens the Pulumi AI web app in your local default browser.
It can be further initialized by providing a prompt to pre-fill in the app,
with the default behavior then automatically submitting that prompt to Pulumi AI.

If you do not want to submit the prompt to Pulumi AI, you can opt-out of this
by passing the --no-auto-submit flag.

Example:
  pulumi ai web "Create an S3 bucket in Python"
`,
		Args: cmdutil.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return aiwebcmd.Run(ctx, args)
		},
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
