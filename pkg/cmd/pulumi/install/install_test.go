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

package install

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var _ workspace.BaseProject = &mockProject{}

type mockProject struct {
	name     string
	packages map[string]workspace.PackageSpec
	runtime  workspace.ProjectRuntimeInfo
}

func (m *mockProject) GetPackageSpecs() map[string]workspace.PackageSpec {
	return m.packages
}

func (m *mockProject) RuntimeInfo() *workspace.ProjectRuntimeInfo {
	return &m.runtime
}

func (m *mockProject) Validate() error {
	panic("mockProject.Validate was called")
}

func (m *mockProject) Save(path string) error {
	panic("mockProject.Save was called")
}

func (m *mockProject) AddPackage(name string, spec workspace.PackageSpec) {
	if m.packages == nil {
		m.packages = make(map[string]workspace.PackageSpec)
	}
	m.packages[name] = spec
}

// newMockProject creates a new mock project with the given name and packages
func newMockProject(name string, packages map[string]workspace.PackageSpec) *mockProject {
	return &mockProject{
		name:     name,
		packages: packages,
		runtime:  workspace.NewProjectRuntimeInfo("nodejs", nil),
	}
}

// callTracker tracks calls to walkPackage and walkProject for testing
type callTracker struct {
	mu       sync.Mutex
	packages []string
	projects []string
}

func (ct *callTracker) walkPackage(
	pctx *plugin.Context, proj workspace.BaseProject, pkgName string, pkgSpec workspace.PackageSpec,
) error {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.packages = append(ct.packages, pkgName)
	return nil
}

func (ct *callTracker) walkProject(pctx *plugin.Context, proj *mockProject) error {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.projects = append(ct.projects, proj.name)
	return nil
}

func (ct *callTracker) getPackageCalls() []string {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	result := make([]string, len(ct.packages))
	copy(result, ct.packages)
	return result
}

func (ct *callTracker) getProjectCalls() []string {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	result := make([]string, len(ct.projects))
	copy(result, ct.projects)
	return result
}

func (ct *callTracker) getPackageCallCount(pkgName string) int {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	count := 0
	for _, p := range ct.packages {
		if p == pkgName {
			count++
		}
	}
	return count
}

// mockProjectRegistry manages mock projects for testing with filesystem support
type mockProjectRegistry struct {
	mu       sync.Mutex
	projects map[string]*mockProject
	tempDir  string
	t        *testing.T
}

func newMockProjectRegistry(t *testing.T) *mockProjectRegistry {
	tempDir := t.TempDir()
	return &mockProjectRegistry{
		projects: make(map[string]*mockProject),
		tempDir:  tempDir,
		t:        t,
	}
}

func (r *mockProjectRegistry) add(name string, proj *mockProject) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Create a directory for this project
	projDir := filepath.Join(r.tempDir, name)
	err := os.MkdirAll(projDir, 0o755)
	require.NoError(r.t, err)

	// Create a minimal Pulumi.yaml file
	pulumiYaml := filepath.Join(projDir, "Pulumi.yaml")
	content := fmt.Sprintf("name: %s\nruntime: nodejs\n", proj.name)
	err = os.WriteFile(pulumiYaml, []byte(content), 0o600)
	require.NoError(r.t, err)

	r.projects[pulumiYaml] = proj
	return projDir
}

func (r *mockProjectRegistry) loadProject(path string) (*mockProject, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	proj, ok := r.projects[path]
	if !ok {
		return nil, fmt.Errorf("project not found at path: %s", path)
	}
	return proj, nil
}

func newTestPluginContext(t *testing.T, root string) *plugin.Context {
	ctx := context.Background()
	pctx, err := plugin.NewContext(ctx, nil, nil, nil, nil, root, nil, false, nil)
	require.NoError(t, err)
	return pctx
}

