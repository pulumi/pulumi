// Copyright 2016-2025, Pulumi Corporation.
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

package operations

import (
	"context"
	//nolint:gosec
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	survey "github.com/AlecAivazis/survey/v2"
	surveycore "github.com/AlecAivazis/survey/v2/core"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/secrets"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// MultistackArgs holds the parsed arguments for a multistack operation.
type MultistackArgs struct {
	// WorkspaceFile is the path to a Pulumispace file (from --workspace flag).
	WorkspaceFile string
	// Dirs is the list of project directories (from positional args).
	Dirs []string
	// StackName is the uniform stack name to apply to all projects (from --stack flag).
	StackName string
}

// IsMultistack returns true if the arguments indicate a multistack operation.
func (a MultistackArgs) IsMultistack() bool {
	return a.WorkspaceFile != "" || len(a.Dirs) > 1
}

// ResolveMultistackEntries resolves the multistack arguments into a list of MultistackEntry objects
// ready for orchestration. It loads each project, resolves the backend, and gets the stack reference.
func ResolveMultistackEntries(
	ctx context.Context,
	args MultistackArgs,
	ws pkgWorkspace.Context,
	lm cmdBackend.LoginManager,
	ssml cmdStack.SecretsManagerLoader,
	displayOpts display.Options,
) ([]backend.MultistackEntry, error) {
	// Determine project directories and per-project stack names.
	type stackSpec struct {
		dir       string
		stackName string
	}
	var specs []stackSpec

	if args.WorkspaceFile != "" {
		// Load from Pulumispace file.
		ps, err := workspace.LoadPulumispace(args.WorkspaceFile)
		if err != nil {
			return nil, fmt.Errorf("loading workspace file %q: %w", args.WorkspaceFile, err)
		}

		// Resolve variables.
		resolved, err := ps.Resolve(args.StackName)
		if err != nil {
			return nil, fmt.Errorf("resolving workspace file %q: %w", args.WorkspaceFile, err)
		}

		if err := resolved.Validate(); err != nil {
			return nil, fmt.Errorf("validating workspace file %q: %w", args.WorkspaceFile, err)
		}

		// Convert to stackSpecs, resolving relative paths against the workspace file's directory.
		wsDir := filepath.Dir(args.WorkspaceFile)
		for _, s := range resolved.Stacks {
			dir := s.Path
			if !filepath.IsAbs(dir) {
				dir = filepath.Join(wsDir, dir)
			}
			specs = append(specs, stackSpec{dir: dir, stackName: s.Stack})
		}
	} else {
		// Ad-hoc directories from positional args.
		for _, dir := range args.Dirs {
			absDir, err := filepath.Abs(dir)
			if err != nil {
				return nil, fmt.Errorf("resolving directory %q: %w", dir, err)
			}
			name := args.StackName
			if name == "" {
				// No --stack flag; try to read the currently selected stack for this project.
				current, err := readCurrentStackForProject(absDir)
				if err != nil {
					return nil, fmt.Errorf(
						"no --stack specified and could not determine current stack for %q: %w", dir, err)
				}
				if current == "" {
					return nil, fmt.Errorf(
						"no --stack specified and no current stack selected for project %q; "+
							"use --stack to specify a stack name, or run `pulumi stack select` in %q first", dir, dir)
				}
				name = current
			}
			specs = append(specs, stackSpec{dir: absDir, stackName: name})
		}
	}

	// Resolve each stack.
	var entries []backend.MultistackEntry
	for _, spec := range specs {
		entry, err := resolveOneStack(ctx, spec.dir, spec.stackName, ws, lm, ssml, displayOpts)
		if err != nil {
			return nil, fmt.Errorf("resolving stack for %q: %w", spec.dir, err)
		}
		entries = append(entries, *entry)
	}

	return entries, nil
}

// resolveOneStack loads a project from a directory, connects to the backend, and resolves the stack.
func resolveOneStack(
	ctx context.Context,
	dir string,
	stackName string,
	ws pkgWorkspace.Context,
	lm cmdBackend.LoginManager,
	ssml cmdStack.SecretsManagerLoader,
	displayOpts display.Options,
) (*backend.MultistackEntry, error) {
	// Verify directory exists and contains a Pulumi.yaml.
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("directory %q: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%q is not a directory", dir)
	}

	// Load the project from the directory.
	projPath := filepath.Join(dir, "Pulumi.yaml")
	proj, err := workspace.LoadProject(projPath)
	if err != nil {
		return nil, fmt.Errorf("loading project from %q: %w", dir, err)
	}

	// Get the backend for this project.
	b, err := cmdBackend.CurrentBackend(ctx, ws, lm, proj, displayOpts)
	if err != nil {
		return nil, fmt.Errorf("getting backend for %q: %w", dir, err)
	}

	// Parse the stack reference.
	if stackName == "" {
		return nil, fmt.Errorf("no stack name specified for project %q", dir)
	}
	stackRef, err := b.ParseStackReference(stackName)
	if err != nil {
		return nil, fmt.Errorf("parsing stack reference %q for %q: %w", stackName, dir, err)
	}

	// Get the stack.
	s, err := b.GetStack(ctx, stackRef)
	if err != nil {
		return nil, fmt.Errorf("getting stack %q for %q: %w", stackName, dir, err)
	}
	if s == nil {
		return nil, fmt.Errorf("stack %q not found for project %q", stackName, dir)
	}

	// Load stack configuration directly from the project directory (not CWD).
	configPath := workspace.ProjectStackPath(proj, projPath, s.Ref().Name().Q())
	workspaceStack, err := workspace.LoadProjectStack(cmdutil.Diag(), proj, configPath)
	if err != nil {
		return nil, fmt.Errorf("loading config for %q/%q: %w", dir, stackName, err)
	}
	sm, state, err := ssml.GetSecretsManager(ctx, s, workspaceStack)
	if err != nil {
		return nil, fmt.Errorf("getting secrets manager for %q/%q: %w", dir, stackName, err)
	}
	if state != cmdStack.SecretsManagerUnchanged {
		if saveErr := cmdStack.SaveProjectStack(ctx, s, workspaceStack); saveErr != nil && state == cmdStack.SecretsManagerMustSave {
			return nil, fmt.Errorf("saving stack config for %q/%q: %w", dir, stackName, saveErr)
		}
	}

	decrypter := sm.Decrypter()
	cfg := backend.StackConfiguration{
		EnvironmentImports: workspaceStack.Environment.Imports(),
		Config:             workspaceStack.Config,
		Decrypter:          decrypter,
	}

	// Build UpdateOperation.
	m := &backend.UpdateMetadata{
		Environment: make(map[string]string),
	}
	op := backend.UpdateOperation{
		Proj: proj,
		Root: dir,
		M:    m,
		Opts: backend.UpdateOptions{
			Display: displayOpts,
		},
		StackConfiguration: cfg,
		SecretsManager:     sm,
		SecretsProvider:    secrets.DefaultProvider,
		Scopes:             backend.CancellationScopes,
	}

	return &backend.MultistackEntry{
		Stack: s,
		Op:    op,
		Dir:   dir,
	}, nil
}

