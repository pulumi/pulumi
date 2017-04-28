// Copyright 2017 Pulumi, Inc. All rights reserved.

package kubefission

import (
	"github.com/pulumi/coconut/pkg/resource/idl"
)

// Watch is a specification of a Kubernetes watch along with a URL to post events to.
type Watch struct {
	idl.NamedResource
	Namespace     string    `coco:"namespace"`
	ObjType       string    `coco:"objType"`
	LabelSelector string    `coco:"labelSelector"`
	FieldSelector string    `coco:"fieldSelector"`
	Function      *Function `coco:"function"`
	Target        string    `coco:"target"` // watch publish target (URL, NATS stream, etc)
}