func TestWalkLocalPackagesFromProject_SinglePackage(t *testing.T) {
	t.Parallel()

	registry := newMockProjectRegistry(t)
	tracker := &callTracker{}

	// Mock the local package
	pkgA := newMockProject("pkg-a", nil)
	pkgAPath := registry.add("pkg-a", pkgA)

	// Create a project with a single local package
	proj := newMockProject("root", map[string]workspace.PackageSpec{
		"pkg-a": {Source: pkgAPath},
	})
	rootPath := registry.add("root", proj)

	pctx := newTestPluginContext(t, rootPath)

	err := walkLocalPackagesFromProject(pctx, proj, tracker.walkPackage, tracker.walkProject, registry.loadProject)
	require.NoError(t, err)

	// Verify walkPackage was called exactly once
	assert.Equal(t, []string{"pkg-a"}, tracker.getPackageCalls())
	// Verify walkProject was called for both the root and the local package
	assert.Equal(t, []string{"pkg-a", "root"}, tracker.getProjectCalls())
}

func TestWalkLocalPackagesFromProject_MultipleIndependentPackages(t *testing.T) {
	t.Parallel()

	registry := newMockProjectRegistry(t)
	tracker := &callTracker{}

	// Mock the local packages
	pkgA := newMockProject("pkg-a", nil)
	pkgB := newMockProject("pkg-b", nil)
	pkgC := newMockProject("pkg-c", nil)
	pkgAPath := registry.add("pkg-a", pkgA)
	pkgBPath := registry.add("pkg-b", pkgB)
	pkgCPath := registry.add("pkg-c", pkgC)

	// Create a project with three independent local packages
	proj := newMockProject("root", map[string]workspace.PackageSpec{
		"pkg-a": {Source: pkgAPath},
		"pkg-b": {Source: pkgBPath},
		"pkg-c": {Source: pkgCPath},
	})
	rootPath := registry.add("root", proj)

	pctx := newTestPluginContext(t, rootPath)

	err := walkLocalPackagesFromProject(pctx, proj, tracker.walkPackage, tracker.walkProject, registry.loadProject)
	require.NoError(t, err)

	// Verify each package was called exactly once
	pkgCalls := tracker.getPackageCalls()
	require.Len(t, pkgCalls, 3)
	assert.Contains(t, pkgCalls, "pkg-a")
	assert.Contains(t, pkgCalls, "pkg-b")
	assert.Contains(t, pkgCalls, "pkg-c")

	// Verify walkProject was called for all projects
	projCalls := tracker.getProjectCalls()
	require.Len(t, projCalls, 4)
	assert.Contains(t, projCalls, "pkg-a")
	assert.Contains(t, projCalls, "pkg-b")
	assert.Contains(t, projCalls, "pkg-c")
	assert.Contains(t, projCalls, "root")
}

func TestWalkLocalPackagesFromProject_LinearDependencyChain(t *testing.T) {
	t.Parallel()

	registry := newMockProjectRegistry(t)
	tracker := &callTracker{}

	// Mock the packages first
	projC := newMockProject("pkg-c", nil)
	pkgCPath := registry.add("pkg-c", projC)

	projB := newMockProject("pkg-b", map[string]workspace.PackageSpec{
		"pkg-c": {Source: pkgCPath},
	})
	pkgBPath := registry.add("pkg-b", projB)

	// Create a linear dependency chain: A -> B -> C
	projA := newMockProject("pkg-a", map[string]workspace.PackageSpec{
		"pkg-b": {Source: pkgBPath},
	})
	pkgAPath := registry.add("pkg-a", projA)

	pctx := newTestPluginContext(t, pkgAPath)

	err := walkLocalPackagesFromProject(pctx, projA, tracker.walkPackage, tracker.walkProject, registry.loadProject)
	require.NoError(t, err)

	// Verify each package was called exactly once
	// Note: projA is the root project, so only its packages (pkg-b) and transitive packages (pkg-c) are walked
	assert.Equal(t, 1, tracker.getPackageCallCount("pkg-b"))
	assert.Equal(t, 1, tracker.getPackageCallCount("pkg-c"))

	// Verify ordering: C must be walked before B (per documented invariant)
	calls := tracker.getPackageCalls()
	require.Len(t, calls, 2)

	indexB := -1
	indexC := -1
	for i, call := range calls {
		switch call {
		case "pkg-b":
			indexB = i
		case "pkg-c":
			indexC = i
		}
	}

	require.NotEqual(t, -1, indexB, "pkg-b not found in calls")
	require.NotEqual(t, -1, indexC, "pkg-c not found in calls")

	assert.Less(t, indexC, indexB, "pkg-c must be walked before pkg-b")
}

