# Resource Provider Implementer's Guide

## Provider Programming Model

### Data Types

- primitive types
	- null
	- bool
	- number
	- string
	- asset
	- archive
	- resource reference
	- unknown
- compound types
	- array
	- object
	- secret

### Custom Resources
- URN
- ID
- inputs
	- difference between unchecked and checked inputs
- outputs
- associated lifecycle
- deployment unit

### Component Resources
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
