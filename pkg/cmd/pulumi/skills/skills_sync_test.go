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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createFakeRepo(t *testing.T, skills map[string]SkillMetadata) string {
	t.Helper()
	repoDir := t.TempDir()
	for dirName, meta := range skills {
		skillDir := filepath.Join(repoDir, dirName)
		require.NoError(t, os.MkdirAll(filepath.Join(skillDir, ".claude-plugin"), 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(skillDir, "skills", "example-skill"), 0o755))

		metaJSON := `{"name": "` + meta.Name + `", "version": "` + meta.Version + `"}`
		require.NoError(t, os.WriteFile(
			filepath.Join(skillDir, ".claude-plugin", "plugin.json"),
			[]byte(metaJSON), 0o600))
		require.NoError(t, os.WriteFile(
			filepath.Join(skillDir, "skills", "example-skill", "SKILL.md"),
			[]byte("# "+meta.Name+"\nversion "+meta.Version), 0o600))
	}
	return repoDir
}

func fakeCloneFunc(t *testing.T, repoDir string) func(ctx context.Context) (string, error) {
	t.Helper()
	return func(ctx context.Context) (string, error) {
		tmpDir := t.TempDir()
		require.NoError(t, copyDirectory(repoDir, tmpDir))
		return tmpDir, nil
	}
}

func TestDetectTargetLocations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func(t *testing.T, dir string)
		expected []TargetLocation
	}{
		{
			name: "claude directory",
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.Mkdir(filepath.Join(dir, ".claude"), 0o755))
			},
			expected: []TargetLocation{
				{Type: "claude", Path: ".claude/skills"},
			},
		},
		{
			name: "cursor directory",
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.Mkdir(filepath.Join(dir, ".cursor"), 0o755))
			},
			expected: []TargetLocation{
				{Type: "cursor", Path: ".cursor/skills"},
			},
		},
		{
			name: "windsurf directory",
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.Mkdir(filepath.Join(dir, ".windsurf"), 0o755))
			},
			expected: []TargetLocation{
				{Type: "windsurf", Path: ".windsurf/skills"},
			},
		},
		{
			name: "copilot skills directory",
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.MkdirAll(filepath.Join(dir, ".github", "skills"), 0o755))
			},
			expected: []TargetLocation{
				{Type: "copilot", Path: ".github/skills"},
			},
		},
		{
			name: "codex agents directory",
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.Mkdir(filepath.Join(dir, ".agents"), 0o755))
			},
			expected: []TargetLocation{
				{Type: "codex", Path: ".agents/skills"},
			},
		},
		{
			name: "multiple targets",
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.Mkdir(filepath.Join(dir, ".claude"), 0o755))
				require.NoError(t, os.Mkdir(filepath.Join(dir, ".cursor"), 0o755))
			},
			expected: []TargetLocation{
				{Type: "claude", Path: ".claude/skills"},
				{Type: "cursor", Path: ".cursor/skills"},
			},
		},
		{
			name: "all targets",
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.Mkdir(filepath.Join(dir, ".claude"), 0o755))
				require.NoError(t, os.Mkdir(filepath.Join(dir, ".cursor"), 0o755))
				require.NoError(t, os.Mkdir(filepath.Join(dir, ".windsurf"), 0o755))
				require.NoError(t, os.MkdirAll(filepath.Join(dir, ".github", "skills"), 0o755))
				require.NoError(t, os.Mkdir(filepath.Join(dir, ".agents"), 0o755))
			},
			expected: []TargetLocation{
				{Type: "claude", Path: ".claude/skills"},
				{Type: "cursor", Path: ".cursor/skills"},
				{Type: "windsurf", Path: ".windsurf/skills"},
				{Type: "copilot", Path: ".github/skills"},
				{Type: "codex", Path: ".agents/skills"},
			},
		},
		{
			name:     "empty directory",
			setup:    func(t *testing.T, dir string) {},
			expected: nil,
		},
		{
			name: "unrelated files only",
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte{}, 0o600))
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			tt.setup(t, dir)

			targets, err := detectTargetLocations(dir)
			require.NoError(t, err)

			if tt.expected == nil {
				assert.Empty(t, targets)
				return
			}

			require.Len(t, targets, len(tt.expected))
			for i, expected := range tt.expected {
				assert.Equal(t, expected.Type, targets[i].Type)
				relPath, err := filepath.Rel(dir, targets[i].Path)
				require.NoError(t, err)
				assert.Equal(t, expected.Path, relPath)
			}
		})
	}
}

