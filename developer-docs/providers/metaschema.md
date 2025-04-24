
# Pulumi Package Metaschema

A description of the schema for a Pulumi Package

`object`

## Properties

---

### `attribution`

Freeform text attribution of derived work, if required.

`string`

---

### `config`

The package's configuration variables.

`object`

#### Properties

---

##### `required`

A list of the names of the package's required configuration variables.

`array`

Items: `string`

---

##### `variables`

A map from variable name to propertySpec that describes a package's configuration variables.

`object`

Additional properties: [Property Definition](#property-definition)

---

---

### `description`

The description of the package. Descriptions are interpreted as Markdown.

`string`

---

### `displayName`

The human-friendly name of the package.

`string`

---

### `functions`

A map from token to functionSpec that describes the set of functions defined by this package.

`object`

Property names: [Token](#token)

Additional properties: [Function Definition](#function-definition)

---

### `homepage`

The package's homepage.

`string`

---

### `keywords`

The list of keywords that are associated with the package, if any.

`array`

Items: `string`

---

### `language`

Additional language-specific data about the package.

`object`

---

### `license`

The name of the license used for the package's contents.

`string`

---

### `logoUrl`

The URL of the package's logo, if any.

`string`

---

### `meta`

Format metadata about this package.

`object`

#### Properties

---

##### `moduleFormat` (_required_)

A regex that is used by the importer to extract a module name from the module portion of a type token. Packages that use the module format "namespace1/namespace2/.../namespaceN" do not need to specify a format. The regex must define one capturing group that contains the module name, which must be formatted as "namespace1/namespace2/...namespaceN".

`string`

Format: `regex`

---

---

### `name` (_required_)

The unqualified name of the package (e.g. "aws", "azure", "gcp", "kubernetes", "random")

`string`

Pattern: `^[a-zA-Z][-a-zA-Z0-9_]*$`

---

### `pluginDownloadUrl`

The URL to use when downloading the provider plugin binary.

`string`

---

### `provider`

The provider type for this package.

[Resource Definition](#resource-definition)

---

### `publisher`

The name of the person or organization that authored and published the package.

`string`

---

### `repository`

The URL at which the package's sources can be found.

`string`

---

### `resources`

A map from type token to resourceSpec that describes the set of resources and components defined by this package.

`object`

Property names: [Token](#token)

Additional properties: [Resource Definition](#resource-definition)

---

### `types`

A map from type token to complexTypeSpec that describes the set of complex types (i.e. object, enum) defined by this package.

`object`

Property names: [Token](#token)

Additional properties: [Type Definition](#type-definition)

---

### `version`

The version of the package. The version must be valid semver.

`string`

Pattern: `^v?(?P<major>0|[1-9]\d*)\.(?P<minor>0|[1-9]\d*)\.(?P<patch>0|[1-9]\d*)(?:-(?P<prerelease>(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+(?P<buildmetadata>[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`

---

## Alias Definition

`object`

### Properties

---

#### `name`

The name portion of the alias, if any

`string`

---

#### `project`

The project portion of the alias, if any

`string`

---

#### `type`

The type portion of the alias, if any

`string`

---

## Array Type

A reference to an array type. The "type" property must be set to "array" and the "items" property must be present. No other properties may be present.

`object`

### Properties

---

#### `items` (_required_)

The element type of the array

[Type Reference](#type-reference)

---

#### `type` (_required_)

Constant: `"array"`

---

## Enum Type Definition

Describes an enum type

`object`

### Properties

---

#### `enum` (_required_)

The list of possible values for the enum

`array`

Items: [Enum Value Definition](#enum-value-definition)

---

#### `type` (_required_)

The underlying primitive type of the enum

`string`

Enum: `"boolean"` | `"integer"` | `"number"` | `"string"`

---

## Enum Value Definition

`object`

### Properties

---

#### `deprecationMessage`

Indicates whether the value is deprecated.

`string`

---

#### `description`

The description of the enum value, if any. Interpreted as Markdown.

`string`

---

#### `name`

If present, overrides the name of the enum value that would usually be derived from the value.

`string`

---

#### `value` (_required_)

The enum value itself

`boolean` | `integer` | `number` | `string`

---

## Function Definition

Describes a function.

`object`

### Properties

---

#### `deprecationMessage`

Indicates whether the function is deprecated

`string`

---

#### `description`

The description of the function, if any. Interpreted as Markdown.

`string`

---

#### `inputs`

The bag of input values for the function, if any.

[Object Type Details](#object-type-details)

---

#### `isOverlay`

Indicates that the implementation of the function should not be generated from the schema, and is instead provided out-of-band by the package author

`boolean`

---

#### `language`

Additional language-specific data about the function.

`object`

---

#### `outputs`

The bag of output values for the function, if any.

[Object Type Details](#object-type-details)

---

## Map Type

A reference to a map type. The "type" property must be set to "object" and the "additionalProperties" property may be present. No other properties may be present.

`object`

### Properties

---

#### `additionalProperties`

The element type of the map. Defaults to "string" when omitted.

[Type Reference](#type-reference)

---

#### `type` (_required_)

Constant: `"object"`

---

## Named Type

A reference to a type in this or another document. The "$ref" property must be present. The "type" property is ignored if it is present. No other properties may be present.

`object`

### Properties

---

#### `$ref` (_required_)

The URI of the referenced type. For example, the built-in Archive, Asset, and Any
types are referenced as "pulumi.json#/Archive", "pulumi.json#/Asset", and "pulumi.json#/Any", respectively.
A type from this document is referenced as "#/types/pulumi:type:token".
A type from another document is referenced as "path#/types/pulumi:type:token", where path is of the form:
  "/provider/vX.Y.Z/schema.json" or "pulumi.json" or "http[s]://example.com/provider/vX.Y.Z/schema.json"
A resource from this document is referenced as "#/resources/pulumi:type:token".
A resource from another document is referenced as "path#/resources/pulumi:type:token", where path is of the form:
  "/provider/vX.Y.Z/schema.json" or "pulumi.json" or "http[s]://example.com/provider/vX.Y.Z/schema.json"

`string`

Format: `uri-reference`

---

#### `type`

ignored; present for compatibility with existing schemas

`string`

---

## Object Type Definition

`object`

All of:
- [Object Type Details](#object-type-details)

### Properties

---

#### `type`

Constant: `"object"`

---

## Object Type Details

Describes an object type

`object`

### Properties

---

#### `properties`

A map from property name to propertySpec that describes the object's properties.

`object`

Additional properties: [Property Definition](#property-definition)

---

#### `required`

A list of the names of an object type's required properties. These properties must be set for inputs and will always be set for outputs.

`array`

Items: `string`

---

## Primitive Type

A reference to a primitive type. A primitive type must have only the "type" property set.

`object`

### Properties

---

#### `type` (_required_)

The primitive type, if any

`string`

Enum: `"boolean"` | `"integer"` | `"number"` | `"string"`

---

## Property Definition

Describes an object or resource property

`object`

All of:
- [Type Reference](#type-reference)

### Properties

---

#### `const`

The constant value for the property, if any. The type of the value must be assignable to the type of the property.

`boolean` | `number` | `string`

---

#### `default`

The default value for the property, if any. The type of the value must be assignable to the type of the property.

`boolean` | `number` | `string`

---

#### `defaultInfo`

Additional information about the property's default value, if any.

`object`

##### Properties

---

###### `environment` (_required_)

A set of environment variables to probe for a default value.

`array`

Items: `string`

---

###### `language`

Additional language-specific data about the default value.

`object`

---

---

#### `deprecationMessage`

Indicates whether the property is deprecated

`string`

---

#### `description`

The description of the property, if any. Interpreted as Markdown.

`string`

---

#### `language`

Additional language-specific data about the property.

`object`

---

#### `replaceOnChanges`

Specifies whether a change to the property causes its containing resource to be replaced instead of updated (default false).

`boolean`

---

#### `willReplaceOnChanges`

Indicates that the provider will replace the resource when this property is changed.

`boolean`

---

#### `secret`

Specifies whether the property is secret (default false).

`boolean`

---

## Resource Definition

Describes a resource or component.

`object`

All of:
- [Object Type Details](#object-type-details)

### Properties

---

#### `aliases`

The list of aliases for the resource.

`array`

Items: [Alias Definition](#alias-definition)

---

#### `deprecationMessage`

Indicates whether the resource is deprecated

`string`

---

#### `description`

The description of the resource, if any. Interpreted as Markdown.

`string`

---

#### `inputProperties`

A map from property name to propertySpec that describes the resource's input properties.

`object`

Additional properties: [Property Definition](#property-definition)

---

#### `isComponent`

Indicates whether the resource is a component.

`boolean`

---

#### `isOverlay`

Indicates that the implementation of the resource should not be generated from the schema, and is instead provided out-of-band by the package author

`boolean`

---

#### `overlaySupportedLanguages`

Indicates what languages the overlay supports. This only has an effect if the Resource is an Overlay (IsOverlay == true).
Supported values are "nodejs", "python", "go", "csharp", "java", "yaml".

`array`

Items: `string`

---

#### `methods`

A map from method name to function token that describes the resource's method set.

`object`

Additional properties: `string`

---

#### `requiredInputs`

A list of the names of the resource's required input properties.

`array`

Items: `string`

---

#### `stateInputs`

An optional objectTypeSpec that describes additional inputs that mau be necessary to get an existing resource. If this is unset, only an ID is necessary.

[Object Type Details](#object-type-details)

---

## Token

`string`

Pattern: `^[a-zA-Z][-a-zA-Z0-9_]*:([^0-9][a-zA-Z0-9._/-]*)?:[^0-9][a-zA-Z0-9._/]*$`

## Type Definition

Describes an object or enum type.

`object`

One of:

### Properties

---

#### `description`

The description of the type, if any. Interpreted as Markdown.

`string`

---

#### `isOverlay`

Indicates that the implementation of the type should not be generated from the schema, and is instead provided out-of-band by the package author

`boolean`

---

#### `language`

Additional language-specific data about the type.

`object`

---

## Type Reference

A reference to a type. The particular kind of type referenced is determined based on the contents of the "type" property and the presence or absence of the "additionalProperties", "items", "oneOf", and "$ref" properties.

`object`

One of:

### Properties

---

#### `plain`

Indicates that when used as an input, this type does not accept eventual values.

`boolean`

---

## Union Type

A reference to a union type. The "oneOf" property must be present. The union may additional specify an underlying primitive type via the "type" property and a discriminator via the "discriminator" property. No other properties may be present.

`object`

### Properties

---

#### `discriminator`

Informs the consumer of an alternative schema based on the value associated with it

`object`

##### Properties

---

###### `mapping`

an optional object to hold mappings between payload values and schema names or references

`object`

Additional properties: `string`

---

###### `propertyName` (_required_)

PropertyName is the name of the property in the payload that will hold the discriminator value

`string`

---

---

#### `oneOf` (_required_)

If present, indicates that values of the type may be one of any of the listed types

`array`

Items: [Type Reference](#type-reference)

---

#### `type`

The underlying primitive type of the union, if any

`string`

Enum: `"boolean"` | `"integer"` | `"number"` | `"string"`

---
