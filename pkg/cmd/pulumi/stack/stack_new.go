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

package stack

// AI Generated - needs human review

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// stackNewClient is the narrow subset of the cloud client used by the new
// command. Tests stub this with a recording mock.
type stackNewClient interface {
	CreateStack(
		ctx context.Context,
		stackID client.StackIdentifier,
		tags map[apitype.StackTagName]string,
		teams []string,
		state *apitype.UntypedDeployment,
		config *apitype.StackConfig,
	) (apitype.Stack, client.CreateStackDetails, error)
}

// stackNewClientFactory resolves a cloud client plus the organization the
// stack should be created in. orgFlag is the raw value of --org (empty means
// "use the default org").
type stackNewClientFactory func(
	ctx context.Context, orgFlag string,
) (stackNewClient, string, error)

// stackNewArgs collects the flag values for runStackNew.
type stackNewArgs struct {
	org             string
	environment     string
	secretsProvider string
	encryptedKey    string
	encryptionSalt  string
	outputFormat    outputflag.OutputFlag[stackNewRenderFunc]
}

// defaultStackNewOutputFormat wires the OutputFlag to the per-format renderers
// so `--output` selects between them.
func defaultStackNewOutputFormat() outputflag.OutputFlag[stackNewRenderFunc] {
	return outputflag.OutputFlag[stackNewRenderFunc]{
		RenderForTerminal: renderStackNewText,
		RenderJSON:        renderStackNewJSON,
	}
}

func newStackNewCmd() *cobra.Command {
	return newStackNewCmdWith(defaultStackNewClientFactory)
}

func newStackNewCmdWith(factory stackNewClientFactory) *cobra.Command {
	contract.Assertf(factory != nil, "stackNewClientFactory must not be nil")
	var args stackNewArgs
	args.outputFormat = defaultStackNewOutputFormat()

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "new <project> <name>",
		Short:  "[EXPERIMENTAL] Create a new stack",
		Long: "[EXPERIMENTAL] Create a new stack.\n" +
			"\n" +
			"Creates a new stack within a project in the organization. If the project\n" +
			"does not exist it will be created. A stack is an isolated, independently\n" +
			"configurable instance of a Pulumi program, typically representing a\n" +
			"deployment environment (e.g. development, staging, production). The\n" +
			"stack name must be unique within the project. This command does not\n" +
			"select the new stack as the current stack or write any local project\n" +
			"files.\n" +
			"\n" +
			"Default output is a human-readable confirmation; pass --output=json for\n" +
			"the created stack identity and any backend messages as JSON.",
		Example: "  # Create a stack in the default organization\n" +
			"  pulumi stack new my-project dev\n\n" +
			"  # Create a stack in a specific organization with an ESC environment\n" +
			"  pulumi stack new my-project prod --org acme --environment acme/prod\n\n" +
			"  # Create a stack and emit JSON for scripting\n" +
			"  pulumi stack new my-project dev --output json",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			return runStackNew(cmd.Context(), cmd.OutOrStdout(), factory, posArgs[0], posArgs[1], args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "project"},
			{Name: "name"},
		},
		Required: 2,
	})

	cmd.Flags().StringVar(&args.org, "org", "", "The organization to create the stack in")
	cmd.Flags().StringVar(&args.environment, "environment", "",
		"Reference to an ESC environment for storing stack configuration")
	cmd.Flags().StringVar(&args.secretsProvider, "secrets-provider", "",
		"The secrets provider for the stack")
	cmd.Flags().StringVar(&args.encryptedKey, "encrypted-key", "",
		"KMS-encrypted ciphertext for the data key (cloud-based secrets providers)")
	cmd.Flags().StringVar(&args.encryptionSalt, "encryption-salt", "",
		"Base64-encoded encryption salt (passphrase-based secrets providers)")
	outputflag.VarP(cmd.Flags(), &args.outputFormat)

	return cmd
}

