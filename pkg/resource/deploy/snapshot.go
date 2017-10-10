// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package deploy

import (
	"time"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/tokens"
)

// Snapshot is a view of a collection of resources in an environment at a point in time.  It describes resources; their
// IDs, names, and properties; their dependencies; and more.  A snapshot is a diffable entity and can be used to create
// or apply an infrastructure deployment plan in order to make reality match the snapshot state.
type Snapshot struct {
	Namespace tokens.QName      // the namespace target being deployed into.
	Time      time.Time         // the time this snapshot was taken.
	Resources []*resource.State // fetches all resources and their associated states.
	Info      interface{}       // optional information about the source.
}

// NewSnapshot creates a snapshot from the given arguments.  The resources must be in topologically sorted order.
func NewSnapshot(ns tokens.QName, time time.Time, resources []*resource.State, info interface{}) *Snapshot {
	return &Snapshot{
		Namespace: ns,
		Time:      time,
		Resources: resources,
		Info:      info,
	}
}
