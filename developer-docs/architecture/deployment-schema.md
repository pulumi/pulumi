# Deployment Schema

## Pulumi Deployment States

A schema for Pulumi deployment states.

`object`

One of:

### Properties

---

#### `deployment` (_required_)

The deployment object.

`object`

---

#### `version` (_required_)

The deployment version.

`integer`

---

### Deployment Manifest

Captures meta-information about a deployment, such as versions of binaries, etc.

`object`

#### Properties

---

##### `magic` (_required_)

A magic number used to validate the manifest's integrity.

`string`

---

##### `plugins`

Information about the plugins used by the deployment.

`array`

Items: [Plugin Info](#plugin-info)

---

##### `time` (_required_)

The deployment's start time.

`string`

Format: `date-time`

---

##### `version` (_required_)

The version of the Pulumi engine that produced the deployment.

`string`

---

### Plugin Info

Information about a plugin.

`object`

#### Properties

---

##### `name` (_required_)

The plugin's name.

`string`

---

##### `path` (_required_)

The path of the plugin's binary.

`string`

---

##### `type` (_required_)

The plugin's type.

Enum: `"analyzer"` | `"language"` | `"resource"`

---

##### `version` (_required_)

The plugin's version.

`string`

---

### Resource Operation V2

Version 2 of a resource operation state

`object`

#### Properties

---

##### `resource` (_required_)

The state of the affected resource as of the start of this operation.

[Resource V3](https://github.com/pulumi/pulumi/blob/master/sdk/go/common/apitype/resources.json#/$defs/resourceV3)

---

##### `type` (_required_)

A string representation of the operation.

Enum: `"creating"` | `"updating"` | `"deleting"` | `"reading"`

---

### Secrets Provider

Configuration information for a secrets provider.

`object`

#### Properties

---

##### `state`

The secrets provider's state, if any.

---

##### `type` (_required_)

The secrets provider's type.

`string`

---

### Unknown Version

Catchall for unknown deployment versions.

`object`

#### Properties

---

##### `deployment`

The deployment object.

`object`

---

##### `version`

The deployment version.

---

### Version 3

The third version of the deployment state.

`object`

#### Properties

---

##### `deployment` (_required_)

The deployment state.

`object`

###### Properties

---

####### `manifest` (_required_)

Metadata about the deployment.

[Deployment Manifest](#deployment-manifest)

---

####### `pending_operations`

Any operations that were pending at the time the deployment finished.

`array`

Items: [Resource Operation V2](#resource-operation-v2)

---

####### `resources`

All resources that are part of the stack.

`array`

Items: [Resource V3](https://github.com/pulumi/pulumi/blob/master/sdk/go/common/apitype/resources.json#/$defs/resourceV3)

---

####### `secrets_providers`

Configuration for this stack's secrets provider.

[Secrets Provider](#secrets-provider)

---

---

##### `version` (_required_)

The deployment version. Must be `3`.

Constant: `3`

---

## Pulumi Property Value

A schema for Pulumi Property values.

One of:

### Archive property values

`object`

One of:

#### Properties

---

##### `4dabf18193072939515e22adb298388d` (_required_)

Archive signature

Constant: `"0def7320c3a5731c473e5ecbe6d01bc7"`

---

##### `hash`

The SHA256 hash of the archive's contents.

`string`

---

### Array property values

`array`

Items: [Pulumi Property Value](#pulumi-property-value)

### Asset property values

`object`

One of:

#### Properties

---

##### `4dabf18193072939515e22adb298388d` (_required_)

Asset signature

Constant: `"c44067f5952c0a294b673a41bacd8c17"`

---

##### `hash`

The SHA256 hash of the asset's contents.

`string`

---

### Decrypted Secret

`object`

#### Properties

---

##### `plaintext` (_required_)

The decrypted, JSON-serialized property value

`string`

---

### Encrypted Secret

`object`

#### Properties

---

##### `ciphertext` (_required_)

The encrypted, JSON-serialized property value

`string`

---

### Hash-only Archive

### Hash-only Asset

### Literal Archive

#### Properties

---

##### `assets` (_required_)

The literal contents of the archive.

`object`

Additional properties: [`https://github.com/pulumi/pulumi/blob/master/sdk/go/common/apitype/property-values.json#/oneOf/5/oneOf/1/properties/assets/additionalProperties`](#httpsgithubcompulumipulumiblobmastersdkgocommonapitypeproperty-valuesjsononeof5oneof1propertiesassetsadditionalproperties)

---

### Literal Asset

#### Properties

---

##### `text` (_required_)

The literal contents of the asset.

`string`

---

### Local File Archive

#### Properties

---

##### `path` (_required_)

The path to a local file that contains the archive's contents.

`string`

---

### Local File Asset

#### Properties

---

##### `path` (_required_)

The path to a local file that contains the asset's contents.

`string`

---

### Object property values

`object`

Additional properties: [Pulumi Property Value](#pulumi-property-value)

### Primitive property values

`null` | `boolean` | `number` | `string`

### Pulumi Property Value

A schema for Pulumi Property values.

One of:

### Resource reference property values

`object`

#### Properties

---

##### `4dabf18193072939515e22adb298388d` (_required_)

Resource reference signature

Constant: `"5cf8f73096256a8f31e491e813e4eb8e"`

---

##### `id`

The ID of the referenced resource.

`string`

---

##### `packageVersion`

The package version of the referenced resource.

`string`

---

##### `urn` (_required_)

The URN of the referenced resource.

`string`

---

### Secret Property Values

`object`

One of:

#### Properties

---

##### `4dabf18193072939515e22adb298388d` (_required_)

Secret signature

Constant: `"1b47061264138c4ac30d75fd1eb44270"`

---

### URI File Archive

#### Properties

---

##### `uri` (_required_)

The URI of a file that contains the archive's contents.

`string`

Format: `uri`

---

### URI File Asset

#### Properties

---

##### `uri` (_required_)

The URI of a file that contains the asset's contents.

`string`

Format: `uri`

---

### Unknown property values

Constant: `"04da6b54-80e4-46f7-96ec-b56ff0331ba9"`

### `https://github.com/pulumi/pulumi/blob/master/sdk/go/common/apitype/property-values.json#/oneOf/5/oneOf/1/properties/assets/additionalProperties`

One of:

## Pulumi Resource State

Schemas for Pulumi resource states.

One of:

### Resource V3

Version 3 of a Pulumi resource state.

`object`

#### Properties

---

##### `additionalSecretOutputs`

A list of outputs that were explicitly marked as secret when the resource was created.

`array`

Items: `string`

---

##### `aliases`

A list of previous URNs that this resource may have had in previous deployments

`array`

Items: [Unique Resource Name (URN)](#unique-resource-name-urn)

---

##### `custom`

True when the resource is managed by a plugin.

`boolean`

---

##### `customTimeouts`

A configuration block that can be used to control timeouts of CRUD operations

`object`

---

##### `delete`

True when the resource should be deleted during the next update.

`boolean`

---

##### `dependencies`

The dependency edges to other resources that this depends on.

`array`

Items: [Unique Resource Name (URN)](#unique-resource-name-urn)

---

##### `external`

True when the lifecycle of this resource is not managed by Pulumi.

`boolean`

---

##### `id`

The provider-assigned resource ID, if any, for custom resources.

`string`

---

##### `importID`

The import input used for imported resources.

`string`

---

##### `initErrors`

The set of errors encountered in the process of initializing resource (i.e. during create or update).

`array`

Items: `string`

---

##### `inputs`

The input properties supplied to the provider.

`object`

Additional properties: [Pulumi Property Value](https://github.com/pulumi/pulumi/blob/master/sdk/go/common/apitype/property-values.json#)

---

##### `outputs`

The output properties returned by the provider after provisioning.

`object`

Additional properties: [Pulumi Property Value](https://github.com/pulumi/pulumi/blob/master/sdk/go/common/apitype/property-values.json#)

---

##### `parent`

An optional parent URN if this resource is a child of it.

[Unique Resource Name (URN)](#unique-resource-name-urn)

---

##### `pendingReplacement`

Tracks delete-before-replace resources that have been deleted but not yet recreated.

`boolean`

---

##### `propertyDependencies`

A map from each input property name to the set of resources that property depends on.

`object`

Additional properties: [`https://github.com/pulumi/pulumi/blob/master/sdk/go/common/apitype/resources.json#/$defs/resourceV3/properties/propertyDependencies/additionalProperties`](#httpsgithubcompulumipulumiblobmastersdkgocommonapityperesourcesjsondefsresourcev3propertiespropertydependenciesadditionalproperties)

---

##### `protect`

True when this resource is "protected" and may not be deleted.

`boolean`

---

##### `provider`

A reference to the provider that is associated with this resource.

`string`

---

##### `type`

The resource's full type token.

`string`

---

##### `urn` (_required_)

The resource's unique name.

[Unique Resource Name (URN)](#unique-resource-name-urn)

---

### Unique Resource Name (URN)

The unique name for a resource in a Pulumi stack.

`string`

### `https://github.com/pulumi/pulumi/blob/master/sdk/go/common/apitype/resources.json#/$defs/resourceV3/properties/propertyDependencies/additionalProperties`

`array`

Items: [Unique Resource Name (URN)](#unique-resource-name-urn)
