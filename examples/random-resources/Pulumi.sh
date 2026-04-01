#!/bin/bash
# Random Resources — demonstrates the Pulumi Bash SDK with the random provider.
#
# This example creates random strings and passwords, shows how to use
# resource options, and chain resource outputs together.
#
# Prerequisites:
#   - jq must be installed
#   - pulumi-language-bash must be on PATH
#   - pulumi-resource-random must be installed (pulumi plugin install resource random)
#
# Usage:
#   pulumi stack init dev
#   pulumi config set petNameLength 3
#   pulumi up

# --- Configuration ---

pet_length=$(pulumi_config_get "petNameLength")
if [ -z "$pet_length" ]; then
    pet_length='3'
fi

# --- Resources ---

# Create a random pet name.
pet=$(pulumi_resource "random:index/randomPet:RandomPet" "my-pet" "$(jq -n \
    --argjson length "$pet_length" \
    '{length: $length}')")

pet_name=$(jq '.state.id' <<< "${pet}")

# Create a random password, protected from accidental deletion.
password=$(pulumi_resource "random:index/randomPassword:RandomPassword" "db-password" \
    '{"length": 32, "special": true}' \
    '{"protect": true, "additionalSecretOutputs": ["result"]}')

password_result=$(jq '.state.result' <<< "${password}")

# Create a random string that uses the pet name as a prefix.
prefixed=$(pulumi_resource "random:index/randomString:RandomString" "prefixed-id" "$(jq -n \
    --argjson prefix "$pet_name" \
    '{length: 8, prefix: $prefix}')")

# --- Outputs ---

pulumi_export "petName" "$pet_name"
pulumi_export "passwordLength" "$(jq '.state.length' <<< "${password}")"
pulumi_export "prefixedId" "$(jq '.state.result' <<< "${prefixed}")"
