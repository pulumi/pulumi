package schema

import schema "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/schema"

func NewCachedLoader(loader ReferenceLoader) ReferenceLoader {
	return schema.NewCachedLoader(loader)
}

// NewCachedLoaderWithEntries creates a new cached loader with the passed in entries pre-loaded.
func NewCachedLoaderWithEntries(loader ReferenceLoader, entries map[string]PackageReference) ReferenceLoader {
	return schema.NewCachedLoaderWithEntries(loader, entries)
}

