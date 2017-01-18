// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"github.com/satori/go.uuid"

	"github.com/marapongo/mu/pkg/compiler/backends"
	"github.com/marapongo/mu/pkg/compiler/backends/clouds"
	"github.com/marapongo/mu/pkg/compiler/backends/schedulers"
	"github.com/marapongo/mu/pkg/compiler/errors"
	"github.com/marapongo/mu/pkg/config"
	"github.com/marapongo/mu/pkg/workspace"
)

// detectClusterArch uses a variety of mechanisms to discover the target architecture, returning it.  If no
// architecture was discovered, an error is issued, and the bool return will be false.
func (c *compiler) detectClusterArch(w workspace.W) (*config.Cluster, backends.Arch) {
	// Cluster and architectures settings may come from one of two places, in order of search preference:
	//		1) command line arguments.
	//		2) cluster-wide settings in a workspace.
	var arch backends.Arch
	var cluster *config.Cluster

	// If a cluster was specified, look it up and load up its options.
	clname := c.Options().Cluster
	if clname != "" {
		if cl, exists := w.Settings().Clusters[clname]; exists {
			cluster = cl
		} else {
			c.Diag().Errorf(errors.ErrorClusterNotFound, clname)
			return nil, arch
		}
	}

	// If no cluster was specified or discovered yet, see if there is a default one to use.
	if cluster == nil {
		for _, cl := range w.Settings().Clusters {
			if cl.Default {
				cluster = cl
				break
			}
		}
	}

	if cluster == nil {
		// If no target was found, and we don't have an architecture, error out.
		if arch.Cloud == clouds.None {
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
func (c *compiler) newAnonCluster(arch backends.Arch) *config.Cluster {
	// TODO: ensure this is unique.
	// TODO: we want to cache names somewhere (~/.mu/?) so that we can reuse temporary local stacks, etc.
	return &config.Cluster{
		Name:      uuid.NewV4().String(),
		Cloud:     clouds.Names[arch.Cloud],
		Scheduler: schedulers.Names[arch.Scheduler],
	}
}

// extractClusterArch gets and validates the architecture from an existing target.
func (c *compiler) extractClusterArch(cluster *config.Cluster, existing backends.Arch) (backends.Arch, bool) {
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
