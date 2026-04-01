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

# run.sh — Pulumi Bash program executor
#
# This script is invoked by pulumi-language-bash to run user programs.
# It sets up the runtime environment, sources the user's program, and
# handles stack resource registration.

set -euo pipefail

# Source the Pulumi SDK.
if [ -n "${PULUMI_BASH_SDK_PATH:-}" ]; then
    source "${PULUMI_BASH_SDK_PATH}/lib/pulumi.sh"
else
    # Try relative to this script.
    script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    source "${script_dir}/../../pulumi.sh"
fi

# Resolve the program directory and entry point.
program_dir="${PULUMI_BASH_PROGRAM_DIRECTORY:-.}"
entry_point="${PULUMI_BASH_ENTRY_POINT:-.}"

# Add vendor directory's package lib directories to PATH-like lookup.
# Packages are extracted to vendor/<pkg_name>/ during InstallDependencies.
vendor_dir="${program_dir}/vendor"
if [ -d "$vendor_dir" ]; then
    for pkg_dir in "$vendor_dir"/*/; do
        if [ -f "${pkg_dir}index.sh" ]; then
            source "${pkg_dir}index.sh"
        fi
    done
fi

# Register the stack resource.
stack_name="${PULUMI_PROJECT}:${PULUMI_STACK}"
stack_result=$(_pulumi_bridge register-resource --component "pulumi:pulumi:Stack" "$stack_name" '{}')
stack_urn=$(echo "$stack_result" | jq -r '.urn')
export _PULUMI_ROOT_STACK_URN="$stack_urn"

# Resolve the program file.
if [ "$entry_point" = "." ]; then
    program_file="${program_dir}/Pulumi.sh"
    if [ ! -f "$program_file" ]; then
        program_file="${program_dir}/__main__.sh"
    fi
else
    program_file="${program_dir}/${entry_point}"
fi

if [ ! -f "$program_file" ]; then
    echo "error: no Pulumi program found at ${program_file}" >&2
    exit 1
fi

# Source the user's program.
source "$program_file"

# Register stack outputs.
# Pass outputs via stdin to avoid "Argument list too long" errors with large values.
_pulumi_get_exports | _pulumi_bridge register-outputs "$stack_urn" --stdin
