#!/bin/bash

EVENT_TYPE=$1
VERSION=$2
SHA=$3

if [ -z "$EVENT_TYPE" ];
then
    echo "Must set event type"
    exit 1
fi

if [ -z "$VERSION" ];
then
    echo "Must set version"
    exit 1
fi

if [ -z "$GITHUB_TOKEN" ];
then
    echo "Must export GITHUB_TOKEN"
    exit 1
fi

payload=$(jq -n --arg version "$VERSION" --arg event_type "$EVENT_TYPE" --arg sha "${SHA}" '{"event_type": $event_type ,"client_payload": { "ref": $version , "sha": $sha } }')

echo $payload

curl -H "Accept: application/vnd.github.everest-preview+json" \
    -H "Authorization: token ${GITHUB_TOKEN}" \
    --request POST \
    --data "$payload" \
    https://api.github.com/repos/pulumi/docs/dispatches
