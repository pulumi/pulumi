// Copyright 2017 Pulumi, Inc. All rights reserved.

package workspace

import (
	"github.com/pulumi/coconut/pkg/config"
	"github.com/pulumi/coconut/pkg/diag"
	"github.com/pulumi/coconut/pkg/pack"
)

// Workspace defines workspace settings shared amongst many related projects.
type Workspace struct {
	Namespace    string            `json:"namespace,omitempty"` // an optional namespace for this workspace.
	Clusters     Clusters          `json:"clusters,omitempty"`  // an optional set of predefined target clusters.
	Dependencies pack.Dependencies `json:"dependencies,omitempty"`

	Doc *diag.Document `json:"-"` // the document from which this came.
}

var _ diag.Diagable = (*Workspace)(nil)

func (s *Workspace) Where() (*diag.Document, *diag.Location) {
	return s.Doc, nil
}

// Clusters is a map of target names to metadata about those targets.
type Clusters map[string]*config.Cluster