func TestShouldUpdate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		local    *SkillMetadata
		remote   *SkillMetadata
		force    bool
		expected bool
	}{
		{
			name:     "remote newer",
			local:    &SkillMetadata{Version: "1.0.0"},
			remote:   &SkillMetadata{Version: "1.0.1"},
			expected: true,
		},
		{
			name:     "local newer",
			local:    &SkillMetadata{Version: "1.0.1"},
			remote:   &SkillMetadata{Version: "1.0.0"},
			expected: false,
		},
		{
			name:     "same version",
			local:    &SkillMetadata{Version: "1.0.0"},
			remote:   &SkillMetadata{Version: "1.0.0"},
			expected: false,
		},
		{
			name:     "major version bump",
			local:    &SkillMetadata{Version: "1.9.9"},
			remote:   &SkillMetadata{Version: "2.0.0"},
			expected: true,
		},
		{
			name:     "not installed",
			local:    nil,
			remote:   &SkillMetadata{Version: "1.0.0"},
			expected: true,
		},
		{
			name:     "force flag",
			local:    &SkillMetadata{Version: "1.0.0"},
			remote:   &SkillMetadata{Version: "1.0.0"},
			force:    true,
			expected: true,
		},
		{
			name:     "force with local newer",
			local:    &SkillMetadata{Version: "2.0.0"},
			remote:   &SkillMetadata{Version: "1.0.0"},
			force:    true,
			expected: true,
		},
		{
			name:     "invalid local version",
			local:    &SkillMetadata{Version: "invalid"},
			remote:   &SkillMetadata{Version: "1.0.0"},
			expected: true,
		},
		{
			name:     "invalid remote version",
			local:    &SkillMetadata{Version: "1.0.0"},
			remote:   &SkillMetadata{Version: "invalid"},
			expected: false,
		},
		{
			name:     "nil remote",
			local:    &SkillMetadata{Version: "1.0.0"},
			remote:   nil,
			expected: false,
		},
		{
			name:     "both nil",
			local:    nil,
			remote:   nil,
			expected: true, // nil local means not installed, should try to install
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := shouldUpdate(tt.local, tt.remote, tt.force)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseSkillMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		content     string
		expected    *SkillMetadata
		expectError bool
	}{
		{
			name:     "valid metadata",
			content:  `{"name": "pulumi-migration", "version": "1.0.0"}`,
			expected: &SkillMetadata{Name: "pulumi-migration", Version: "1.0.0"},
		},
		{
			name:     "valid metadata with extra fields",
			content:  `{"name": "pulumi-authoring", "version": "2.1.0", "description": "test"}`,
			expected: &SkillMetadata{Name: "pulumi-authoring", Version: "2.1.0"},
		},
		{
			name:        "missing version",
			content:     `{"name": "pulumi-migration"}`,
			expectError: true,
		},
		{
			name:        "missing name",
			content:     `{"version": "1.0.0"}`,
			expectError: true,
		},
		{
			name:        "invalid json",
			content:     `{invalid}`,
			expectError: true,
		},
		{
			name:        "empty file",
			content:     ``,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			metaDir := filepath.Join(dir, ".claude-plugin")
			require.NoError(t, os.MkdirAll(metaDir, 0o755))
			require.NoError(t, os.WriteFile(
				filepath.Join(metaDir, "plugin.json"), []byte(tt.content), 0o600))

			metadata, err := parseSkillMetadata(dir)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected.Name, metadata.Name)
				assert.Equal(t, tt.expected.Version, metadata.Version)
			}
		})
	}
}

func TestParseSkillMetadataNoFile(t *testing.T) {
	t.Parallel()
	_, err := parseSkillMetadata(t.TempDir())
	assert.Error(t, err)
}

func TestDiscoverSkillPackages(t *testing.T) {
	t.Parallel()

	t.Run("finds skill packages with metadata", func(t *testing.T) {
		t.Parallel()
		repoDir := createFakeRepo(t, map[string]SkillMetadata{
			"migration": {Name: "pulumi-migration", Version: "1.0.0"},
			"authoring": {Name: "pulumi-authoring", Version: "1.0.0"},
		})

		packages, err := discoverSkillPackages(repoDir)
		require.NoError(t, err)
		require.Len(t, packages, 2)
		assert.Contains(t, packages, "migration")
		assert.Contains(t, packages, "authoring")
	})

	t.Run("ignores directories without metadata", func(t *testing.T) {
		t.Parallel()
		repoDir := createFakeRepo(t, map[string]SkillMetadata{
			"migration": {Name: "pulumi-migration", Version: "1.0.0"},
		})
		require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "docs"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("hi"), 0o600))

		packages, err := discoverSkillPackages(repoDir)
		require.NoError(t, err)
		assert.Equal(t, []string{"migration"}, packages)
	})

	t.Run("returns empty for repo with no skills", func(t *testing.T) {
		t.Parallel()
		repoDir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "docs"), 0o755))

		packages, err := discoverSkillPackages(repoDir)
		require.NoError(t, err)
		assert.Empty(t, packages)
	})
}