func TestWalkLocalPackagesFromProject_DiamondDependency(t *testing.T) {
	t.Parallel()

	registry := newMockProjectRegistry(t)
	tracker := &callTracker{}

	// Mock packages starting from the bottom of the dependency tree
	projD := newMockProject("pkg-d", nil)
	pkgDPath := registry.add("pkg-d", projD)

	projB := newMockProject("pkg-b", map[string]workspace.PackageSpec{
		"pkg-d": {Source: pkgDPath},
	})
	pkgBPath := registry.add("pkg-b", projB)

	projC := newMockProject("pkg-c", map[string]workspace.PackageSpec{
		"pkg-d": {Source: pkgDPath},
	})
	pkgCPath := registry.add("pkg-c", projC)

	// Create a diamond dependency: A depends on B and C, both B and C depend on D
	projA := newMockProject("pkg-a", map[string]workspace.PackageSpec{
		"pkg-b": {Source: pkgBPath},
		"pkg-c": {Source: pkgCPath},
	})
	pkgAPath := registry.add("pkg-a", projA)

	pctx := newTestPluginContext(t, pkgAPath)

	err := walkLocalPackagesFromProject(pctx, projA, tracker.walkPackage, tracker.walkProject, registry.loadProject)
	require.NoError(t, err)

	// CRITICAL: pkg-d must be walked exactly once (deduplication test)
	assert.Equal(t, 1, tracker.getPackageCallCount("pkg-d"), "pkg-d must be walked exactly once")

	// Verify all packages were walked
	// Note: projA is the root, so only its packages (pkg-b, pkg-c) and their transitive deps (pkg-d) are walked
	assert.Equal(t, 1, tracker.getPackageCallCount("pkg-b"))
	assert.Equal(t, 1, tracker.getPackageCallCount("pkg-c"))

	// Verify ordering: D must be walked before both B and C (per documented invariant)
	calls := tracker.getPackageCalls()
	require.Len(t, calls, 3)

	indexB := -1
	indexC := -1
	indexD := -1
	for i, call := range calls {
		switch call {
		case "pkg-b":
			indexB = i
		case "pkg-c":
			indexC = i
		case "pkg-d":
			indexD = i
		}
	}

	require.NotEqual(t, -1, indexB, "pkg-b not found in calls")
	require.NotEqual(t, -1, indexC, "pkg-c not found in calls")
	require.NotEqual(t, -1, indexD, "pkg-d not found in calls")

	assert.Less(t, indexD, indexB, "pkg-d must be walked before pkg-b")
	assert.Less(t, indexD, indexC, "pkg-d must be walked before pkg-c")
}

func TestWalkLocalPackagesFromProject_MixedLocalAndRemote(t *testing.T) {
	t.Parallel()

	registry := newMockProjectRegistry(t)
	tracker := &callTracker{}

	// Mock only the local package
	pkgA := newMockProject("pkg-a", nil)
	pkgAPath := registry.add("pkg-a", pkgA)

	// Create a project with both local and remote packages
	proj := newMockProject("root", map[string]workspace.PackageSpec{
		"local-pkg":  {Source: pkgAPath},
		"remote-pkg": {Source: "pulumi/aws", Version: "6.0.0"},
	})
	rootPath := registry.add("root", proj)

	pctx := newTestPluginContext(t, rootPath)

	err := walkLocalPackagesFromProject(pctx, proj, tracker.walkPackage, tracker.walkProject, registry.loadProject)
	require.NoError(t, err)

	// Verify both packages were walked
	pkgCalls := tracker.getPackageCalls()
	require.Len(t, pkgCalls, 2)
	assert.Contains(t, pkgCalls, "local-pkg")
	assert.Contains(t, pkgCalls, "remote-pkg")

	// Verify walkProject was called for root and local package
	projCalls := tracker.getProjectCalls()
	require.Len(t, projCalls, 2)
	assert.Contains(t, projCalls, "pkg-a")
	assert.Contains(t, projCalls, "root")
}

