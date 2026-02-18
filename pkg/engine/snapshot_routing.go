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

package engine

import (
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// RoutingSnapshotManager routes snapshot operations to per-stack SnapshotManagers
// based on the resource URN. The engine sees one merged deployment, but every
// snapshot write is routed to the correct per-stack manager.
type RoutingSnapshotManager struct {
	// managers maps stack FQN → per-stack SnapshotManager.
	managers map[string]SnapshotManager

	// projectToFQN maps project name → stack FQN. URNs contain the project name,
	// so this is used to resolve which stack a resource belongs to.
	projectToFQN map[tokens.PackageName]string

	// perStackSnapshots maps stack FQN → the original per-stack snapshot (for Write partitioning).
	perStackSnapshots map[string]*deploy.Snapshot
}

// NewRoutingSnapshotManager creates a RoutingSnapshotManager that routes writes to the
// appropriate per-stack SnapshotManager. The projectToFQN map must contain an entry for
// every project in the multistack deployment.
func NewRoutingSnapshotManager(
	managers map[string]SnapshotManager,
	projectToFQN map[tokens.PackageName]string,
	perStackSnapshots map[string]*deploy.Snapshot,
) *RoutingSnapshotManager {
	return &RoutingSnapshotManager{
		managers:          managers,
		projectToFQN:      projectToFQN,
		perStackSnapshots: perStackSnapshots,
	}
}

// resolve maps a resource URN to the stack FQN it belongs to.
func (r *RoutingSnapshotManager) resolve(urn resource.URN) (string, error) {
	project := urn.Project()
	fqn, ok := r.projectToFQN[project]
	if !ok {
		return "", fmt.Errorf("no stack found for project %q (URN: %s)", project, urn)
	}
	return fqn, nil
}

// managerFor returns the SnapshotManager for the given URN's stack.
func (r *RoutingSnapshotManager) managerFor(urn resource.URN) (SnapshotManager, error) {
	fqn, err := r.resolve(urn)
	if err != nil {
		return nil, err
	}
	mgr, ok := r.managers[fqn]
	if !ok {
		return nil, fmt.Errorf("no snapshot manager for stack %q", fqn)
	}
	return mgr, nil
}

// BeginMutation routes to the appropriate per-stack manager based on the step's URN.
func (r *RoutingSnapshotManager) BeginMutation(step deploy.Step) (SnapshotMutation, error) {
	mgr, err := r.managerFor(step.URN())
	if err != nil {
		logging.V(4).Infof("RoutingSnapshotManager.BeginMutation: %v", err)
		return nil, err
	}
	return mgr.BeginMutation(step)
}

// RegisterResourceOutputs routes to the appropriate per-stack manager.
func (r *RoutingSnapshotManager) RegisterResourceOutputs(step deploy.Step) error {
	mgr, err := r.managerFor(step.URN())
	if err != nil {
		logging.V(4).Infof("RoutingSnapshotManager.RegisterResourceOutputs: %v", err)
		return err
	}
	return mgr.RegisterResourceOutputs(step)
}

// Write partitions the merged snapshot by stack and writes each partition to its manager.
func (r *RoutingSnapshotManager) Write(base *deploy.Snapshot) error {
	if base == nil {
		// Write nil to all managers.
		for _, mgr := range r.managers {
			if err := mgr.Write(nil); err != nil {
				return err
			}
		}
		return nil
	}

	// Partition resources by stack and build per-stack URN sets.
	partitioned := make(map[string][]*resource.State)
	partitionedURNs := make(map[string]map[resource.URN]bool)
	for _, res := range base.Resources {
		fqn, err := r.resolve(res.URN)
		if err != nil {
			logging.V(4).Infof("RoutingSnapshotManager.Write: skipping resource %s: %v", res.URN, err)
			continue
		}
		partitioned[fqn] = append(partitioned[fqn], res)
		if partitionedURNs[fqn] == nil {
			partitionedURNs[fqn] = make(map[resource.URN]bool)
		}
		partitionedURNs[fqn][res.URN] = true
	}

	// Partition pending operations by stack.
	partitionedOps := make(map[string][]resource.Operation)
	for _, op := range base.PendingOperations {
		fqn, err := r.resolve(op.Resource.URN)
		if err != nil {
			continue
		}
		partitionedOps[fqn] = append(partitionedOps[fqn], op)
	}

	// Write each partition to its manager, stripping cross-stack dependencies.
	for fqn, mgr := range r.managers {
		resources := partitioned[fqn]
		ops := partitionedOps[fqn]
		urnSet := partitionedURNs[fqn]

		// Strip cross-stack dependencies from resource states so per-stack snapshots
		// are self-consistent and pass integrity checks when loaded independently.
		cleaned := make([]*resource.State, len(resources))
		for i, res := range resources {
			cleaned[i] = stripCrossStackDeps(res, urnSet)
		}

		// Use the per-stack snapshot's secrets manager and manifest if available.
		var sm = base.SecretsManager
		var manifest = base.Manifest
		if orig, ok := r.perStackSnapshots[fqn]; ok && orig != nil {
			if orig.SecretsManager != nil {
				sm = orig.SecretsManager
			}
			manifest = orig.Manifest
		}

		snap := deploy.NewSnapshot(manifest, sm, cleaned, ops, deploy.SnapshotMetadata{})
		if err := mgr.Write(snap); err != nil {
			return fmt.Errorf("writing snapshot for stack %q: %w", fqn, err)
		}
	}
	return nil
}

// stripCrossStackDeps returns a shallow copy of the resource state with any dependencies
// or property dependencies that reference URNs outside the given set removed. This ensures
// per-stack snapshots are self-consistent when persisted independently.
func stripCrossStackDeps(res *resource.State, urnSet map[resource.URN]bool) *resource.State {
	// Check if any deps reference outside URNs.
	needsClean := false
	for _, dep := range res.Dependencies {
		if !urnSet[dep] {
			needsClean = true
			break
		}
	}
	if !needsClean {
		for _, deps := range res.PropertyDependencies {
			for _, dep := range deps {
				if !urnSet[dep] {
					needsClean = true
					break
				}
			}
			if needsClean {
				break
			}
		}
	}

	if !needsClean {
		return res
	}

	// Shallow-copy the state and filter deps.
	clone := *res
	clone.Dependencies = filterURNs(res.Dependencies, urnSet)
	if len(res.PropertyDependencies) > 0 {
		clone.PropertyDependencies = make(map[resource.PropertyKey][]resource.URN, len(res.PropertyDependencies))
		for key, deps := range res.PropertyDependencies {
			clone.PropertyDependencies[key] = filterURNs(deps, urnSet)
		}
	}
	return &clone
}

// filterURNs returns only the URNs present in the given set.
func filterURNs(urns []resource.URN, keep map[resource.URN]bool) []resource.URN {
	var filtered []resource.URN
	for _, u := range urns {
		if keep[u] {
			filtered = append(filtered, u)
		}
	}
	return filtered
}

// RebuiltBaseState calls RebuiltBaseState on all managers.
func (r *RoutingSnapshotManager) RebuiltBaseState() error {
	for fqn, mgr := range r.managers {
		if err := mgr.RebuiltBaseState(); err != nil {
			return fmt.Errorf("rebuilding base state for stack %q: %w", fqn, err)
		}
	}
	return nil
}

// Close closes all per-stack managers.
func (r *RoutingSnapshotManager) Close() error {
	var firstErr error
	for fqn, mgr := range r.managers {
		if err := mgr.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("closing snapshot manager for stack %q: %w", fqn, err)
		}
	}
	return firstErr
}
