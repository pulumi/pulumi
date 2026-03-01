// Copyright 2016-2026, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
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
	ws                pkgWorkspace.Context
	disableAutoSubmit bool
	language          PulumiAILanguage

	Stdout io.Writer // defaults to os.Stdout

	// currentBackend is a reference to the top-level currentBackend function.
	// This is used to override the default implementation for testing purposes.
	currentBackend func(
		context.Context, pkgWorkspace.Context, cmdBackend.LoginManager, *workspace.Project, display.Options,
	) (backend.Backend, error)
	openBrowser func(url string) error
}

func (cmd *aiWebCmd) Run(ctx context.Context, args []string) error {
	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}

	var prompt string
	if len(args) > 0 {
		prompt = args[0]
	}
	if prompt != "" && cmd.language != "" {
		prompt = fmt.Sprintf("%s\n\nPlease use %s.", prompt, cmd.language)
	}

	project, _, err := cmd.ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return err
	}

	if cmd.currentBackend == nil {
		cmd.currentBackend = cmdBackend.CurrentBackend
	}

	opts := display.Options{
		Color: cmdutil.GetGlobalColorization(),
	}

	b, err := cmd.currentBackend(ctx, cmd.ws, cmdBackend.DefaultLoginManager, project, opts)
	if err != nil {
		return fmt.Errorf("failed to get current backend: %w", err)
	}

	// Check if it's a cloud backend
	cloudBackend, isCloud := b.(httpstate.Backend)
	if !isCloud {
		return errors.New("Neo tasks are only available with the Pulumi Cloud backend. " +
			"Please run 'pulumi login' to connect to Pulumi Cloud.")
	}

	// Get the default org for the URL
	orgName, err := backend.GetDefaultOrg(ctx, b, project)
	if err != nil {
		return fmt.Errorf("failed to get default organization: %w", err)
	}
	if orgName == "" {
		// Fallback to username if no default org
		orgName, _, _, err = b.CurrentUser()
		if err != nil {
			return fmt.Errorf("could not determine organization: %w", err)
		}
	}

	if cmd.disableAutoSubmit || prompt == "" {
		// Open Neo console without creating a task
		consoleURL := cloudBackend.CloudConsoleURL(orgName, "neo", "tasks")
		parsedURL, err := url.Parse(consoleURL)
		if err != nil {
			return fmt.Errorf("failed to parse console URL: %w", err)
		}
		if prompt != "" {
			query := parsedURL.Query()
			query.Set("prompt", prompt)
			parsedURL.RawQuery = query.Encode()
		}

		browserURL := parsedURL.String()
		if err = cmd.openBrowser(browserURL); err != nil {
			fmt.Fprintf(cmd.Stdout,
				"We couldn't launch your web browser. Please visit:\n\n%s\n\n"+
					"to continue with Pulumi Neo.\n", browserURL)
			return errors.Join(err, fmt.Errorf("failed to open URL: %s", browserURL))
		}
		return nil
	}

	// Try to get the current stack for context
	var stackRef backend.StackReference
	stack, err := state.CurrentStack(ctx, cmd.ws, b)
	if err == nil && stack != nil {
		stackRef = stack.Ref()
	}

	neoURL, err := cloudBackend.CreateNeoTask(ctx, stackRef, prompt)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.Stdout, "\nPulumi Neo task created successfully!\n")
	fmt.Fprintf(cmd.Stdout, "View your task at:\n%s\n\n", neoURL)

	// Open the browser to the task URL
	if err = cmd.openBrowser(neoURL); err != nil {
		fmt.Fprintf(cmd.Stdout,
			"We couldn't launch your web browser automatically. Please visit the URL above.\n")
	}

	return nil
}

func newAIWebCommand(ws pkgWorkspace.Context) *cobra.Command {
	var aiwebcmd aiWebCmd
	aiwebcmd.ws = ws
	aiwebcmd.openBrowser = browser.OpenURL

	cmd := &cobra.Command{
		Use:   "web [prompt]",
		Short: "Open Pulumi Neo in your browser",
		Long: `Open Pulumi Neo in your browser

This command opens Pulumi Neo, Pulumi's AI assistant that can help you with
infrastructure as code tasks.

When a prompt is provided, it creates a new task and opens it in your browser.
Without a prompt, it simply opens the Pulumi Neo interface.

Use --no-auto-submit to open Pulumi Neo with your prompt pre-filled without
automatically creating a task.

Example:
  pulumi ai web "Create an S3 bucket in Python"
  pulumi ai web --no-auto-submit "Help me with my infrastructure"
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return aiwebcmd.Run(ctx, args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "prompt"}},
		Required:  0,
	})

	cmd.PersistentFlags().BoolVar(
		&aiwebcmd.disableAutoSubmit, "no-auto-submit", false,
		"Open Pulumi Neo with the prompt pre-filled without automatically creating a task",
	)
	cmd.PersistentFlags().VarP(
		&aiwebcmd.language, "language", "l",
		"Language to use for the prompt - appends a note to the prompt to specify the language. "+
			"[TypeScript, Python, Go, C#, Java, YAML]",
	)
	return cmd
}