func TestCopyDirectory(t *testing.T) {
	t.Parallel()

	t.Run("copies all files preserving structure", func(t *testing.T) {
		t.Parallel()

		src := t.TempDir()
		dst := t.TempDir()

		require.NoError(t, os.MkdirAll(filepath.Join(src, ".claude-plugin"), 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(src, ".claude-plugin", "plugin.json"),
			[]byte(`{"name": "test", "version": "1.0.0"}`), 0o600))
		require.NoError(t, os.MkdirAll(filepath.Join(src, "skills", "test-skill"), 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(src, "skills", "test-skill", "SKILL.md"),
			[]byte("# Test Skill"), 0o600))

		require.NoError(t, copyDirectory(src, dst))

		content, err := os.ReadFile(filepath.Join(dst, ".claude-plugin", "plugin.json"))
		require.NoError(t, err)
		assert.Contains(t, string(content), `"test"`)

		content, err = os.ReadFile(filepath.Join(dst, "skills", "test-skill", "SKILL.md"))
		require.NoError(t, err)
		assert.Equal(t, "# Test Skill", string(content))
	})

	t.Run("overwrites existing files", func(t *testing.T) {
		t.Parallel()

		src := t.TempDir()
		dst := t.TempDir()

		require.NoError(t, os.WriteFile(filepath.Join(src, "file.txt"), []byte("new"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(dst, "file.txt"), []byte("old"), 0o600))

		require.NoError(t, copyDirectory(src, dst))

		content, err := os.ReadFile(filepath.Join(dst, "file.txt"))
		require.NoError(t, err)
		assert.Equal(t, "new", string(content))
	})

	t.Run("skips .git directory", func(t *testing.T) {
		t.Parallel()

		src := t.TempDir()
		dst := t.TempDir()

		require.NoError(t, os.MkdirAll(filepath.Join(src, ".git"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(src, ".git", "config"), []byte("git"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(src, "file.txt"), []byte("content"), 0o600))

		require.NoError(t, copyDirectory(src, dst))

		_, err := os.Stat(filepath.Join(dst, ".git"))
		assert.True(t, os.IsNotExist(err))

		_, err = os.Stat(filepath.Join(dst, "file.txt"))
		require.NoError(t, err)
	})
}

func TestSyncNoTargets(t *testing.T) {
	t.Parallel()

	cmd := &skillsSyncCmd{}
	err := cmd.runInDir(context.Background(), t.TempDir())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no AI assistant configuration directories found")
}

func TestSyncFreshInstall(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".claude"), 0o755))

	repoDir := createFakeRepo(t, map[string]SkillMetadata{
		"migration": {Name: "pulumi-migration", Version: "1.0.0"},
		"authoring": {Name: "pulumi-authoring", Version: "2.0.0"},
	})

	cmd := &skillsSyncCmd{cloneFunc: fakeCloneFunc(t, repoDir)}
	require.NoError(t, cmd.runInDir(context.Background(), dir))

	data, err := os.ReadFile(filepath.Join(dir, ".claude", "skills", "pulumi-migration",
		".claude-plugin", "plugin.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"version": "1.0.0"`)

	data, err = os.ReadFile(filepath.Join(dir, ".claude", "skills", "pulumi-migration",
		"skills", "example-skill", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "pulumi-migration")

	data, err = os.ReadFile(filepath.Join(dir, ".claude", "skills", "pulumi-authoring",
		".claude-plugin", "plugin.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"version": "2.0.0"`)
}

func TestSyncMultipleTargets(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".claude"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".cursor"), 0o755))

	repoDir := createFakeRepo(t, map[string]SkillMetadata{
		"migration": {Name: "pulumi-migration", Version: "1.0.0"},
	})

	cmd := &skillsSyncCmd{cloneFunc: fakeCloneFunc(t, repoDir)}
	require.NoError(t, cmd.runInDir(context.Background(), dir))

	for _, target := range []string{".claude", ".cursor"} {
		data, err := os.ReadFile(filepath.Join(dir, target, "skills", "pulumi-migration",
			".claude-plugin", "plugin.json"))
		require.NoError(t, err, "reading metadata from %s", target)
		assert.Contains(t, string(data), `"pulumi-migration"`)
	}
}

func TestSyncSkipsUpToDate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".claude"), 0o755))

	repoDir := createFakeRepo(t, map[string]SkillMetadata{
		"migration": {Name: "pulumi-migration", Version: "1.0.0"},
	})

	cmd := &skillsSyncCmd{cloneFunc: fakeCloneFunc(t, repoDir)}
	require.NoError(t, cmd.runInDir(context.Background(), dir))

	skillFile := filepath.Join(dir, ".claude", "skills", "pulumi-migration",
		"skills", "example-skill", "SKILL.md")
	require.NoError(t, os.WriteFile(skillFile, []byte("TAMPERED"), 0o600))

	require.NoError(t, cmd.runInDir(context.Background(), dir))

	data, err := os.ReadFile(skillFile)
	require.NoError(t, err)
	assert.Equal(t, "TAMPERED", string(data), "file should not be overwritten when up to date")
}

