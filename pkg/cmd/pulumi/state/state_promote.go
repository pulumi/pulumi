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

package state

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/secrets"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageworkspace"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/codegen/convert"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	pkghost "github.com/pulumi/pulumi/pkg/v3/host"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	sdkWorkspace "github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func newStatePromoteCommand(ws pkgWorkspace.Context, lm cmdBackend.LoginManager) *cobra.Command {
	var stackName string
	var yes bool

	cmd := &cobra.Command{
		Use:   "promote <snippet-name>",
		Short: "Promote a snippet from state into Pulumi program code",
		Long: `Promote a snippet from state into Pulumi program code

This command generates Pulumi program code for a stateful snippet, prints the
generated files, then removes the snippet from the stack while leaving the
resources it registered in state. The argument is the snippet's logical name.

This command must be run from a real Pulumi project so the generated code has
a backing project runtime.`,
		Example: "pulumi state promote myBucket",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			sink := cmdutil.Diag()
			yes = yes || env.SkipConfirmations.Value()

			proj, root, err := requireProject(ws)
			if err != nil {
				return err
			}

			opts := display.Options{Color: cmdutil.GetGlobalColorization()}
			s, err := cmdStack.RequireStack(ctx, sink, ws, lm, stackName, cmdStack.LoadOnly, opts, "")
			if err != nil {
				return fmt.Errorf("load stack: %w", err)
			}
			snap, err := s.Snapshot(ctx, secrets.DefaultProvider)
			if err != nil {
				return fmt.Errorf("load stack snapshot: %w", err)
			}
			if snap == nil {
				return fmt.Errorf("no state found for stack %s", s.Ref())
			}

			snippet, err := resolveSnippetForPromote(snap, args[0])
			if err != nil {
				return err
			}
			if err := validateSnippetPromoteReferences(snap, snippet); err != nil {
				return err
			}

			files, err := generateSnippetProgram(ctx, sink, ws, lm, proj, root, snippet)
			if err != nil {
				return fmt.Errorf("generate program: %w", err)
			}
			printGeneratedSnippetProgram(cmd.OutOrStdout(), snippet, files)

			var cleared int
			err = runTotalStateEditWithPrompt(ctx, sink, ws, lm, stackName, !yes,
				func(opts display.Options, snap *deploy.Snapshot) error {
					var err error
					cleared, err = promoteSnippetFromSnapshot(snap, snippet)
					return err
				},
				"This command will remove the snippet from state and leave its resources in state. Confirm?")
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"\nSnippet %q promoted from state; %d resource(s) retained\n",
				snippet.Name, cleared)
			return nil
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "snippet-name"},
		},
		Required: 1,
	})

	cmd.Flags().StringVarP(
		&stackName, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompts")
	return cmd
}

func requireProject(ws pkgWorkspace.Context) (*sdkWorkspace.Project, string, error) {
	project, root, err := ws.ReadProject("")
	if err != nil && !errors.Is(err, sdkWorkspace.ErrProjectNotFound) {
		return nil, "", err
	}
	if project == nil {
		return nil, "", errors.New("pulumi state promote must be run from a Pulumi project")
	}
	if project.Runtime.Name() == "" {
		return nil, "", errors.New("pulumi state promote requires a project runtime")
	}
	return project, root, nil
}

func promoteSnippetFromSnapshot(snap *deploy.Snapshot, expected resource.Snippet) (int, error) {
	if snap == nil {
		return 0, errors.New("no state found")
	}
	index := -1
	for i, snippet := range snap.Snippets {
		if snippet.UUID == expected.UUID {
			index = i
			break
		}
	}
	if index == -1 {
		return 0, fmt.Errorf("no snippet %q exists in the current state", expected.UUID)
	}

	snap.Snippets = append(snap.Snippets[:index], snap.Snippets[index+1:]...)
	cleared := 0
	for _, res := range snap.Resources {
		if res != nil && res.SnippetID == expected.UUID {
			res.SnippetID = ""
			cleared++
		}
	}
	return cleared, nil
}

func resolveSnippetForPromote(snap *deploy.Snapshot, name string) (resource.Snippet, error) {
	var matches []resource.Snippet
	for _, snippet := range snap.Snippets {
		if snippet.Name == name {
			matches = append(matches, snippet)
		}
	}
	switch len(matches) {
	case 0:
		return resource.Snippet{}, fmt.Errorf("no snippet named %q exists in the current state", name)
	case 1:
		return matches[0], nil
	default:
		return resource.Snippet{}, fmt.Errorf(
			"snippet name %q is ambiguous: %d snippets share this name\n"+
				"This is a bug! We would appreciate a report: https://github.com/pulumi/pulumi/issues/",
			name, len(matches))
	}
}

