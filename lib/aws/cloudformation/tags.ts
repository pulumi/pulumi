// Copyright 2017 Pulumi, Inc. All rights reserved.

// Tag can be applied to a resource, helping to identify and categorize those resources.
export interface Tag {
    // The key name of the tag. You can specify a value that is 1 to 127 Unicode characters in length and cannot be
    // prefixed with aws:. You can use any of the following characters: the set of Unicode letters, digits, whitespace,
    // _, ., /, =, +, and -.
    key: string;
    // The value for the tag. You can specify a value that is 1 to 255 Unicode characters in length and cannot be
    // prefixed with aws:. You can use any of the following characters: the set of Unicode letters, digits, whitespace,
    // _, ., /, =, +, and -.
    value: string;
}

// Tags is simply a collection of Tag values.
export type Tags = Tag[];

// TagArgs represents a base type for the common pattern of resources accepting tags and a name.
export interface TagArgs {
    // An arbitrary set of tags (key-value pairs) for this resource.
    tags?: Tags;
}