// defaultStackNewClientFactory is the production wiring: resolve the cloud
// backend, pick the effective organization, and hand back the underlying
// *client.Client.
func defaultStackNewClientFactory(
	ctx context.Context, orgFlag string,
) (stackNewClient, string, error) {
	ws := pkgWorkspace.Instance
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	be, err := cmdBackend.CurrentBackend(ctx, ws, cmdBackend.DefaultLoginManager, nil, opts)
	if err != nil {
		return nil, "", err
	}
	cloudBackend, ok := be.(httpstate.Backend)
	if !ok {
		return nil, "", errors.New(
			"creating a stack requires the Pulumi Cloud backend; run `pulumi login`")
	}

	userName, orgs, _, err := cloudBackend.CurrentUser()
	if err != nil {
		return nil, "", err
	}

	org := orgFlag
	if org == "" {
		defaultOrg, err := cloudBackend.GetDefaultOrg(ctx)
		if err != nil {
			return nil, "", err
		}
		org = defaultOrg
	}
	if org == "" {
		org = userName
	}

	if !slices.Contains(orgs, org) && org != userName {
		return nil, "", fmt.Errorf("user %s is not a member of organization %s", userName, org)
	}

	return cloudBackend.Client(), org, nil
}

// runStackNew is the cobra-decoupled body of `pulumi stack new`, so tests can
// drive it directly with a buffer.
func runStackNew(
	ctx context.Context,
	w io.Writer,
	factory stackNewClientFactory,
	project, name string,
	args stackNewArgs,
) error {
	stackName, err := tokens.ParseStackName(name)
	if err != nil {
		return fmt.Errorf("creating stack: %w", err)
	}

	c, org, err := factory(ctx, args.org)
	if err != nil {
		return err
	}

	stackID := client.StackIdentifier{
		Owner:   org,
		Project: project,
		Stack:   stackName,
	}

	cfg := buildStackNewConfig(args)

	_, details, err := c.CreateStack(ctx, stackID, nil, nil, nil, cfg)
	if err != nil {
		return fmt.Errorf("creating stack: %w", err)
	}

	return args.outputFormat.Get()(w, stackID, details)
}

// buildStackNewConfig returns a *StackConfig only if at least one of the
// config-shaped flags is set; otherwise nil so we don't send an empty config
// object to the backend.
func buildStackNewConfig(args stackNewArgs) *apitype.StackConfig {
	if args.environment == "" && args.secretsProvider == "" &&
		args.encryptedKey == "" && args.encryptionSalt == "" {
		return nil
	}
	return &apitype.StackConfig{
		Environment:     args.environment,
		SecretsProvider: args.secretsProvider,
		EncryptedKey:    args.encryptedKey,
		EncryptionSalt:  args.encryptionSalt,
	}
}

type stackNewRenderFunc func(w io.Writer, stackID client.StackIdentifier, details client.CreateStackDetails) error

func renderStackNewText(
	w io.Writer, stackID client.StackIdentifier, details client.CreateStackDetails,
) error {
	fmt.Fprintf(w, "Created stack %s\n", stackID)
	fmt.Fprintf(w, "%-15s %s\n", "Organization:", stackID.Owner)
	fmt.Fprintf(w, "%-15s %s\n", "Project:", stackID.Project)
	fmt.Fprintf(w, "%-15s %s\n", "Stack:", stackID.Stack.String())
	for _, m := range details.Messages {
		fmt.Fprintln(w, m.Message)
	}
	return nil
}

// stackNewJSON is the JSON envelope emitted by `pulumi stack new --output=json`.
type stackNewJSON struct {
	OrganizationName string            `json:"organizationName"`
	ProjectName      string            `json:"projectName"`
	StackName        string            `json:"stackName"`
	Messages         []apitype.Message `json:"messages"`
}

func renderStackNewJSON(
	w io.Writer, stackID client.StackIdentifier, details client.CreateStackDetails,
) error {
	messages := details.Messages
	if messages == nil {
		messages = []apitype.Message{}
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(stackNewJSON{
		OrganizationName: stackID.Owner,
		ProjectName:      stackID.Project,
		StackName:        stackID.Stack.String(),
		Messages:         messages,
	})
}
