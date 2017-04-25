// Copyright 2017 Pulumi, Inc. All rights reserved.

package idl

// Resource is a marker struct to indicate that an IDL struct is a resource.
type Resource struct {
}

// NamedResource is a marker struct to indicate that an IDL struct is a named resource.
type NamedResource struct {
	Name string `coco:"name,replaces"`
}
