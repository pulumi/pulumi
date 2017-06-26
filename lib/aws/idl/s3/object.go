// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

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
	Source *idl.Asset `lumi:"source,in"`
	// A standard MIME type describing the format of the object data.
	ContentType *string `lumi:"contentType,optional"`
	// Specifies presentational information for the object.
	ContentDisposition *string `lumi:"contentDisposition,optional"`
	// Specifies caching behavior along the request/reply chain.
	CacheControl *string `lumi:"cacheControl,optional"`
	// Specifies what content encodings have been applied to the object and thus
	// what decoding mechanisms must be applied to obtain the media-type referenced
	// by the Content-Type header field.
	ContentEncoding *string `lumi:"contentEncoding,optional"`
	// The language the content is in.
	ContentLanguage *string `lumi:"contentLanguage,optional"`
	// Size of the body in bytes. This parameter is useful when the size of the
	// body cannot be determined automatically.
	ContentLength *float64 `lumi:"contentLength,optional"`
}
