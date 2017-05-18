// Copyright 2017 Pulumi, Inc. All rights reserved.

package s3

import (
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// Object represents an Amazon Simple Storage Service (S3) object (key/value blob).
type Object struct {
	idl.Resource
	// The Key that uniquely identifies this object.
	Key string `lumi:"key,replaces"`
	// The Bucket this object belongs to.
	Bucket *Bucket `lumi:"bucket,replaces"`
	// The Source of content for this object.
	Source *idl.Asset `lumi:"source,replaces"`
}