func TestWalkLocalPackagesFromProject_ErrorPropagation(t *testing.T) {
	t.Parallel()

	t.Run("loadProject error", func(t *testing.T) {
		t.Parallel()

		pctx := newTestPluginContext(t, "")
		registry := newMockProjectRegistry(t)
		tracker := &callTracker{}

		// Create a real directory with Pulumi.yaml so DetectProjectPathFrom succeeds,
		// but have loadProject fail
		pkgA := newMockProject("pkg-a", nil)
		pkgAPath := registry.add("pkg-a", pkgA)

		proj := newMockProject("root", map[string]workspace.PackageSpec{
			"pkg-a": {Source: pkgAPath},
		})

		loadProjectErr := errors.New("failed to load project")
		loadProject := func(path string) (*mockProject, error) {
			return nil, loadProjectErr
		}

		err := walkLocalPackagesFromProject(pctx, proj, tracker.walkPackage, nil, loadProject)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load project")

		// Verify walkPackage was not called since loading failed
		assert.Empty(t, tracker.getPackageCalls())
	})

	t.Run("walkPackage error", func(t *testing.T) {
		t.Parallel()

		pctx := newTestPluginContext(t, "")
		registry := newMockProjectRegistry(t)

		pkgA := newMockProject("pkg-a", nil)
		pkgAPath := registry.add("pkg-a", pkgA)

		proj := newMockProject("root", map[string]workspace.PackageSpec{
			"pkg-a": {Source: pkgAPath},
		})

		walkPackageErr := errors.New("failed to walk package")
		walkPackage := func(*plugin.Context, workspace.BaseProject, string, workspace.PackageSpec) error {
			return walkPackageErr
		}

		err := walkLocalPackagesFromProject(pctx, proj, walkPackage, nil, registry.loadProject)
		require.ErrorIs(t, err, walkPackageErr)
	})

	t.Run("walkProject error", func(t *testing.T) {
		t.Parallel()

		pctx := newTestPluginContext(t, "")
		registry := newMockProjectRegistry(t)
		tracker := &callTracker{}

		pkgA := newMockProject("pkg-a", nil)
		pkgAPath := registry.add("pkg-a", pkgA)

		proj := newMockProject("root", map[string]workspace.PackageSpec{
			"pkg-a": {Source: pkgAPath},
		})

		walkProjectErr := errors.New("failed to walk project")
		walkProject := func(pctx *plugin.Context, proj *mockProject) error {
			return walkProjectErr
		}

		err := walkLocalPackagesFromProject(pctx, proj, tracker.walkPackage, walkProject, registry.loadProject)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to walk project")

		// Verify walkPackage WAS called (it runs before walkProject)
		assert.Equal(t, 1, tracker.getPackageCallCount("pkg-a"))
	})

	t.Run("multiple errors", func(t *testing.T) {
		t.Parallel()

		pctx := newTestPluginContext(t, "")
		registry := newMockProjectRegistry(t)

		pkgA := newMockProject("pkg-a", nil)
		pkgB := newMockProject("pkg-b", nil)
		pkgAPath := registry.add("pkg-a", pkgA)
		pkgBPath := registry.add("pkg-b", pkgB)

		proj := newMockProject("root", map[string]workspace.PackageSpec{
			"pkg-a": {Source: pkgAPath},
			"pkg-b": {Source: pkgBPath},
		})

		walkPackageErr := func(
			pctx *plugin.Context, proj workspace.BaseProject, pkgName string, pkgSpec workspace.PackageSpec,
		) error {
			return fmt.Errorf("failed to walk %s", pkgName)
		}

		err := walkLocalPackagesFromProject(pctx, proj, walkPackageErr, nil, registry.loadProject)
		require.Error(t, err)
		// At least one error should be present (both packages fail, but due to parallel execution
		// and map iteration order being random, we may get one or both errors)
		errMsg := err.Error()
		hasErrorA := strings.Contains(errMsg, "failed to walk pkg-a")
		hasErrorB := strings.Contains(errMsg, "failed to walk pkg-b")
		assert.True(t, hasErrorA || hasErrorB, "should contain at least one package error")
	})
}

