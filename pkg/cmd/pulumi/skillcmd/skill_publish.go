// Copyright 2025, Pulumi Corporation.
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

package skillcmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type publishSkillArgs struct {
	publisher string
	version   string
	name      string
}

type skillPublishCmd struct {
	defaultOrg func(context.Context, backend.Backend, *workspace.Project) (string, error)
}

// skillMetadata represents the YAML frontmatter in SKILL.md
type skillMetadata struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func newSkillPublishCmd() *cobra.Command {
	var args publishSkillArgs

	cmd := &cobra.Command{
		Use:   "publish <directory>",
		Args:  cmdutil.ExactArgs(1),
		Short: "Publish a Pulumi Neo skill to the Private Registry",
		Long: "Publish a Pulumi Neo skill to the Private Registry.\n\n" +
			"This command publishes a skill directory containing a SKILL.md file to the Private Registry.",
		RunE: func(cmd *cobra.Command, cliArgs []string) error {
			ctx := cmd.Context()
			skillPublishCmd := skillPublishCmd{defaultOrg: backend.GetDefaultOrg}
			return skillPublishCmd.Run(ctx, cmd, args, cliArgs[0])
		},
	}

	cmd.Flags().StringVar(
		&args.version, "version", "",
		"The version of the skill (required, semver format)")
	cmd.Flags().StringVar(
		&args.name, "name", "",
		"The name of the skill (defaults to name from SKILL.md frontmatter)")
	cmd.Flags().StringVar(
		&args.publisher, "publisher", "",
		"The publisher of the skill (e.g., 'pulumi'). Defaults to the default organization in your pulumi config.")
	contract.AssertNoErrorf(cmd.MarkFlagRequired("version"), "Could not mark \"version\" as required")

	return cmd
}

func (skillCmd *skillPublishCmd) Run(
	ctx context.Context,
	cmd *cobra.Command,
	args publishSkillArgs,
	skillDir string,
) error {
	if _, err := os.Stat(skillDir); os.IsNotExist(err) {
		return fmt.Errorf("skill directory does not exist: %s", skillDir)
	}

	absSkillDir, err := filepath.Abs(skillDir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path for skill directory: %w", err)
	}

	// Validate SKILL.md exists and extract metadata
	skillMDPath := filepath.Join(absSkillDir, "SKILL.md")
	if _, err := os.Stat(skillMDPath); os.IsNotExist(err) {
		return fmt.Errorf("SKILL.md not found in directory: %s", skillDir)
	}

	metadata, err := extractSkillMetadata(skillMDPath)
	if err != nil {
		return fmt.Errorf("failed to extract skill metadata from SKILL.md: %w", err)
	}

	// Use name from SKILL.md if not provided via flag
	skillName := args.name
	if skillName == "" {
		if metadata.Name == "" {
			return errors.New("skill name not specified and not found in SKILL.md frontmatter")
		}
		skillName = metadata.Name
	}

	version, err := semver.ParseTolerant(args.version)
	if err != nil {
		return fmt.Errorf("invalid version format: %w", err)
	}

	project, _, err := pkgWorkspace.Instance.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return fmt.Errorf("failed to determine current project: %w", err)
	}

	b, err := cmdBackend.CurrentBackend(
		ctx, pkgWorkspace.Instance, cmdBackend.DefaultLoginManager, project,
		display.Options{Color: cmdutil.GetGlobalColorization()})
	if err != nil {
		return err
	}

	_, err = b.GetCloudRegistry()
	if err != nil {
		return fmt.Errorf("backend does not support Private Registry operations: %w", err)
	}

	var publisher string
	if args.publisher != "" {
		publisher = args.publisher
	} else {
		publisher, err = skillCmd.defaultOrg(ctx, b, project)
		if err != nil {
			return fmt.Errorf("failed to determine default organization: %w", err)
		}
		if publisher == "" {
			return errors.New(
				"no publisher specified and no default organization found, " +
					"please set a publisher or set a default organization in your pulumi config",
			)
		}
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "Creating archive from directory: %s\n", skillDir)
	archiveBytes, err := archive.TGZ(absSkillDir, "", true /*useDefaultExcludes*/)
	if err != nil {
		return fmt.Errorf("failed to create archive: %w", err)
	}
	archiveData := bytes.NewBuffer(archiveBytes)

	if err := tokens.ValidateProjectName(skillName); err != nil {
		return fmt.Errorf("invalid skill name: %w", err)
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "Publishing skill %s/%s@%s...\n", publisher, skillName, version.String())
	err = skillCmd.publishSkill(ctx, b, publisher, skillName, version, archiveData)
	if err != nil {
		return fmt.Errorf("failed to publish skill: %w", err)
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "Successfully published skill %s/%s@%s\n", publisher, skillName, version.String())
	return nil
}

func (skillCmd *skillPublishCmd) publishSkill(
	ctx context.Context,
	b backend.Backend,
	publisher, name string,
	version semver.Version,
	archiveData *bytes.Buffer,
) error {
	registry, err := b.GetCloudRegistry()
	if err != nil {
		return fmt.Errorf("failed to get the Private Registry backend: %w", err)
	}

	publishInput := apitype.SkillPublishOp{
		Source:    "private",
		Publisher: publisher,
		Name:      name,
		Version:   version,
		Archive:   archiveData,
	}

	err = registry.PublishSkill(ctx, publishInput)
	if err != nil {
		return fmt.Errorf("failed to publish skill: %w", err)
	}

	return nil
}

// extractSkillMetadata reads the SKILL.md file and extracts YAML frontmatter
func extractSkillMetadata(skillMDPath string) (*skillMetadata, error) {
	content, err := os.ReadFile(skillMDPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read SKILL.md: %w", err)
	}

	// Parse YAML frontmatter (delimited by --- at start and end)
	contentStr := string(content)
	if !strings.HasPrefix(contentStr, "---\n") {
		return nil, errors.New("SKILL.md must start with YAML frontmatter (---)")
	}

	// Find the closing ---
	endIndex := strings.Index(contentStr[4:], "\n---\n")
	if endIndex == -1 {
		return nil, errors.New("SKILL.md frontmatter must be closed with ---")
	}

	frontmatter := contentStr[4 : 4+endIndex]

	var metadata skillMetadata
	if err := yaml.Unmarshal([]byte(frontmatter), &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse YAML frontmatter: %w", err)
	}

	return &metadata, nil
}
