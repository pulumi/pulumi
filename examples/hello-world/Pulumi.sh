#!/bin/bash
# Hello World — a minimal Pulumi Bash program.
#
# This program doesn't create any cloud resources.
# It just demonstrates stack outputs and configuration.
#
# Usage:
#   pulumi stack init dev
#   pulumi config set greeting "Hello"
#   pulumi config set name "World"
#   pulumi up

# Read config values (with defaults).
greeting=$(pulumi_config_get "greeting")
if [ -z "$greeting" ]; then
    greeting='"Hello"'
fi

name=$(pulumi_config_get "name")
if [ -z "$name" ]; then
    name='"World"'
fi

# Build a greeting message using jq.
message=$(jq -n --argjson g "$greeting" --argjson n "$name" '$g + ", " + $n + "!"')

# Export outputs.
pulumi_export "greeting" "$greeting"
pulumi_export "name" "$name"
pulumi_export "message" "$message"