func TestWalkLocalPackagesFromProject_NilWalkProject(t *testing.T) {
	t.Parallel()

	pctx := newTestPluginContext(t, "")
	registry := newMockProjectRegistry(t)
	tracker := &callTracker{}

	pkgA := newMockProject("pkg-a", nil)
	pkgAPath := registry.add("pkg-a", pkgA)

	proj := newMockProject("root", map[string]workspace.PackageSpec{
		"pkg-a": {Source: pkgAPath},
	})

	// Pass nil for walkProject
	err := walkLocalPackagesFromProject(pctx, proj, tracker.walkPackage, nil, registry.loadProject)
	require.NoError(t, err)

	// Verify walkPackage was still called
	assert.Equal(t, []string{"pkg-a"}, tracker.getPackageCalls())
	// Verify no project calls were made
	assert.Empty(t, tracker.getProjectCalls())
}

func TestWalkLocalPackagesFromProject_EmptyProject(t *testing.T) {
	t.Parallel()

	pctx := newTestPluginContext(t, "")
	tracker := &callTracker{}

	// Create a project with no packages
	proj := newMockProject("root", nil)

	loadProject := func(path string) (*mockProject, error) {
		return nil, errors.New("should not be called")
	}

	err := walkLocalPackagesFromProject(pctx, proj, tracker.walkPackage, tracker.walkProject, loadProject)
	require.NoError(t, err)

	// Verify no packages were walked
	assert.Empty(t, tracker.getPackageCalls())
	// Verify walkProject was still called for the root project
	assert.Equal(t, []string{"root"}, tracker.getProjectCalls())
}

func TestWalkLocalPackagesFromProject_CircularDependency(t *testing.T) {
	t.Parallel()

	registry := newMockProjectRegistry(t)
	tracker := &callTracker{}

	// We need to create both projects and then update their packages after getting paths
	// Create placeholder projects
	projA := newMockProject("pkg-a", nil)
	pkgAPath := registry.add("pkg-a", projA)

	projB := newMockProject("pkg-b", nil)
	pkgBPath := registry.add("pkg-b", projB)

	// Now add the circular dependency: A -> B -> A
	projA.AddPackage("pkg-b", workspace.PackageSpec{Source: pkgBPath})
	projB.AddPackage("pkg-a", workspace.PackageSpec{Source: pkgAPath})

	pctx := newTestPluginContext(t, pkgAPath)

	err := walkLocalPackagesFromProject(pctx, projA, tracker.walkPackage, tracker.walkProject, registry.loadProject)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cyclic dependency detected")
}

