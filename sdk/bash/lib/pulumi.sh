#!/bin/bash
# Copyright 2024-2025, Pulumi Corporation.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# pulumi.sh — Core Pulumi SDK for Bash
#
# This file provides shell functions for interacting with the Pulumi engine
# through the bridge subcommand of pulumi-language-bash.
#
# Required: jq must be installed and available on PATH.

# Verify jq is available.
if ! command -v jq &>/dev/null; then
    echo "error: pulumi-language-bash requires 'jq' to be installed" >&2
    exit 1
fi

# _PULUMI_EXPORTS accumulates stack outputs as a JSON object.
_PULUMI_EXPORTS='{}'

# _pulumi_bridge calls the bridge subcommand.
_pulumi_bridge() {
    "$PULUMI_BASH_BRIDGE_CMD" bridge "$@"
}

# pulumi_resource registers a resource with the Pulumi engine.
#
# Usage: result=$(pulumi_resource <type> <name> <inputs_json> [opts_json])
#
# The function returns a JSON object with "urn", "id", and "state" fields.
# Use jq to extract specific values:
#   urn=$(echo "$result" | jq -r '.urn')
#   value=$(echo "$result" | jq -r '.state.value')
pulumi_resource() {
    local type="$1"
    local name="$2"
    local inputs="${3:-"{}"}"
    local opts="${4:-}"

    if [ -n "$opts" ]; then
        _pulumi_bridge register-resource --custom "$type" "$name" "$inputs" "$opts"
    else
        _pulumi_bridge register-resource --custom "$type" "$name" "$inputs"
    fi
}

# pulumi_component registers a component resource with the Pulumi engine.
#
# Usage: result=$(pulumi_component <type> <name> [opts_json])
pulumi_component() {
    local type="$1"
    local name="$2"
    local opts="${3:-}"

    if [ -n "$opts" ]; then
        _pulumi_bridge register-resource --component "$type" "$name" '{}' "$opts"
    else
        _pulumi_bridge register-resource --component "$type" "$name" '{}'
    fi
}

# pulumi_register_outputs registers outputs for a resource.
#
# Usage: pulumi_register_outputs <urn> <outputs_json>
pulumi_register_outputs() {
    local urn="$1"
    local outputs="$2"
    _pulumi_bridge register-outputs "$urn" "$outputs"
}

# pulumi_export exports a value as a stack output.
#
# Usage: pulumi_export <name> <json_value>
#
# Example:
#   pulumi_export "bucketName" '"my-bucket"'
#   pulumi_export "count" '42'
pulumi_export() {
    local name="$1"
    local value="$2"
    # Use temp files to avoid "Argument list too long" errors with large values.
    # Here-strings (<<<) bypass ARG_MAX since bash handles them via pipes/redirects.
    local _tmpval _tmpexports
    _tmpval=$(mktemp)
    _tmpexports=$(mktemp)
    cat > "$_tmpval" <<< "$value"
    cat > "$_tmpexports" <<< "$_PULUMI_EXPORTS"
    _PULUMI_EXPORTS=$(jq --arg k "$name" --slurpfile v "$_tmpval" '. + {($k): $v[0]}' "$_tmpexports")
    rm -f "$_tmpval" "$_tmpexports"
}

# _pulumi_get_exports returns the accumulated stack outputs as a JSON string.
_pulumi_get_exports() {
    echo "$_PULUMI_EXPORTS"
}

# pulumi_config_get reads a configuration value.
#
# Usage: value=$(pulumi_config_get <key>)
#
# Returns the value from PULUMI_CONFIG for the given key.
# If the key contains a colon, it is used as-is. Otherwise, the project
# name is prepended (e.g., "key" becomes "project:key").
pulumi_config_get() {
    local key="$1"

    # If the key doesn't contain a colon, prepend the project name.
    if [[ "$key" != *:* ]]; then
        key="${PULUMI_PROJECT}:${key}"
    fi

    echo "$PULUMI_CONFIG" | jq -r --arg k "$key" '.[$k] // empty'
}

# pulumi_invoke calls a provider function.
#
# Usage: result=$(pulumi_invoke <token> <args_json> [opts_json])
pulumi_invoke() {
    local token="$1"
    local _empty_obj='{}'
    local args="${2:-$_empty_obj}"
    local opts="${3:-}"

    if [ -n "$opts" ]; then
        _pulumi_bridge invoke "$token" "$args" "$opts"
    else
        _pulumi_bridge invoke "$token" "$args"
    fi
}

# pulumi_log logs a message to the Pulumi engine.
#
# Usage: pulumi_log <severity> <message>
#
# Severity: debug, info, warning, error
pulumi_log() {
    local severity="$1"
    local message="$2"
    _pulumi_bridge log "$severity" "$message"
}

# pulumi_supports_feature checks if the engine supports a feature.
#
# Usage: if pulumi_supports_feature <feature>; then ...; fi
pulumi_supports_feature() {
    local feature="$1"
    _pulumi_bridge supports-feature "$feature"
}
