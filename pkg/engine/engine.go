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

// SnapshotProvider abstracts away retriving and storing snapshots
type SnapshotProvider interface {
	GetSnapshot(name tokens.QName) (*deploy.Snapshot, error)
	SaveSnapshot(snapshot *deploy.Snapshot) error
}
