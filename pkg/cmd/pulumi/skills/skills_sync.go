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

package skills

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/blang/semver"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/gitutil"
)

const (
	skillsRepoURL = "https://github.com/pulumi/agent-skills.git"
	skillsBranch  = "main"

	metadataPath = ".claude-plugin/plugin.json"
)

// TargetLocation represents a detected AI assistant config directory.
type TargetLocation struct {
	Type string
	Path string
}

// SkillMetadata holds the name and version read from a skill package's metadata file.
type SkillMetadata struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// SyncResult is the JSON output structure.
type SyncResult struct {
	Targets        []TargetResult `json:"targets"`
	SkillsSynced   int            `json:"skillsSynced"`
	SkillsUpToDate int            `json:"skillsUpToDate"`
	Errors         []string       `json:"errors,omitempty"`
}

// TargetResult represents the sync outcome for a single target/skill pair.
type TargetResult struct {
	Path      string `json:"path"`
	Type      string `json:"type"`
	SkillName string `json:"skillName"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

type skillsSyncCmd struct {
	jsonOutput bool
	force      bool

	cloneFunc func(ctx context.Context) (string, error)
}

func newSyncCmd() *cobra.Command {
	var cmd skillsSyncCmd
	cobraCmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync Pulumi agent skills to AI assistant config directories",
		Long: "Sync Pulumi agent skills from github.com/pulumi/agent-skills to AI assistant\n" +
			"configuration directories in the current working directory.\n" +
			"\n" +
			"This command detects AI assistant configuration directories (.claude/, .cursor/,\n" +
			".windsurf/, .github/, .agents/) and installs Pulumi skills into each one.\n" +
			"Skills are only updated when a newer version is available unless --force is used.\n" +
			"\n" +
			"Supported AI assistants:\n" +
			"  - Claude Code (.claude/)\n" +
			"  - Cursor (.cursor/)\n" +
			"  - Windsurf (.windsurf/)\n" +
			"  - GitHub Copilot (.github/skills/)\n" +
			"  - Codex (.agents/)",
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			return cmd.Run(cobraCmd.Context())
		},
	}

	constrictor.AttachArguments(cobraCmd, constrictor.NoArgs)

	cobraCmd.PersistentFlags().BoolVarP(&cmd.force, "force", "f", false,
		"Force sync even if skills are up to date")
	cobraCmd.PersistentFlags().BoolVar(&cmd.jsonOutput, "json", false,
		"Emit output as JSON")

	return cobraCmd
}

// Run executes the sync command using the current working directory.
func (cmd *skillsSyncCmd) Run(ctx context.Context) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}
	return cmd.runInDir(ctx, cwd)
}

func (cmd *skillsSyncCmd) getCloneFunc() func(ctx context.Context) (string, error) {
	if cmd.cloneFunc != nil {
		return cmd.cloneFunc
	}
	return cloneSkillsRepo
}

func (cmd *skillsSyncCmd) runInDir(ctx context.Context, dir string) error {
	targets, err := detectTargetLocations(dir)
	if err != nil {
		return fmt.Errorf("detecting target locations: %w", err)
	}
	if len(targets) == 0 {
		return fmt.Errorf("no AI assistant configuration directories found in %s\n\n"+
			"Supported configurations:\n"+
			"  - .claude/                           (Claude Code)\n"+
			"  - .cursor/                           (Cursor)\n"+
			"  - .windsurf/                         (Windsurf)\n"+
			"  - .github/skills/                    (GitHub Copilot)\n"+
			"  - .agents/                           (Codex)\n\n"+
			"Create one of these directories and try again.", dir)
	}

	if !cmd.jsonOutput {
		fmt.Fprintf(os.Stdout, "Detected %d AI assistant configuration(s):\n", len(targets))
		for _, t := range targets {
			fmt.Fprintf(os.Stdout, "  - %s (%s)\n", t.Type, t.Path)
		}
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "Fetching skills from github.com/pulumi/agent-skills...")
	}

	repoDir, err := cmd.getCloneFunc()(ctx)
	if err != nil {
		return err
	}
	defer os.RemoveAll(repoDir)

	skillDirs, err := discoverSkillPackages(repoDir)
	if err != nil {
		return fmt.Errorf("discovering skills: %w", err)
	}
	if len(skillDirs) == 0 {
		return errors.New("no skills found in repository")
	}

	result := SyncResult{}
	for _, skillDirName := range skillDirs {
		skillPath := filepath.Join(repoDir, skillDirName)

		remoteMeta, err := parseSkillMetadata(skillPath)
		if err != nil {
			errMsg := fmt.Sprintf("reading metadata for %s: %s", skillDirName, err.Error())
			result.Errors = append(result.Errors, errMsg)
			if !cmd.jsonOutput {
				fmt.Fprintf(os.Stderr, "Warning: %s\n", errMsg)
			}
			continue
		}

		for _, target := range targets {
			targetResult := TargetResult{
				Path:      target.Path,
				Type:      target.Type,
				SkillName: remoteMeta.Name,
			}

			destDir := filepath.Join(target.Path, remoteMeta.Name)
			localMeta, _ := parseSkillMetadata(destDir)

			if !shouldUpdate(localMeta, remoteMeta, cmd.force) {
				targetResult.Status = "up-to-date"
				result.SkillsUpToDate++
				result.Targets = append(result.Targets, targetResult)
				if !cmd.jsonOutput {
					fmt.Fprintf(os.Stdout, "  %s/%s: up to date (v%s)\n",
						target.Type, remoteMeta.Name, remoteMeta.Version)
				}
				continue
			}

			if localMeta != nil {
				if err := os.RemoveAll(destDir); err != nil {
					targetResult.Status = "error"
					targetResult.Error = err.Error()
					result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", destDir, err.Error()))
					result.Targets = append(result.Targets, targetResult)
					continue
				}
			}
			if err := os.MkdirAll(destDir, 0o755); err != nil {
				targetResult.Status = "error"
				targetResult.Error = err.Error()
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", destDir, err.Error()))
				result.Targets = append(result.Targets, targetResult)
				continue
			}

			if err := copyDirectory(skillPath, destDir); err != nil {
				targetResult.Status = "error"
				targetResult.Error = err.Error()
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", destDir, err.Error()))
				result.Targets = append(result.Targets, targetResult)
				continue
			}

			targetResult.Status = "synced"
			result.SkillsSynced++
			result.Targets = append(result.Targets, targetResult)
			if !cmd.jsonOutput {
				action := "installed"
				if localMeta != nil {
					action = "updated from v" + localMeta.Version
				}
				fmt.Fprintf(os.Stdout, "  %s/%s: %s (v%s)\n",
					target.Type, remoteMeta.Name, action, remoteMeta.Version)
			}
		}
	}

	if cmd.jsonOutput {
		return ui.PrintJSON(result)
	}

	fmt.Fprintln(os.Stdout)
	if result.SkillsSynced > 0 {
		fmt.Fprintf(os.Stdout, "Synced %d skill(s).\n", result.SkillsSynced)
	}
	if result.SkillsUpToDate > 0 {
		fmt.Fprintf(os.Stdout, "%d skill(s) already up to date.\n", result.SkillsUpToDate)
	}
	if len(result.Errors) > 0 {
		return fmt.Errorf("completed with %d error(s)", len(result.Errors))
	}

	return nil
}

func detectTargetLocations(cwd string) ([]TargetLocation, error) {
	var targets []TargetLocation

	if info, err := os.Stat(filepath.Join(cwd, ".claude")); err == nil && info.IsDir() {
		targets = append(targets, TargetLocation{
			Type: "claude",
			Path: filepath.Join(cwd, ".claude", "skills"),
		})
	}

	if info, err := os.Stat(filepath.Join(cwd, ".cursor")); err == nil && info.IsDir() {
		targets = append(targets, TargetLocation{
			Type: "cursor",
			Path: filepath.Join(cwd, ".cursor", "skills"),
		})
	}

	if info, err := os.Stat(filepath.Join(cwd, ".windsurf")); err == nil && info.IsDir() {
		targets = append(targets, TargetLocation{
			Type: "windsurf",
			Path: filepath.Join(cwd, ".windsurf", "skills"),
		})
	}

	if info, err := os.Stat(filepath.Join(cwd, ".github", "skills")); err == nil && info.IsDir() {
		targets = append(targets, TargetLocation{
			Type: "copilot",
			Path: filepath.Join(cwd, ".github", "skills"),
		})
	}

	if info, err := os.Stat(filepath.Join(cwd, ".agents")); err == nil && info.IsDir() {
		targets = append(targets, TargetLocation{
			Type: "codex",
			Path: filepath.Join(cwd, ".agents", "skills"),
		})
	}

	return targets, nil
}

func discoverSkillPackages(repoDir string) ([]string, error) {
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return nil, err
	}
	var skills []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(repoDir, entry.Name(), metadataPath)); err == nil {
			skills = append(skills, entry.Name())
		}
	}
	return skills, nil
}

func cloneSkillsRepo(ctx context.Context) (string, error) {
	tempDir, err := os.MkdirTemp("", "pulumi-skills-")
	if err != nil {
		return "", fmt.Errorf("creating temp directory: %w", err)
	}

	branch := plumbing.NewBranchReferenceName(skillsBranch)

	if err := gitutil.GitCloneOrPull(ctx, skillsRepoURL, branch, tempDir, true); err != nil {
		os.RemoveAll(tempDir)
		return "", fmt.Errorf("cloning skills repository: %w", err)
	}

	return tempDir, nil
}

func parseSkillMetadata(skillDir string) (*SkillMetadata, error) {
	path := filepath.Join(skillDir, metadataPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading skill metadata: %w", err)
	}

	var meta SkillMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parsing skill metadata: %w", err)
	}

	if meta.Version == "" {
		return nil, errors.New("skill metadata missing version field")
	}
	if meta.Name == "" {
		return nil, errors.New("skill metadata missing name field")
	}

	return &meta, nil
}

func shouldUpdate(local, remote *SkillMetadata, force bool) bool {
	if force {
		return true
	}
	if local == nil {
		return true
	}
	if remote == nil {
		return false
	}

	localVer, err := semver.ParseTolerant(local.Version)
	if err != nil {
		return true
	}

	remoteVer, err := semver.ParseTolerant(remote.Version)
	if err != nil {
		return false
	}

	return remoteVer.GT(localVer)
}

func copyDirectory(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if d.IsDir() {
			return os.MkdirAll(dstPath, 0o755)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		return os.WriteFile(dstPath, content, info.Mode().Perm()|0o644)
	})
}
