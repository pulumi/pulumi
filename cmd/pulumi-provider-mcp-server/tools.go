// Copyright 2016-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// Tool handler functions for the MCP server

// handleConfigureProvider configures a new provider instance.
func handleConfigureProvider(ctx context.Context, session *Session, args map[string]any) (map[string]any, error) {
	pkg, ok := args["package"].(string)
	if !ok || pkg == "" {
		return nil, fmt.Errorf("package is required")
	}

	version := ""
	if v, ok := args["version"].(string); ok {
		version = v
	}

	config := make(map[string]any)
	if c, ok := args["config"].(map[string]any); ok {
		config = c
	}

	id := ""
	if i, ok := args["id"].(string); ok {
		id = i
	}

	// Add the provider to the session
	providerId, err := session.AddProvider(id, pkg, version, config)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"providerId": providerId,
	}, nil
}

// handleGetSchema retrieves the complete provider schema.
func handleGetSchema(ctx context.Context, session *Session, args map[string]any) (map[string]any, error) {
	providerId, ok := args["providerId"].(string)
	if !ok || providerId == "" {
		return nil, fmt.Errorf("providerId is required")
	}

	pkgSchema, err := session.GetSchema(providerId)
	if err != nil {
		return nil, err
	}

	schemaJSON, err := PackageSpecToJSON(pkgSchema)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"schema": schemaJSON,
	}, nil
}

// handleGetResourceSchema retrieves the schema for a specific resource type.
func handleGetResourceSchema(ctx context.Context, session *Session, args map[string]any) (map[string]any, error) {
	providerId, ok := args["providerId"].(string)
	if !ok || providerId == "" {
		return nil, fmt.Errorf("providerId is required")
	}

	typeToken, ok := args["type"].(string)
	if !ok || typeToken == "" {
		return nil, fmt.Errorf("type is required")
	}

	pkgSchema, err := session.GetSchema(providerId)
	if err != nil {
		return nil, err
	}

	resourceSchema, err := ExtractResourceSchema(pkgSchema, typeToken)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"resourceSchema": resourceSchema,
	}, nil
}

// handleGetFunctionSchema retrieves the schema for a specific function.
func handleGetFunctionSchema(ctx context.Context, session *Session, args map[string]any) (map[string]any, error) {
	providerId, ok := args["providerId"].(string)
	if !ok || providerId == "" {
		return nil, fmt.Errorf("providerId is required")
	}

	token, ok := args["token"].(string)
	if !ok || token == "" {
		return nil, fmt.Errorf("token is required")
	}

	pkgSchema, err := session.GetSchema(providerId)
	if err != nil {
		return nil, err
	}

	functionSchema, err := ExtractFunctionSchema(pkgSchema, token)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"functionSchema": functionSchema,
	}, nil
}

