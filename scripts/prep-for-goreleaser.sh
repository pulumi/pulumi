#!/bin/bash

pushd sdk
go mod tidy
go mod download
popd

pushd pkg 
go mod tidy
go mod download
popd

cp sdk/nodejs/dist/pulumi-resource-pulumi-nodejs .
cp sdk/nodejs/dist/pulumi-resource-pulumi-nodejs.cmd .
cp sdk/python/dist/pulumi-resource-pulumi-python .
cp sdk/python/dist/pulumi-resource-pulumi-python.cmd .
cp sdk/python/dist/pulumi-python3-shim.cmd .
cp sdk/python/dist/pulumi-python-shim.cmd .
cp sdk/nodejs/dist/pulumi-analyzer-policy .
cp sdk/nodejs/dist/pulumi-analyzer-policy.cmd .
cp sdk/python/dist/pulumi-analyzer-policy-python .
cp sdk/python/dist/pulumi-analyzer-policy-python.cmd .
cp sdk/python/cmd/pulumi-language-python-exec .
