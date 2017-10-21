// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/tokens"
)

type Engine struct {
	Targets   TargetProvider
	Snapshots SnapshotProvider
}

// TargetProvider abstracts away retriving a target
type TargetProvider interface {
	GetTarget(name tokens.QName) (*deploy.Target, error)
}

// Mutation abstracts away managing changes to snapshots
type Mutation interface {
	End(snapshot *deploy.Snapshot) error
}

// SnapshotProvider abstracts away retrieving and storing snapshots
type SnapshotProvider interface {
	GetSnapshot(name tokens.QName) (*deploy.Snapshot, error)
	BeginMutation(name tokens.QName) (Mutation, error)
}