// TestWalkLocalPackagesFromProject_ConcurrencyLimit checks that
// walkLocalPackagesFromProject will saturate to the concurrency limit, but not above it.
//
// Unfortunately, the test is non-deterministic for detecting over-saturation. That is
// inherent in this kind of test, since we can't wait for over-saturation (otherwise
// working implementation would never return).
func TestWalkLocalPackagesFromProject_ConcurrencyLimit(t *testing.T) {
	t.Parallel()

	/*
		Package dependency structure (10 packages total):

		           root
		          /    \
		         A      B
		       / | \ \ / | \ \
		      A1 A2 A3 A4 B1 B2 B3 B4
		              \ X /
		               \ /

		This creates a dependency tree where:
		- root depends on A and B
		- A depends on A1, A2, A3, A4, B1 (shared: A4, B1)
		- B depends on B2, B3, B4, A4, B1 (shared: A4, B1)
		- Total of 10 non-root packages to install
		- A4 and B1 are shared dependencies (diamond pattern)
	*/

	registry := newMockProjectRegistry(t)
	pctx := newTestPluginContext(t, "")

	// Create leaf packages (A1-A4, B1-B4)
	leafPackages := []string{"A1", "A2", "A3", "A4", "B1", "B2", "B3", "B4"}
	for _, name := range leafPackages {
		proj := newMockProject(name, nil)
		registry.add(name, proj)
	}

	// Create package A with dependencies on A1-A4 and B1 (shared)
	pkgA := newMockProject("A", map[string]workspace.PackageSpec{
		"A1": {Source: registry.tempDir + "/A1"},
		"A2": {Source: registry.tempDir + "/A2"},
		"A3": {Source: registry.tempDir + "/A3"},
		"A4": {Source: registry.tempDir + "/A4"},
		"B1": {Source: registry.tempDir + "/B1"},
	})
	pkgAPath := registry.add("A", pkgA)

	// Create package B with dependencies on B1-B4 and A4 (shared)
	pkgB := newMockProject("B", map[string]workspace.PackageSpec{
		"B1": {Source: registry.tempDir + "/B1"},
		"B2": {Source: registry.tempDir + "/B2"},
		"B3": {Source: registry.tempDir + "/B3"},
		"B4": {Source: registry.tempDir + "/B4"},
		"A4": {Source: registry.tempDir + "/A4"},
	})
	pkgBPath := registry.add("B", pkgB)

	// Create root with dependencies on A and B
	root := newMockProject("root", map[string]workspace.PackageSpec{
		"A": {Source: pkgAPath},
		"B": {Source: pkgBPath},
	})
	rootPath := registry.add("root", root)

	pctx = newTestPluginContext(t, rootPath)

	phases := []*struct {
		maxConcurency                atomic.Int32
		expectedMaxConcurrency       int
		done, decr                   chan struct{}
		saturatedIncr, saturatedDecr sync.WaitGroup
	}{
		{expectedMaxConcurrency: 4}, // The first 4 packages
		{expectedMaxConcurrency: 4}, // The next 4 packages
		{expectedMaxConcurrency: 2}, // The last 2 packages
	}
	for _, p := range phases {
		p.done = make(chan struct{})
		p.decr = make(chan struct{})
		p.saturatedIncr.Add(p.expectedMaxConcurrency)
		p.saturatedDecr.Add(p.expectedMaxConcurrency)
	}
	var phaseNumber atomic.Int32
	var currentConcurency atomic.Int32
	walkPackage := func(*plugin.Context, workspace.BaseProject, string, workspace.PackageSpec) error {
		// Set the max concurrency for the phase
		phase := phases[phaseNumber.Load()]
		maxConcurrency := &phase.maxConcurency
		for newMax := currentConcurency.Add(1); ; {
			currentMax := maxConcurrency.Load()
			if currentMax >= newMax {
				break
			}
			if currentMax < newMax && maxConcurrency.CompareAndSwap(currentMax, newMax) {
				break
			}
		}

		phase.saturatedIncr.Done()
		<-phase.done

		// Decrement the concurrency for the phase
		currentConcurency.Add(-1)
		phase.saturatedDecr.Done()
		<-phase.decr

		return nil
	}

	done := make(chan struct{})
	go func() { // walkLocalPackagesFromProject is sync, so it must run in the background
		err := walkLocalPackagesFromProject(pctx, root, walkPackage, nil, registry.loadProject)
		require.NoError(t, err)
		close(done)
	}()

	for i, phase := range phases {
		t.Run(fmt.Sprintf("phase-%d", i+1), func(t *testing.T) {
			phase.saturatedIncr.Wait()
			assert.Equal(t, phase.expectedMaxConcurrency, int(phase.maxConcurency.Load()))
			close(phase.done)
			phase.saturatedDecr.Wait()
			phaseNumber.Add(1)
			close(phase.decr)
		})
	}
	<-done // Check that walkLocalPackagesFromProject has returned
}