func TestSyncUpdatesWhenNewer(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".claude"), 0o755))

	repoV1 := createFakeRepo(t, map[string]SkillMetadata{
		"migration": {Name: "pulumi-migration", Version: "1.0.0"},
	})
	cmd := &skillsSyncCmd{cloneFunc: fakeCloneFunc(t, repoV1)}
	require.NoError(t, cmd.runInDir(context.Background(), dir))

	repoV2 := createFakeRepo(t, map[string]SkillMetadata{
		"migration": {Name: "pulumi-migration", Version: "2.0.0"},
	})
	cmd = &skillsSyncCmd{cloneFunc: fakeCloneFunc(t, repoV2)}
	require.NoError(t, cmd.runInDir(context.Background(), dir))

	data, err := os.ReadFile(filepath.Join(dir, ".claude", "skills", "pulumi-migration",
		".claude-plugin", "plugin.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"version": "2.0.0"`)

	data, err = os.ReadFile(filepath.Join(dir, ".claude", "skills", "pulumi-migration",
		"skills", "example-skill", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "version 2.0.0")
}

func TestSyncRemovesStaleFilesOnUpdate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".claude"), 0o755))

	repoV1 := createFakeRepo(t, map[string]SkillMetadata{
		"migration": {Name: "pulumi-migration", Version: "1.0.0"},
	})
	cmd := &skillsSyncCmd{cloneFunc: fakeCloneFunc(t, repoV1)}
	require.NoError(t, cmd.runInDir(context.Background(), dir))

	staleDir := filepath.Join(dir, ".claude", "skills", "pulumi-migration", "skills", "old-skill")
	require.NoError(t, os.MkdirAll(staleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(staleDir, "SKILL.md"), []byte("stale"), 0o600))

	repoV2 := createFakeRepo(t, map[string]SkillMetadata{
		"migration": {Name: "pulumi-migration", Version: "2.0.0"},
	})
	cmd = &skillsSyncCmd{cloneFunc: fakeCloneFunc(t, repoV2)}
	require.NoError(t, cmd.runInDir(context.Background(), dir))

	_, err := os.Stat(filepath.Join(staleDir, "SKILL.md"))
	assert.True(t, os.IsNotExist(err), "stale file should be removed on update")
}

func TestSyncForce(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".claude"), 0o755))

	repoDir := createFakeRepo(t, map[string]SkillMetadata{
		"migration": {Name: "pulumi-migration", Version: "1.0.0"},
	})

	cmd := &skillsSyncCmd{cloneFunc: fakeCloneFunc(t, repoDir)}
	require.NoError(t, cmd.runInDir(context.Background(), dir))

	skillFile := filepath.Join(dir, ".claude", "skills", "pulumi-migration",
		"skills", "example-skill", "SKILL.md")
	require.NoError(t, os.WriteFile(skillFile, []byte("TAMPERED"), 0o600))

	cmd.force = true
	require.NoError(t, cmd.runInDir(context.Background(), dir))

	data, err := os.ReadFile(skillFile)
	require.NoError(t, err)
	assert.NotEqual(t, "TAMPERED", string(data), "force should overwrite tampered file")
	assert.Contains(t, string(data), "pulumi-migration")
}

func TestSyncNoSkillsInRepo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".claude"), 0o755))

	emptyRepo := t.TempDir()
	cmd := &skillsSyncCmd{cloneFunc: fakeCloneFunc(t, emptyRepo)}
	err := cmd.runInDir(context.Background(), dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no skills found")
}

func TestSyncIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".claude"), 0o755))

	cmd := &skillsSyncCmd{}
	require.NoError(t, cmd.runInDir(context.Background(), dir))

	entries, err := os.ReadDir(filepath.Join(dir, ".claude", "skills"))
	require.NoError(t, err)
	assert.NotEmpty(t, entries, "expected at least one skill installed")

	for _, entry := range entries {
		meta, err := parseSkillMetadata(filepath.Join(dir, ".claude", "skills", entry.Name()))
		require.NoError(t, err, "each installed dir should have valid metadata")
		assert.NotEmpty(t, meta.Name)
		assert.NotEmpty(t, meta.Version)
	}
}
