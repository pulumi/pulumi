// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package local

import (
	"os"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

type localSnapshotPersister struct{}

func (sp *localSnapshotPersister) SaveSnapshot(snapshot *deploy.Snapshot) error {
	stack := snapshot.Stack
	contract.Assert(stack != "")

	config, _, _, err := getStack(stack)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	_, err = saveStack(stack, config, snapshot)
	return err
}

var _ backend.SnapshotPersister = &localSnapshotPersister{}
