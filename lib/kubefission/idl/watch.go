// Copyright 2017 Pulumi, Inc. All rights reserved.

package kubefission

import (
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// Watch is a specification of a Kubernetes watch along with a URL to post events to.
type Watch struct {
	idl.NamedResource
	Namespace     string    `lumi:"namespace"`
	ObjType       string    `lumi:"objType"`
	LabelSelector string    `lumi:"labelSelector"`
	FieldSelector string    `lumi:"fieldSelector"`
	Function      *Function `lumi:"function"`
	Target        string    `lumi:"target"` // watch publish target (URL, NATS stream, etc)
}