// PrintMultistackConfirmation prints a stylized confirmation header for a multistack operation.
// The format mirrors single-stack headers (e.g., "Previewing update (stack)") but lists multiple stacks.
func PrintMultistackConfirmation(entries []backend.MultistackEntry, operation string) {
	c := cmdutil.GetGlobalColorization()

	fmt.Print(c.Colorize(
		colors.SpecHeadline + fmt.Sprintf("%s (%d stacks)", operation, len(entries)) + colors.Reset + "\n"))

	// Compute max directory width for alignment.
	maxDirLen := 0
	type entryInfo struct {
		dir string
		ref string
	}
	infos := make([]entryInfo, len(entries))
	for i, entry := range entries {
		dir := entry.Dir
		if cwd, err := os.Getwd(); err == nil {
			if relDir, err := filepath.Rel(cwd, dir); err == nil {
				dir = relDir
			}
		}
		ref := string(entry.Stack.Ref().FullyQualifiedName())
		infos[i] = entryInfo{dir: dir, ref: ref}
		if len(dir) > maxDirLen {
			maxDirLen = len(dir)
		}
	}

	for _, info := range infos {
		line := fmt.Sprintf("    %-*s  %s  %s",
			maxDirLen,
			c.Colorize(colors.Bold+info.dir+colors.Reset),
			c.Colorize(colors.SpecUnimportant+"→"+colors.Reset),
			c.Colorize(colors.BrightCyan+info.ref+colors.Reset),
		)
		fmt.Println(line)
	}
	fmt.Println()
}

// checkMultistackErrors checks if any stack in the results had errors and returns
// an appropriate error. The unified display already renders resource changes and
// summary, so this only surfaces per-stack failures.
func checkMultistackErrors(results map[string]*backend.MultistackResult, operation string) error {
	// Collect unique errors (deduplicate identical errors from unified engine path).
	var errors []error
	seen := make(map[string]bool)
	for _, result := range results {
		if result != nil && result.Error != nil {
			msg := result.Error.Error()
			if !seen[msg] {
				seen[msg] = true
				errors = append(errors, result.Error)
			}
		}
	}
	if len(errors) == 0 {
		return nil
	}
	// Single error: return it directly (matches normal single-stack error behavior).
	if len(errors) == 1 {
		return errors[0]
	}
	// Multiple distinct errors: list them.
	var failedDetails []string
	for key, result := range results {
		if result != nil && result.Error != nil {
			failedDetails = append(failedDetails, fmt.Sprintf("  %s: %v", key, result.Error))
		}
	}
	sort.Strings(failedDetails)
	return fmt.Errorf("one or more stacks failed during %s:\n%s",
		operation, strings.Join(failedDetails, "\n"))
}

