# Pulumi Bash Examples

Example Pulumi programs written in Bash.

## Prerequisites

- [Pulumi CLI](https://www.pulumi.com/docs/install/)
- `pulumi-language-bash` binary on `PATH`
- `jq` installed
- `bash` 4.0+

## Examples

### [hello-world](./hello-world/)

A minimal program that reads config and exports stack outputs. No cloud provider needed.

```bash
cd hello-world
pulumi stack init dev
pulumi config set greeting "Hello"
pulumi config set name "World"
pulumi up
```

### [random-resources](./random-resources/)

Creates random resources (pet names, passwords, strings) using the `random` provider.
Demonstrates resource options, chaining outputs, and jq-based data manipulation.

```bash
cd random-resources
pulumi plugin install resource random
pulumi stack init dev
pulumi config set petNameLength 3
pulumi up
```

### [simple-resource](./simple-resource/)

Creates an S3 bucket and object using the `aws` provider. Demonstrates real cloud
resource creation with dependent resources.

```bash
cd simple-resource
pulumi stack init dev
pulumi config set name "my-bucket-name"
pulumi up
```

## How It Works

Pulumi Bash programs are regular shell scripts. The SDK provides functions that
communicate with the Pulumi engine through a bridge binary:

| Function | Description |
|----------|-------------|
| `pulumi_resource <type> <name> <inputs_json> [opts_json]` | Create a resource |
| `pulumi_export <name> <json_value>` | Export a stack output |
| `pulumi_config_get <key>` | Read a config value |
| `pulumi_invoke <token> <args_json> [opts_json]` | Call a provider function |
| `pulumi_log <severity> <message>` | Log to the engine |

All values are JSON. Use `jq` to construct inputs and extract outputs:

```bash
# Create a resource
result=$(pulumi_resource "pkg:mod:Resource" "name" '{"key": "value"}')

# Extract an output field
value=$(jq '.state.key' <<< "${result}")

# Build inputs from variables
inputs=$(jq -n --argjson v "$value" '{dep: $v}')

# Export an output
pulumi_export "myOutput" "$value"
```