func validateSnippetPromoteReferences(snap *deploy.Snapshot, snippet resource.Snippet) error {
	for name, rawURN := range snippet.References {
		urn := resource.URN(rawURN)
		if !urn.IsValid() {
			return fmt.Errorf("snippet reference %q has invalid URN %q", name, rawURN)
		}
		for _, res := range snap.Resources {
			if res.URN == urn && res.SnippetID != "" {
				return fmt.Errorf(
					"snippet reference %q points to %q, which was produced by snippet %q; "+
						"pulumi state promote does not yet support references to other snippets",
					name, rawURN, res.SnippetID)
			}
		}
	}
	return nil
}

// snippetPCLSource renders a snippet as a PCL `resource` block. Name and Type
// are quoted with Go's %q; this is safe for the identifier-shaped values Pulumi
// uses (resource names and token strings), which do not exercise Go-only escape
// sequences that PCL/HCL would reject.
func snippetPCLSource(snippet resource.Snippet) string {
	code := strings.TrimRight(snippet.Code, "\n")
	if code != "" {
		lines := strings.Split(code, "\n")
		for i := range lines {
			if lines[i] != "" {
				lines[i] = "\t" + lines[i]
			}
		}
		code = "\n" + strings.Join(lines, "\n") + "\n"
	}
	return fmt.Sprintf("resource %q %q {%s}\n", snippet.Name, snippet.Type, code)
}

func generateSnippetProgram(
	ctx context.Context,
	sink diag.Sink,
	ws pkgWorkspace.Context,
	lm cmdBackend.LoginManager,
	proj *sdkWorkspace.Project,
	root string,
	snippet resource.Snippet,
) (map[string][]byte, error) {
	wd, _, err := (&engine.Projinfo{Proj: proj, Root: root}).GetPwdMain()
	if err != nil {
		return nil, fmt.Errorf("get project working directory: %w", err)
	}

	reg := cmdCmd.NewDefaultRegistry(ctx, lm, ws, proj, sink, env.Global())
	host, err := pkghost.New(
		context.WithoutCancel(ctx), sink, sink, nil, pkgWorkspace.EnsureLanguageInstalled,
		schema.NewLoaderServerFromContext, convert.NewMapperServerFromContext,
		packageworkspace.NewResolverServer(reg))
	if err != nil {
		return nil, fmt.Errorf("create plugin host: %w", err)
	}
	defer contract.IgnoreClose(host)

	pctx, err := plugin.NewContext(ctx, sink, sink, host, nil, wd, nil, true, nil)
	if err != nil {
		return nil, fmt.Errorf("create plugin context: %w", err)
	}
	defer contract.IgnoreClose(pctx)

	languagePlugin, err := pctx.Host.LanguageRuntime(pctx, proj.Runtime.Name())
	if err != nil {
		return nil, fmt.Errorf("load language runtime %q: %w", proj.Runtime.Name(), err)
	}

	loader := schema.NewPluginLoader(pctx)
	loaderServer := schema.NewLoaderServer(loader)
	grpcServer, err := plugin.NewServer(pctx, schema.LoaderRegistration(loaderServer))
	if err != nil {
		return nil, fmt.Errorf("create loader server: %w", err)
	}
	defer contract.IgnoreClose(grpcServer)

	files, diagnostics, err := languagePlugin.GenerateProgram(
		pctx.Request(),
		map[string]string{"promote.pp": snippetPCLSource(snippet)},
		grpcServer.Addr(),
		false,
	)
	if err != nil {
		return nil, err
	}
	if diagnostics.HasErrors() {
		return nil, hclDiagnosticsError(diagnostics)
	}
	return files, nil
}

func hclDiagnosticsError(diags hcl.Diagnostics) error {
	var buf bytes.Buffer
	for _, d := range diags {
		if d.Severity != hcl.DiagError {
			continue
		}
		if buf.Len() > 0 {
			buf.WriteString("; ")
		}
		buf.WriteString(d.Summary)
		if d.Detail != "" {
			buf.WriteString(": ")
			buf.WriteString(d.Detail)
		}
	}
	if buf.Len() == 0 {
		// We shouldn't ever hit this, it means all the diagnostics had empty summaries.
		return errors.New("code generation returned diagnostics")
	}
	return errors.New(buf.String())
}

func printGeneratedSnippetProgram(w io.Writer, snippet resource.Snippet, files map[string][]byte) {
	fmt.Fprintf(w, "Generated code for snippet %q:\n", snippet.Name)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(w, "\n%s\n%s\n", name, strings.Repeat("=", len(name)))
		data := files[name]
		_, err := w.Write(data)
		contract.IgnoreError(err)
		if n := len(data); n > 0 && data[n-1] != '\n' {
			fmt.Fprintln(w)
		}
	}
}