// PrintDownstreamWarnings prints warnings about downstream stacks that consume outputs
// from the given stack via StackReferences. This is used during single-stack preview.
func PrintDownstreamWarnings(
	ctx context.Context,
	b backend.Backend,
	stackRef backend.StackReference,
) {
	// Check if the backend supports downstream reference lookup.
	lister, ok := b.(backend.DownstreamReferenceLister)
	if !ok {
		return
	}

	refs, err := lister.GetDownstreamReferences(ctx, stackRef)
	if err != nil {
		logging.V(4).Infof("multistack: failed to get downstream references: %v", err)
		return
	}

	if len(refs) == 0 {
		return
	}

	// Sort for deterministic output.
	sort.Slice(refs, func(i, j int) bool {
		ri := fmt.Sprintf("%s/%s/%s", refs[i].OrgName, refs[i].ProjectName, refs[i].StackName)
		rj := fmt.Sprintf("%s/%s/%s", refs[j].OrgName, refs[j].ProjectName, refs[j].StackName)
		return ri < rj
	})

	fmt.Fprintf(os.Stderr, "\nWarning: Downstream stacks consume outputs from this stack:\n")
	for _, ref := range refs {
		fmt.Fprintf(os.Stderr, "  - %s/%s/%s (via StackReference)\n",
			ref.OrgName, ref.ProjectName, ref.StackName)
	}

	// Suggest multistack preview.
	dirs := collectProjectDirs(refs)
	if len(dirs) > 0 {
		fmt.Fprintf(os.Stderr, "Consider running: pulumi preview %s\n\n",
			strings.Join(dirs, " "))
	}
}

// collectProjectDirs attempts to suggest directory names from downstream references.
// This is a best-effort heuristic based on project names.
func collectProjectDirs(refs []backend.DownstreamStackReference) []string {
	var dirs []string
	seen := make(map[string]bool)
	for _, ref := range refs {
		dir := "./" + ref.ProjectName
		if !seen[dir] {
			seen[dir] = true
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

// readCurrentStackForProject reads the currently selected stack name for a project in
// the given directory by looking at its workspace settings file. Returns "" if no stack
// is currently selected.
func readCurrentStackForProject(dir string) (string, error) {
	// Check PULUMI_STACK env var first, same as the normal workspace code path.
	if stackName, ok := os.LookupEnv("PULUMI_STACK"); ok {
		return stackName, nil
	}

	// Load the project to get its name and path.
	projPath := filepath.Join(dir, "Pulumi.yaml")
	proj, err := workspace.LoadProject(projPath)
	if err != nil {
		return "", fmt.Errorf("loading project from %q: %w", dir, err)
	}

	// Compute the workspace settings path: ~/.pulumi/workspaces/<name>-<sha1(projPath)>-workspace.json
	absProjPath, err := filepath.Abs(projPath)
	if err != nil {
		return "", err
	}
	//nolint:gosec
	h := sha1.New()
	h.Write([]byte(absProjPath))
	hash := hex.EncodeToString(h.Sum(nil))
	settingsFile := string(proj.Name) + "-" + hash + "-" + workspace.WorkspaceFile
	settingsPath, err := workspace.GetPulumiPath(workspace.WorkspaceDir, settingsFile)
	if err != nil {
		return "", err
	}

	// Read and parse the settings file.
	b, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // no settings file means no stack selected
		}
		return "", err
	}

	var settings struct {
		Stack string `json:"stack"`
	}
	if err := json.Unmarshal(b, &settings); err != nil {
		return "", fmt.Errorf("parsing workspace settings %q: %w", settingsPath, err)
	}

	return settings.Stack, nil
}

// confirmMultistackOperation prompts the user whether to proceed with a multistack operation.
// Supports yes/no/details, matching the single-stack confirmation flow.
func confirmMultistackOperation(
	kind string,
	previewResults map[string]*backend.MultistackResult,
	entries []backend.MultistackEntry,
	displayOpts display.Options,
) error {
	// Collect all preview events across stacks for the "details" view.
	var allEvents []engine.Event
	for _, entry := range entries {
		key := string(entry.Stack.Ref().FullyQualifiedName())
		if r := previewResults[key]; r != nil {
			allEvents = append(allEvents, r.Events...)
		}
	}

	for {
		var response string

		surveycore.DisableColor = true
		surveyIcons := survey.WithIcons(func(icons *survey.IconSet) {
			icons.Question = survey.Icon{}
			icons.SelectFocus = survey.Icon{
				Text: displayOpts.Color.Colorize(colors.BrightGreen + ">" + colors.Reset),
			}
		})

		prompt := "\b" + displayOpts.Color.Colorize(
			colors.SpecPrompt+fmt.Sprintf("Do you want to perform this %s?", kind)+colors.Reset)

		if err := survey.AskOne(&survey.Select{
			Message: prompt,
			Options: []string{"yes", "no", "details"},
			Default: "no",
		}, &response, surveyIcons); err != nil {
			return fmt.Errorf("confirmation cancelled, not proceeding with the %s: %w", kind, err)
		}

		if response == "no" {
			return result.FprintBailf(os.Stdout, "confirmation declined, not proceeding with the %s", kind)
		}

		if response == "yes" {
			return nil
		}

		if response == "details" {
			diff, err := display.CreateDiff(allEvents, displayOpts)
			if err != nil {
				return err
			}
			fmt.Println(diff)
			continue
		}
	}
}
