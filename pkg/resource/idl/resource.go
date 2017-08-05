// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package idl

// Resource is a marker struct to indicate that an IDL struct is a resource.
type Resource struct {
}

// NamedResource is a marker struct to indicate that an IDL struct is a named resource.
type NamedResource struct {
	// URNName is the logical name used to create a globally unique URN for an object.  It is mixed with the resource's
	// type, parent module, target deployment environment, and other information to help ensure that it is unique.
	URNName *string `lumi:"urnName,replaces,in"`
}
