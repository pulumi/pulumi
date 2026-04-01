#!/bin/bash
# A simple Pulumi program written in Bash.
#
# This example demonstrates:
# - Reading configuration values
# - Creating resources with inputs
# - Accessing resource outputs
# - Exporting stack outputs
#
# Prerequisites:
#   - jq must be installed
#   - pulumi-language-bash must be on PATH
#
# Usage:
#   pulumi config set name "my-bucket"
#   pulumi up

# Read configuration.
name=$(pulumi_config_get "name" || echo '"my-resource"')

# Create a simple resource.
# pulumi_resource returns JSON: {"urn": "...", "id": "...", "state": {...}}
bucket=$(pulumi_resource "aws:s3:Bucket" "my-bucket" "$(jq -n --argjson name "$name" '{
    bucket: $name
}')")

# Extract outputs from the resource result using jq.
bucket_name=$(jq '.state.bucket' <<< "${bucket}")
bucket_arn=$(jq '.state.arn' <<< "${bucket}")

# Create a second resource that depends on the first.
object=$(pulumi_resource "aws:s3:BucketObject" "my-object" "$(jq -n \
    --argjson bucket "$bucket_name" \
    --arg key "index.html" \
    --arg content "<h1>Hello from Pulumi Bash!</h1>" \
    '{bucket: $bucket, key: $key, contentType: "text/html", content: $content}')")

# Export stack outputs.
pulumi_export "bucketName" "$bucket_name"
pulumi_export "bucketArn" "$bucket_arn"
pulumi_export "objectKey" "$(jq '.state.key' <<< "${object}")"
