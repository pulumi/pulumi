// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package idl

// Resource is a marker struct to indicate that an IDL struct is a resource.
type Resource struct {
}

// NamedResource is a marker struct to indicate that an IDL struct is a named resource.
type NamedResource struct {
	Name *string `lumi:"name,replaces,in"`
}