// handleCheck validates resource inputs.
func handleCheck(ctx context.Context, session *Session, args map[string]any) (map[string]any, error) {
	providerId, ok := args["providerId"].(string)
	if !ok || providerId == "" {
		return nil, fmt.Errorf("providerId is required")
	}

	urnStr, ok := args["urn"].(string)
	if !ok || urnStr == "" {
		return nil, fmt.Errorf("urn is required")
	}

	typeToken, ok := args["type"].(string)
	if !ok || typeToken == "" {
		return nil, fmt.Errorf("type is required")
	}

	inputs, ok := args["inputs"].(map[string]any)
	if !ok {
		inputs = make(map[string]any)
	}

	// Optional old inputs
	var oldInputs map[string]any
	if old, ok := args["oldInputs"].(map[string]any); ok {
		oldInputs = old
	}

	// Optional random seed
	randomSeed := ""
	if seed, ok := args["randomSeed"].(string); ok {
		randomSeed = seed
	}

	// Convert inputs to PropertyMap
	inputProps, err := JSONToPropertyMap(inputs)
	if err != nil {
		return nil, fmt.Errorf("failed to convert inputs: %w", err)
	}

	var oldInputProps resource.PropertyMap
	if oldInputs != nil {
		oldInputProps, err = JSONToPropertyMap(oldInputs)
		if err != nil {
			return nil, fmt.Errorf("failed to convert old inputs: %w", err)
		}
	}

	randomSeedBytes, err := RandomSeedToBytes(randomSeed)
	if err != nil {
		return nil, fmt.Errorf("failed to decode random seed: %w", err)
	}

	// Get the provider
	provider, err := session.GetProvider(providerId)
	if err != nil {
		return nil, err
	}

	// Call Check
	checkResp, err := provider.Check(ctx, plugin.CheckRequest{
		URN:        resource.URN(urnStr),
		News:       inputProps,
		Olds:       oldInputProps,
		RandomSeed: randomSeedBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("check failed: %w", err)
	}

	// Convert validated inputs back to JSON
	validatedInputs, err := PropertyMapToJSON(checkResp.Properties)
	if err != nil {
		return nil, fmt.Errorf("failed to convert validated inputs: %w", err)
	}

	// Convert failures
	failures := make([]map[string]any, len(checkResp.Failures))
	for i, f := range checkResp.Failures {
		failures[i] = map[string]any{
			"property": string(f.Property),
			"reason":   f.Reason,
		}
	}

	return map[string]any{
		"inputs":   validatedInputs,
		"failures": failures,
	}, nil
}

// handleDiff compares old and new resource properties.
func handleDiff(ctx context.Context, session *Session, args map[string]any) (map[string]any, error) {
	providerId, ok := args["providerId"].(string)
	if !ok || providerId == "" {
		return nil, fmt.Errorf("providerId is required")
	}

	urnStr, ok := args["urn"].(string)
	if !ok || urnStr == "" {
		return nil, fmt.Errorf("urn is required")
	}

	id, ok := args["id"].(string)
	if !ok || id == "" {
		return nil, fmt.Errorf("id is required")
	}

	typeToken, ok := args["type"].(string)
	if !ok || typeToken == "" {
		return nil, fmt.Errorf("type is required")
	}

	oldInputs, ok := args["oldInputs"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("oldInputs is required")
	}

	oldOutputs, ok := args["oldOutputs"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("oldOutputs is required")
	}

	newInputs, ok := args["newInputs"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("newInputs is required")
	}

	// Convert to PropertyMaps
	oldInputProps, err := JSONToPropertyMap(oldInputs)
	if err != nil {
		return nil, fmt.Errorf("failed to convert old inputs: %w", err)
	}

	oldOutputProps, err := JSONToPropertyMap(oldOutputs)
	if err != nil {
		return nil, fmt.Errorf("failed to convert old outputs: %w", err)
	}

	newInputProps, err := JSONToPropertyMap(newInputs)
	if err != nil {
		return nil, fmt.Errorf("failed to convert new inputs: %w", err)
	}

	// Get the provider
	provider, err := session.GetProvider(providerId)
	if err != nil {
		return nil, err
	}

	// Call Diff
	diffResp, err := provider.Diff(ctx, plugin.DiffRequest{
		URN:        resource.URN(urnStr),
		ID:         resource.ID(id),
		OldInputs:  oldInputProps,
		OldOutputs: oldOutputProps,
		NewInputs:  newInputProps,
	})
	if err != nil {
		return nil, fmt.Errorf("diff failed: %w", err)
	}

	// Convert changes
	var changes string
	switch diffResp.Changes {
	case plugin.DiffNone:
		changes = "DIFF_NONE"
	case plugin.DiffSome:
		changes = "DIFF_SOME"
	default:
		changes = "DIFF_UNKNOWN"
	}

	// Convert replaces
	replaces := make([]string, len(diffResp.ReplaceKeys))
	for i, key := range diffResp.ReplaceKeys {
		replaces[i] = string(key)
	}

	// Convert detailed diff
	detailedDiff := make(map[string]any)
	for path, diff := range diffResp.DetailedDiff {
		detailedDiff[path] = map[string]any{
			"kind":      diff.Kind.String(),
			"inputDiff": diff.InputDiff,
		}
	}

	return map[string]any{
		"changes":             changes,
		"replaces":            replaces,
		"deleteBeforeReplace": diffResp.DeleteBeforeReplace,
		"detailedDiff":        detailedDiff,
	}, nil
}

// handleCreate provisions a new resource.
func handleCreate(ctx context.Context, session *Session, args map[string]any) (map[string]any, error) {
	providerId, ok := args["providerId"].(string)
	if !ok || providerId == "" {
		return nil, fmt.Errorf("providerId is required")
	}

	urnStr, ok := args["urn"].(string)
	if !ok || urnStr == "" {
		return nil, fmt.Errorf("urn is required")
	}

	typeToken, ok := args["type"].(string)
	if !ok || typeToken == "" {
		return nil, fmt.Errorf("type is required")
	}

	inputs, ok := args["inputs"].(map[string]any)
	if !ok {
		inputs = make(map[string]any)
	}

	timeout := float64(300)
	if t, ok := args["timeout"].(float64); ok {
		timeout = t
	}

	preview := false
	if p, ok := args["preview"].(bool); ok {
		preview = p
	}

	// Convert inputs to PropertyMap
	inputProps, err := JSONToPropertyMap(inputs)
	if err != nil {
		return nil, fmt.Errorf("failed to convert inputs: %w", err)
	}

	// Get the provider
	provider, err := session.GetProvider(providerId)
	if err != nil {
		return nil, err
	}

	// Call Create
	createResp, err := provider.Create(ctx, plugin.CreateRequest{
		URN:        resource.URN(urnStr),
		Properties: inputProps,
		Timeout:    timeout,
		Preview:    preview,
	})
	if err != nil {
		return nil, fmt.Errorf("create failed: %w", err)
	}

	// Convert properties back to JSON
	properties, err := PropertyMapToJSON(createResp.Properties)
	if err != nil {
		return nil, fmt.Errorf("failed to convert properties: %w", err)
	}

	return map[string]any{
		"id":         string(createResp.ID),
		"properties": properties,
	}, nil
}

// handleRead reads the current live state of a resource.
func handleRead(ctx context.Context, session *Session, args map[string]any) (map[string]any, error) {
	providerId, ok := args["providerId"].(string)
	if !ok || providerId == "" {
		return nil, fmt.Errorf("providerId is required")
	}

	urnStr, ok := args["urn"].(string)
	if !ok || urnStr == "" {
		return nil, fmt.Errorf("urn is required")
	}

	id, ok := args["id"].(string)
	if !ok || id == "" {
		return nil, fmt.Errorf("id is required")
	}

	typeToken, ok := args["type"].(string)
	if !ok || typeToken == "" {
		return nil, fmt.Errorf("type is required")
	}

	// Optional inputs and properties
	var inputs, properties map[string]any
	if inp, ok := args["inputs"].(map[string]any); ok {
		inputs = inp
	}
	if props, ok := args["properties"].(map[string]any); ok {
		properties = props
	}

	// Convert to PropertyMaps
	var inputProps, outputProps resource.PropertyMap
	var err error
	if inputs != nil {
		inputProps, err = JSONToPropertyMap(inputs)
		if err != nil {
			return nil, fmt.Errorf("failed to convert inputs: %w", err)
		}
	}
	if properties != nil {
		outputProps, err = JSONToPropertyMap(properties)
		if err != nil {
			return nil, fmt.Errorf("failed to convert properties: %w", err)
		}
	}

	// Get the provider
	provider, err := session.GetProvider(providerId)
	if err != nil {
		return nil, err
	}

	// Call Read
	readResp, err := provider.Read(ctx, plugin.ReadRequest{
		URN:    resource.URN(urnStr),
		ID:     resource.ID(id),
		Inputs: inputProps,
		State:  outputProps,
	})
	if err != nil {
		return nil, fmt.Errorf("read failed: %w", err)
	}

	// Convert outputs back to JSON
	var resultProps, resultInputs map[string]any
	if readResp.Outputs.ContainsUnknowns() || readResp.Outputs.ContainsSecrets() || len(readResp.Outputs) > 0 {
		resultProps, err = PropertyMapToJSON(readResp.Outputs)
		if err != nil {
			return nil, fmt.Errorf("failed to convert outputs: %w", err)
		}
	}
	if readResp.Inputs.ContainsUnknowns() || readResp.Inputs.ContainsSecrets() || len(readResp.Inputs) > 0 {
		resultInputs, err = PropertyMapToJSON(readResp.Inputs)
		if err != nil {
			return nil, fmt.Errorf("failed to convert inputs: %w", err)
		}
	}

	result := map[string]any{
		"id": string(readResp.ID),
	}
	if resultProps != nil {
		result["properties"] = resultProps
	}
	if resultInputs != nil {
		result["inputs"] = resultInputs
	}

	return result, nil
}

// handleUpdate updates an existing resource.
func handleUpdate(ctx context.Context, session *Session, args map[string]any) (map[string]any, error) {
	providerId, ok := args["providerId"].(string)
	if !ok || providerId == "" {
		return nil, fmt.Errorf("providerId is required")
	}

	urnStr, ok := args["urn"].(string)
	if !ok || urnStr == "" {
		return nil, fmt.Errorf("urn is required")
	}

	id, ok := args["id"].(string)
	if !ok || id == "" {
		return nil, fmt.Errorf("id is required")
	}

	typeToken, ok := args["type"].(string)
	if !ok || typeToken == "" {
		return nil, fmt.Errorf("type is required")
	}

	oldInputs, ok := args["oldInputs"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("oldInputs is required")
	}

	oldOutputs, ok := args["oldOutputs"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("oldOutputs is required")
	}

	newInputs, ok := args["newInputs"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("newInputs is required")
	}

	timeout := float64(300)
	if t, ok := args["timeout"].(float64); ok {
		timeout = t
	}

	preview := false
	if p, ok := args["preview"].(bool); ok {
		preview = p
	}

	// Convert to PropertyMaps
	oldInputProps, err := JSONToPropertyMap(oldInputs)
	if err != nil {
		return nil, fmt.Errorf("failed to convert old inputs: %w", err)
	}

	oldOutputProps, err := JSONToPropertyMap(oldOutputs)
	if err != nil {
		return nil, fmt.Errorf("failed to convert old outputs: %w", err)
	}

	newInputProps, err := JSONToPropertyMap(newInputs)
	if err != nil {
		return nil, fmt.Errorf("failed to convert new inputs: %w", err)
	}

	// Get the provider
	provider, err := session.GetProvider(providerId)
	if err != nil {
		return nil, err
	}

	// Call Update
	updateResp, err := provider.Update(ctx, plugin.UpdateRequest{
		URN:        resource.URN(urnStr),
		ID:         resource.ID(id),
		OldInputs:  oldInputProps,
		OldOutputs: oldOutputProps,
		NewInputs:  newInputProps,
		Timeout:    timeout,
		Preview:    preview,
	})
	if err != nil {
		return nil, fmt.Errorf("update failed: %w", err)
	}

	// Convert properties back to JSON
	properties, err := PropertyMapToJSON(updateResp.Properties)
	if err != nil {
		return nil, fmt.Errorf("failed to convert properties: %w", err)
	}

	return map[string]any{
		"properties": properties,
	}, nil
}

// handleDelete deprovisions an existing resource.
func handleDelete(ctx context.Context, session *Session, args map[string]any) (map[string]any, error) {
	providerId, ok := args["providerId"].(string)
	if !ok || providerId == "" {
		return nil, fmt.Errorf("providerId is required")
	}

	urnStr, ok := args["urn"].(string)
	if !ok || urnStr == "" {
		return nil, fmt.Errorf("urn is required")
	}

	id, ok := args["id"].(string)
	if !ok || id == "" {
		return nil, fmt.Errorf("id is required")
	}

	typeToken, ok := args["type"].(string)
	if !ok || typeToken == "" {
		return nil, fmt.Errorf("type is required")
	}

	properties, ok := args["properties"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("properties is required")
	}

	timeout := float64(300)
	if t, ok := args["timeout"].(float64); ok {
		timeout = t
	}

	// Convert to PropertyMap
	propsMap, err := JSONToPropertyMap(properties)
	if err != nil {
		return nil, fmt.Errorf("failed to convert properties: %w", err)
	}

	// Get the provider
	provider, err := session.GetProvider(providerId)
	if err != nil {
		return nil, err
	}

	// Call Delete
	_, err = provider.Delete(ctx, plugin.DeleteRequest{
		URN:     resource.URN(urnStr),
		ID:      resource.ID(id),
		Outputs: propsMap,
		Timeout: timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("delete failed: %w", err)
	}

	return map[string]any{}, nil
}

// handleInvoke executes a provider function.
func handleInvoke(ctx context.Context, session *Session, args map[string]any) (map[string]any, error) {
	providerId, ok := args["providerId"].(string)
	if !ok || providerId == "" {
		return nil, fmt.Errorf("providerId is required")
	}

	token, ok := args["token"].(string)
	if !ok || token == "" {
		return nil, fmt.Errorf("token is required")
	}

	invokeArgs, ok := args["args"].(map[string]any)
	if !ok {
		invokeArgs = make(map[string]any)
	}

	// Convert args to PropertyMap
	argsProps, err := JSONToPropertyMap(invokeArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to convert args: %w", err)
	}

	// Get the provider
	provider, err := session.GetProvider(providerId)
	if err != nil {
		return nil, err
	}

	// Call Invoke
	invokeResp, err := provider.Invoke(ctx, plugin.InvokeRequest{
		Tok:  tokens.ModuleMember(token),
		Args: argsProps,
	})
	if err != nil {
		return nil, fmt.Errorf("invoke failed: %w", err)
	}

	// Convert return value back to JSON
	var returnVal map[string]any
	if invokeResp.Properties.ContainsUnknowns() || invokeResp.Properties.ContainsSecrets() || len(invokeResp.Properties) > 0 {
		returnVal, err = PropertyMapToJSON(invokeResp.Properties)
		if err != nil {
			return nil, fmt.Errorf("failed to convert return value: %w", err)
		}
	}

	// Convert failures
	failures := make([]map[string]any, len(invokeResp.Failures))
	for i, f := range invokeResp.Failures {
		failures[i] = map[string]any{
			"property": string(f.Property),
			"reason":   f.Reason,
		}
	}

	result := map[string]any{
		"failures": failures,
	}
	if returnVal != nil {
		result["return"] = returnVal
	}

	return result, nil
}
