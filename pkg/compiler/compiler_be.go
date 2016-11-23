// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"github.com/golang/glog"
	"github.com/satori/go.uuid"

	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/compiler/backends"
	"github.com/marapongo/mu/pkg/compiler/backends/clouds"
	"github.com/marapongo/mu/pkg/compiler/backends/schedulers"
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/errors"
	"github.com/marapongo/mu/pkg/workspace"
)

// buildDocumentBE runs the back-end phases of the compiler.
func (c *compiler) buildDocumentBE(w workspace.W, stack *ast.Stack) {
	if c.opts.SkipCodegen {
		glog.V(2).Infof("Skipping code-generation (opts.SkipCodegen=true)")
	} else {
		// Figure out which cloud architecture we will be targeting during code-gen.
		cluster, arch := c.discoverClusterArch(w, stack)
		if cluster != nil {
			glog.V(2).Infof("Stack %v targets cluster=%v arch=%v", stack.Name, cluster.Name, arch)

			// Now get the backend cloud provider to process the stack from here on out.
			be := backends.New(arch, c.Diag())
			be.CodeGen(core.Compiland{cluster, stack})
		}
	}
}

// discoverClusterArch uses a variety of mechanisms to discover the target architecture, returning it.  If no
// architecture was discovered, an error is issued, and the bool return will be false.
func (c *compiler) discoverClusterArch(w workspace.W, stack *ast.Stack) (*ast.Cluster, backends.Arch) {
	// Cluster and architectures settings may come from one of three places, in order of search preference:
	//		1) command line arguments.
	//		2) settings specific to this stack.
	//		3) cluster-wide settings in a workspace.
	// In other words, 1 overrides 2 which overrides 3.
	arch := c.opts.Arch

	// If a cluster was specified, look it up and load up its options.
	var cluster *ast.Cluster
	if c.opts.Cluster != "" {
		// First, check the stack to see if it has a targets section.
		if cl, exists := stack.Clusters[c.opts.Cluster]; exists {
			cluster = &cl
		} else {
			// If that didn't work, see if the workspace has an opinion.
			if cl, exists := w.Settings().Clusters[c.opts.Cluster]; exists {
				cluster = &cl
			} else {
				c.Diag().Errorf(errors.ErrorClusterNotFound.At(stack.Doc), c.opts.Cluster)
				return nil, arch
			}
		}
	}

	// If no cluster was specified or discovered yet, see if there is a default one to use.
	if cluster == nil {
		for _, cl := range stack.Clusters {
			if cl.Default {
				cluster = &cl
				break
			}
		}
		if cluster == nil {
			for _, cl := range w.Settings().Clusters {
				if cl.Default {
					cluster = &cl
					break
				}
			}
		}
	}

	if cluster == nil {
		// If no target was found, and we don't have an architecture, error out.
		if arch.Cloud == clouds.None {
			c.Diag().Errorf(errors.ErrorMissingTarget.At(stack.Doc))
			return nil, arch
		}

		// If we got here, generate an "anonymous" cluster, so that we at least have a name.
		cluster = c.newAnonCluster(arch)
	} else {
		// If a target was found, go ahead and extract and validate the target architecture.
		a, ok := c.getClusterArch(stack, cluster, arch)
		if !ok {
			return nil, arch
		}
		arch = a
	}

	return cluster, arch
}

// newAnonCluster creates an anonymous cluster for stacks that didn't declare one.
func (c *compiler) newAnonCluster(arch backends.Arch) *ast.Cluster {
	// TODO: ensure this is unique.
	// TODO: we want to cache names somewhere (~/.mu/?) so that we can reuse temporary local stacks, etc.
	return &ast.Cluster{
		Name:      uuid.NewV4().String(),
		Cloud:     clouds.Names[arch.Cloud],
		Scheduler: schedulers.Names[arch.Scheduler],
	}
}

// getClusterArch gets and validates the architecture from an existing target.
func (c *compiler) getClusterArch(stack *ast.Stack, cluster *ast.Cluster,
	existing backends.Arch) (backends.Arch, bool) {
	targetCloud := existing.Cloud
	targetScheduler := existing.Scheduler

	// If specified, look up the cluster's architecture settings.
	if cluster.Cloud != "" {
		tc, ok := clouds.Values[cluster.Cloud]
		if !ok {
			c.Diag().Errorf(errors.ErrorUnrecognizedCloudArch.At(stack.Doc), cluster.Cloud)
			return existing, false
		}
		targetCloud = tc
	}
	if cluster.Scheduler != "" {
		ts, ok := schedulers.Values[cluster.Scheduler]
		if !ok {
			c.Diag().Errorf(errors.ErrorUnrecognizedSchedulerArch.At(stack.Doc), cluster.Scheduler)
			return existing, false
		}
		targetScheduler = ts
	}

	// Ensure there aren't any conflicts, comparing compiler options to cluster settings.
	tarch := backends.Arch{targetCloud, targetScheduler}
	if targetCloud != existing.Cloud && existing.Cloud != clouds.None {
		c.Diag().Errorf(
			errors.ErrorConflictingClusterArchSelection.At(stack.Doc), existing, cluster.Name, tarch)
		return tarch, false
	}
	if targetScheduler != existing.Scheduler && existing.Scheduler != schedulers.None {
		c.Diag().Errorf(
			errors.ErrorConflictingClusterArchSelection.At(stack.Doc), existing, cluster.Name, tarch)
		return tarch, false
	}

	return tarch, true
}
