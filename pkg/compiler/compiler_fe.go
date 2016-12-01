// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"github.com/satori/go.uuid"

	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/compiler/backends"
	"github.com/marapongo/mu/pkg/compiler/backends/clouds"
	"github.com/marapongo/mu/pkg/compiler/backends/schedulers"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/errors"
	"github.com/marapongo/mu/pkg/util"
	"github.com/marapongo/mu/pkg/workspace"
)

// buildDocumentFE runs the front-end phases of the compiler.
func (c *compiler) buildDocumentFE(w workspace.W, doc *diag.Document) *ast.Stack {
	// If there's a workspace-wide settings file available, load it up.
	wdoc, err := w.ReadSettings()
	if err != nil {
		// TODO: we should include the file information in the error message.
		c.Diag().Errorf(errors.ErrorIO, err)
		return nil
	}

	// Now create a parser to create ASTs from the workspace settings file and Mufile.
	p := NewParser(c)
	if wdoc != nil {
		// Store the parsed AST on the workspace object itself.
		*w.Settings() = *p.ParseWorkspace(wdoc)
	}

	// Determine what cloud target we will be using; we need this to process the Mufile and imports.
	cl, a := c.detectClusterArch(w)
	if !c.Diag().Success() {
		return nil
	}
	c.ctx = c.ctx.WithClusterArch(cl, a)
	util.Assert(c.ctx.Cluster != nil)

	// Now parse the stack, using whatever args may have been supplied as the properties.
	// TODO[marapongo/mu#7]: we want to strongly type the properties; e.g., a stack expecting a number should
	//     get a number, etc.  However, to know that we must first have parsed the metadata for the target stack!
	props := make(ast.PropertyBag)
	for arg, val := range c.opts.Args {
		props[arg] = val
	}
	stack := p.ParseStack(doc, props)

	// If any parser errors occurred, bail now to prevent needlessly obtuse error messages.
	if !p.Diag().Success() {
		return nil
	}

	return stack
}

// detectClusterArch uses a variety of mechanisms to discover the target architecture, returning it.  If no
// architecture was discovered, an error is issued, and the bool return will be false.
func (c *compiler) detectClusterArch(w workspace.W) (*ast.Cluster, backends.Arch) {
	// Cluster and architectures settings may come from one of two places, in order of search preference:
	//		1) command line arguments.
	//		2) cluster-wide settings in a workspace.
	arch := c.opts.Arch

	// If a cluster was specified, look it up and load up its options.
	var cluster *ast.Cluster
	if c.opts.Cluster != "" {
		if cl, exists := w.Settings().Clusters[c.opts.Cluster]; exists {
			cluster = &cl
		} else {
			c.Diag().Errorf(errors.ErrorClusterNotFound, c.opts.Cluster)
			return nil, arch
		}
	}

	// If no cluster was specified or discovered yet, see if there is a default one to use.
	if cluster == nil {
		for _, cl := range w.Settings().Clusters {
			if cl.Default {
				cluster = &cl
				break
			}
		}
	}

	if cluster == nil {
		// If no target was found, and we don't have an architecture, error out.
		if arch.Cloud == clouds.None && !c.opts.SkipCodegen {
			c.Diag().Errorf(errors.ErrorMissingTarget)
			return nil, arch
		}

		// If we got here, generate an "anonymous" cluster, so that we at least have a name.
		cluster = c.newAnonCluster(arch)
	} else {
		// If a target was found, go ahead and extract and validate the target architecture.
		a, ok := c.extractClusterArch(cluster, arch)
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

// extractClusterArch gets and validates the architecture from an existing target.
func (c *compiler) extractClusterArch(cluster *ast.Cluster, existing backends.Arch) (backends.Arch, bool) {
	targetCloud := existing.Cloud
	targetScheduler := existing.Scheduler

	// If specified, look up the cluster's architecture settings.
	if cluster.Cloud != "" {
		tc, ok := clouds.Values[cluster.Cloud]
		if !ok {
			c.Diag().Errorf(errors.ErrorUnrecognizedCloudArch, cluster.Cloud)
			return existing, false
		}
		targetCloud = tc
	}
	if cluster.Scheduler != "" {
		ts, ok := schedulers.Values[cluster.Scheduler]
		if !ok {
			c.Diag().Errorf(errors.ErrorUnrecognizedSchedulerArch, cluster.Scheduler)
			return existing, false
		}
		targetScheduler = ts
	}

	// Ensure there aren't any conflicts, comparing compiler options to cluster settings.
	tarch := backends.Arch{targetCloud, targetScheduler}
	if targetCloud != existing.Cloud && existing.Cloud != clouds.None {
		c.Diag().Errorf(errors.ErrorConflictingClusterArchSelection, existing, cluster.Name, tarch)
		return tarch, false
	}
	if targetScheduler != existing.Scheduler && existing.Scheduler != schedulers.None {
		c.Diag().Errorf(errors.ErrorConflictingClusterArchSelection, existing, cluster.Name, tarch)
		return tarch, false
	}

	return tarch, true
}
