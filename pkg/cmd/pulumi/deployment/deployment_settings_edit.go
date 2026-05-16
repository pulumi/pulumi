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

package deployment

// AI Generated - needs human review

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// deploymentSettingsEditClient is the narrow API surface this command depends
// on.
type deploymentSettingsEditClient interface {
	PatchStackDeploymentSettings(
		ctx context.Context, stack client.StackIdentifier, patch json.RawMessage,
	) error
	GetStackDeploymentSettings(
		ctx context.Context, stack client.StackIdentifier,
	) (*apitype.DeploymentSettings, error)
}

// deploymentSettingsEditClientFactory resolves a client and StackIdentifier
// for the edit command. stackFlag carries the raw `--stack` value (empty means
// "use the current stack").
type deploymentSettingsEditClientFactory func(
	ctx context.Context, stackFlag string,
) (deploymentSettingsEditClient, client.StackIdentifier, error)

// deploymentSettingsEditArgs collects the resolved flag values so Run can be
// driven directly from tests.
type deploymentSettingsEditArgs struct {
	stack        string
	file         string
	outputFormat outputflag.OutputFlag[deploymentSettingsGetRenderFunc]
}

// newDeploymentSettingsEditCmd builds `pulumi deployment settings edit` wired
// to the real cloud client factory.
func newDeploymentSettingsEditCmd() *cobra.Command {
	return newDeploymentSettingsEditCmdWith(defaultDeploymentSettingsEditClientFactory)
}

func newDeploymentSettingsEditCmdWith(factory deploymentSettingsEditClientFactory) *cobra.Command {
	contract.Assertf(factory != nil, "deploymentSettingsEditClientFactory must not be nil")
	var args deploymentSettingsEditArgs
	args.outputFormat = defaultDeploymentSettingsGetOutputFormat()

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "edit",
		Short:  "[EXPERIMENTAL] Create or update deployment settings for a stack",
		Long: "[EXPERIMENTAL] Create or update deployment settings for a stack.\n" +
			"\n" +
			"Applies a JSON patch to the stack's Pulumi Deployments settings. If no\n" +
			"settings exist they are created from the patch.\n" +
			"\n" +
			"Use --file to point at a JSON document containing the patch; pass `-` to\n" +
			"read the patch from stdin. On success the resulting settings are printed\n" +
			"in the same format as `pulumi deployment settings get`.\n" +
			"\n" +
			"Default output is a human-readable summary; pass --output=json for the\n" +
			"raw response as JSON.",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			return runDeploymentSettingsEdit(cmd.Context(), cmd.OutOrStdout(), os.Stdin, factory, args)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVarP(&args.stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().StringVarP(&args.file, "file", "f", "",
		"Read settings patch from file; `-` reads stdin")
	outputflag.VarP(cmd.Flags(), &args.outputFormat)
	contract.AssertNoErrorf(cmd.MarkFlagRequired("file"), "marking --file required")

	return cmd
}

// defaultDeploymentSettingsEditClientFactory mirrors the production wiring
// used elsewhere: resolve the stack, ensure we're on the Pulumi Cloud backend,
// and hand back the underlying *client.Client.
func defaultDeploymentSettingsEditClientFactory(
	ctx context.Context, stackFlag string,
) (deploymentSettingsEditClient, client.StackIdentifier, error) {
	ws := pkgWorkspace.Instance
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	s, err := cmdStack.RequireStack(ctx, cmdutil.Diag(), ws, cmdBackend.DefaultLoginManager,
		stackFlag, cmdStack.LoadOnly, opts)
	if err != nil {
		return nil, client.StackIdentifier{}, fmt.Errorf("resolving stack: %w", err)
	}

	cloudStack, ok := s.(httpstate.Stack)
	if !ok {
		return nil, client.StackIdentifier{},
			errors.New("editing deployment settings requires the Pulumi Cloud backend; run `pulumi login`")
	}

	ref := cloudStack.Ref()
	project := ""
	if p, ok := ref.Project(); ok {
		project = string(p)
	}
	stackID := client.StackIdentifier{
		Owner:   cloudStack.OrgName(),
		Project: project,
		Stack:   ref.Name(),
	}

	be, ok := cloudStack.Backend().(httpstate.Backend)
	if !ok {
		return nil, client.StackIdentifier{},
			errors.New("editing deployment settings requires the Pulumi Cloud backend; run `pulumi login`")
	}
	return be.Client(), stackID, nil
}

// runDeploymentSettingsEdit is the cobra-decoupled entry point so tests can
// drive the command without parsing flags. stdin is an injectable reader used
// when --file is `-`.
func runDeploymentSettingsEdit(
	ctx context.Context, w io.Writer, stdin io.Reader,
	factory deploymentSettingsEditClientFactory, args deploymentSettingsEditArgs,
) error {
	if args.file == "" {
		return errors.New("--file is required (use `-` to read the patch from stdin)")
	}

	patch, err := readDeploymentSettingsPatch(args.file, stdin)
	if err != nil {
		return fmt.Errorf("reading deployment settings patch: %w", err)
	}

	c, stackID, err := factory(ctx, args.stack)
	if err != nil {
		return err
	}

	if err := c.PatchStackDeploymentSettings(ctx, stackID, patch); err != nil {
		return fmt.Errorf("editing deployment settings: %w", err)
	}

	resp, err := c.GetStackDeploymentSettings(ctx, stackID)
	if err != nil {
		return fmt.Errorf("getting deployment settings: %w", err)
	}
	if resp == nil {
		resp = &apitype.DeploymentSettings{}
	}

	return args.outputFormat.Get()(w, *resp)
}

// readDeploymentSettingsPatch reads the JSON patch from path (or stdin when
// path is `-`). The bytes are validated against apitype.DeploymentSettings
// (with unknown fields rejected) so typos surface here instead of silently
// no-op'ing on the server, but the original bytes are sent through verbatim
// so that we can send partial objects (undefined fields) or null for deleting
// fields.
func readDeploymentSettingsPatch(path string, stdin io.Reader) (json.RawMessage, error) {
	var raw []byte
	var err error
	if path == "-" {
		if stdin == nil {
			return nil, errors.New("no stdin reader available")
		}
		raw, err = io.ReadAll(stdin)
		if err != nil {
			return nil, err
		}
	} else {
		raw, err = os.ReadFile(path)
		if err != nil {
			return nil, err
		}
	}

	if len(raw) == 0 || isAllWhitespace(raw) {
		return nil, errors.New("patch file is empty")
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var probe apitype.DeploymentSettings
	if err := dec.Decode(&probe); err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

func isAllWhitespace(b []byte) bool {
	for _, c := range b {
		switch c {
		case ' ', '\t', '\n', '\r':
			continue
		default:
			return false
		}
	}
	return true
}
