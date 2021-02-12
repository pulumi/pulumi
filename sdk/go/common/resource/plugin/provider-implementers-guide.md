# Resource Provider Implementer's Guide

## Provider Programming Model

### Data Types

The values exchanged between Pulumi resource providers and the Pulumi engine are a 
superset of the values expressible in JSON.

Pulumi supports the following data types:
- `Null`, which represents the lack of a value
- `Bool`, which represents a boolean value
- `Number`, which represents an IEEE-754 double-precision number
- `String`, which represents a sequence of UTF-8 encoded unicode code points
- `Array`, which represents a numbered sequence of values
- `Object`, which represents an unordered map from strings to values
- [`Asset`](#assets-and-archives), which represents a blob
- [`Archive`](#assets-and-archives), which represents a map from strings to `Asset`s or
  `Archive`s
- [`ResourceReference`](#resource-references), which represents a reference to a [Pulumi 
  resource](#resources)
- [`Unknown`](#unknowns), which represents a value whose type and concrete value are not 
  known
- [`Secret`](#secrets), which demarcates a value whose contents are sensitive

#### Assets and Archives

An `Asset` or `Archive` may contain either literal data or a reference to a file or URL.
In the former case, the literal data is a textual string or a map from strings to `Asset`s
or `Archive`s, respectively. In the latter case, the referenced file or URL is an opaque
blob or a TAR, gzipped TAR, or ZIP archive, respectively.

Each `Asset` or `Archive` also carries the SHA-256 hash of its contents. This hash can be
used to uniquely identify the asset (e.g. for locally caching `Asset` or `Archive`
contents).

#### Resource References

A `ResourceReference` represents a reference to a [Pulumi resource](#Resources). Although
all that is necessary to uniquely identify a resource is its URN, a `ResourceReference`
also carries the resource's ID (if it is a [custom resource](#custom-resources)) and the
version of the provider that manages the resource. If the contents of the referenced
resource must be inspected, the reference must be resolved by invoking the `getResource`
function of the engine's builtin provider. Note that this is only possible if there is a 
connection to the engine's resource monitor, e.g. within the scope of a call to `Construct`.
This implies that resource references may not be resolved within calls to other 
provider methods. Therefore, custom resources and provider functions should not rely on 
the ability to resolve resource references, and should instead treat resource references 
as either their ID (if present) or URN. If the ID is present and empty, it should be 
treated as an [`Unknown`](#unknowns).

#### Unknowns

An `Unknown` represents a value whose type and concrete value are not known. Resources
typically produce these values during [previews](#preview) for properties with values
that cannot be determined until the resource is actually created or updated.
[Functions](#functions) must not accept or return unknown values.

#### Secrets

A `Secret` represents a value whose contents are sensitive. Values of this type are 
merely wrappers around the sensitive value. A provider should take care not to leak a
secret value. and should wrap any resource output values that are always sensitive in a
`Secret`. [Functions](#functions) must not accept or return secret values.

### Resources

#### Custom Resources
- URN
- ID
- inputs
  - - difference between unchecked and checked inputs
- outputs
- associated lifecycle
- deployment unit

#### Component Resources
- URN, inputs, outputs
- resource monitor connection

### Functions
- simplified data model (no secrets / unknowns)

## Schema

- configuration
- types
- resources
- functions

## Provider Lifecycle

- load
- configure
- use
- shutdown

### Loading

- Probing process

### Configuration

- configuration variables
- replacement semantics

#### CheckConfig

- validate configuration
- apply provider-side defaults

#### DiffConfig

- determine differences from old configuration
- decide whether or not resources can still be managed (replacement)

#### Configure

- create clients, etc. etc.

### Shutdown

- cancel pending resource operations

## Resource Lifecycle

### Scenarios

- preview
- update
- import
- refresh
- destroy

#### Preview

- check
- diff
- create/update preview, read operation

#### Update

- check
- diff
- create/update/read/delete operation

#### Import

- read operation

#### Refresh

- read operations

#### Destroy

- delete operation

### Operations

#### Check

- validate inputs
- apply provider-side defaults

#### Diff

- determine differences between requested config and last state
- decide whether or not diffs require replacement
- detailed diff

#### Create

- create resource
- partial failures

#### Update

- update resource in-place
- partial failures

#### Read

- read live state given an ID

#### Delete

- delete resource

## Component Resources

### Construct

- user-level programming model

## Functions

### Invoke

## Out-of-Process Plugin Lifecycle

## gRPC Interface

- feature negotiation
- data representation
