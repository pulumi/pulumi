// Copyright 2016 Marapongo, Inc. All rights reserved.

module "aws/cloudformation"

// Tag can be applied to a resource, helping to identify and categorize those resources.
schema Tag {
    // The key name of the tag. You can specify a value that is 1 to 127 Unicode characters in length and cannot be
    // prefixed with aws:. You can use any of the following characters: the set of Unicode letters, digits, whitespace,
    // _, ., /, =, +, and -.
    key: string
    // The value for the tag. You can specify a value that is 1 to 255 Unicode characters in length and cannot be
    // prefixed with aws:. You can use any of the following characters: the set of Unicode letters, digits, whitespace,
    // _, ., /, =, +, and -.
    value: string
}

// Tags is simply a collection of Tag values.
schema Tags = Tag[]

// TagSchema represents a base type for the common pattern of resources accepting tags and a name.
schema TagSchema {
    // An optional name for this resource.
    optional name: string
    // An arbitrary set of tags (key-value pairs) for this resource.
    optional tags: Tags
}

// ExpandTags takes a TagArgs and expands the "Name" key in-place, for naming, when present.
func ExpandTags(t: TagArgs) {
    if (t.name != null) {
        t.tags = append(t.tags, { key: "Name", value: t.name })
        ts.name = null
    }
}

