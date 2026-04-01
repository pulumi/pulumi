// Copyright 2024-2025, Pulumi Corporation.
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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"
)

// runBridge handles the "bridge" subcommand, which provides a CLI-to-gRPC
// translation layer for the Bash SDK. Each invocation creates a new gRPC
// connection, performs the requested operation, and exits.
func runBridge(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("bridge: missing subcommand")
	}

	subcmd := args[0]
	subargs := args[1:]

	switch subcmd {
	case "register-resource":
		return bridgeRegisterResource(subargs)
	case "register-outputs":
		return bridgeRegisterOutputs(subargs)
	case "invoke":
		return bridgeInvoke(subargs)
	case "log":
		return bridgeLog(subargs)
	case "supports-feature":
		return bridgeSupportsFeature(subargs)
	case "check-version":
		return bridgeCheckVersion(subargs)
	default:
		return fmt.Errorf("bridge: unknown subcommand %q", subcmd)
	}
}

// connectMonitor creates a gRPC connection to the resource monitor.
func connectMonitor() (pulumirpc.ResourceMonitorClient, *grpc.ClientConn, error) {
	addr := os.Getenv("PULUMI_MONITOR_ADDRESS")
	if addr == "" {
		return nil, nil, fmt.Errorf("PULUMI_MONITOR_ADDRESS not set")
	}

	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(rpcutil.OpenTracingClientInterceptor()),
		grpc.WithStreamInterceptor(rpcutil.OpenTracingStreamClientInterceptor()),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to monitor: %w", err)
	}

	return pulumirpc.NewResourceMonitorClient(conn), conn, nil
}

// connectEngine creates a gRPC connection to the engine.
func connectEngine() (pulumirpc.EngineClient, *grpc.ClientConn, error) {
	addr := os.Getenv("PULUMI_ENGINE_ADDRESS")
	if addr == "" {
		return nil, nil, fmt.Errorf("PULUMI_ENGINE_ADDRESS not set")
	}

	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(rpcutil.OpenTracingClientInterceptor()),
		grpc.WithStreamInterceptor(rpcutil.OpenTracingStreamClientInterceptor()),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to engine: %w", err)
	}

	return pulumirpc.NewEngineClient(conn), conn, nil
}

// jsonToStruct converts a JSON string to a protobuf Struct.
func jsonToStruct(jsonStr string) (*structpb.Struct, error) {
	if jsonStr == "" || jsonStr == "{}" {
		return &structpb.Struct{Fields: map[string]*structpb.Value{}}, nil
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	return structpb.NewStruct(m)
}

// structToJSON converts a protobuf Struct to a JSON string.
func structToJSON(s *structpb.Struct) string {
	if s == nil {
		return "{}"
	}

	m := s.AsMap()
	data, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// bridgeRegisterResource handles the "register-resource" bridge subcommand.
//
// Usage: bridge register-resource [--custom|--component] <type> <name> <inputs_json> [opts_json]
func bridgeRegisterResource(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("register-resource: need at least type, name, inputs")
	}

	// Parse flags.
	custom := false
	component := false
	idx := 0
	for idx < len(args) && strings.HasPrefix(args[idx], "--") {
		switch args[idx] {
		case "--custom":
			custom = true
		case "--component":
			component = true
		default:
			return fmt.Errorf("register-resource: unknown flag %q", args[idx])
		}
		idx++
	}

	remaining := args[idx:]
	if len(remaining) < 3 {
		return fmt.Errorf("register-resource: need type, name, inputs after flags")
	}

	typeName := remaining[0]
	name := remaining[1]
	inputsJSON := remaining[2]
	optsJSON := ""
	if len(remaining) > 3 {
		optsJSON = remaining[3]
	}

	// If neither custom nor component was specified, default to custom.
	if !custom && !component {
		custom = true
	}

	inputs, err := jsonToStruct(inputsJSON)
	if err != nil {
		return fmt.Errorf("register-resource: parse inputs: %w", err)
	}

	// Build the request.
	req := &pulumirpc.RegisterResourceRequest{
		Type:                    typeName,
		Name:                    name,
		Object:                  inputs,
		Custom:                  custom,
		AcceptSecrets:           true,
		SupportsResultReporting: true,
		SupportsPartialValues:   true,
	}

	// Parse options if provided.
	if optsJSON != "" {
		if err := applyResourceOptions(req, optsJSON); err != nil {
			return fmt.Errorf("register-resource: parse options: %w", err)
		}
	}

	// If no parent was set by options, default to the root stack URN.
	if req.Parent == "" {
		if rootStackURN := os.Getenv("_PULUMI_ROOT_STACK_URN"); rootStackURN != "" {
			req.Parent = rootStackURN
		}
	}

	monitor, conn, err := connectMonitor()
	if err != nil {
		return err
	}
	defer conn.Close()

	resp, err := monitor.RegisterResource(context.Background(), req)
	if err != nil {
		return fmt.Errorf("register-resource: %w", err)
	}

	// Build result JSON.
	stateMap := structToMap(resp.Object)
	result := map[string]any{
		"urn":   resp.Urn,
		"id":    resp.Id,
		"state": stateMap,
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("register-resource: marshal result: %w", err)
	}

	fmt.Print(string(data))
	return nil
}

// unknownSentinel is the well-known sentinel string used in the Pulumi protocol
// to represent unknown/computed string values during preview.
const unknownSentinel = "04da6b54-80e4-46f7-96ec-b56ff0331ba9"

// structToMap converts a protobuf Struct to a Go map, or empty map if nil.
// During preview (dry run), null values are converted to the unknown sentinel
// string, matching the behavior of other SDKs that treat null outputs as
// "not yet known" during preview.
func structToMap(s *structpb.Struct) map[string]any {
	if s == nil {
		return map[string]any{}
	}

	isDryRun := os.Getenv("PULUMI_DRY_RUN") == "true"
	if !isDryRun {
		return s.AsMap()
	}

	// During preview, walk the struct and convert null values to the
	// unknown sentinel string so that downstream code (e.g. replacement
	// triggers) can recognize them as unknown.
	result := make(map[string]any, len(s.Fields))
	for k, v := range s.Fields {
		result[k] = valueToAnyPreview(v)
	}
	return result
}

// valueToAnyPreview converts a protobuf Value to a Go value, replacing null
// values with the unknown sentinel string. This is used during preview to
// preserve unknown semantics for values that the engine serializes as null
// (e.g., Computed{} with untyped elements).
func valueToAnyPreview(v *structpb.Value) any {
	if v == nil {
		return unknownSentinel
	}
	switch kind := v.Kind.(type) {
	case *structpb.Value_NullValue:
		return unknownSentinel
	case *structpb.Value_BoolValue:
		return kind.BoolValue
	case *structpb.Value_NumberValue:
		return kind.NumberValue
	case *structpb.Value_StringValue:
		return kind.StringValue
	case *structpb.Value_StructValue:
		m := make(map[string]any, len(kind.StructValue.Fields))
		for k, fv := range kind.StructValue.Fields {
			m[k] = valueToAnyPreview(fv)
		}
		return m
	case *structpb.Value_ListValue:
		arr := make([]any, len(kind.ListValue.Values))
		for i, lv := range kind.ListValue.Values {
			arr[i] = valueToAnyPreview(lv)
		}
		return arr
	default:
		return unknownSentinel
	}
}

// applyResourceOptions parses an options JSON string and applies it to a RegisterResourceRequest.
func applyResourceOptions(req *pulumirpc.RegisterResourceRequest, optsJSON string) error {
	var opts map[string]any
	if err := json.Unmarshal([]byte(optsJSON), &opts); err != nil {
		return err
	}

	if parent, ok := opts["parent"].(string); ok {
		req.Parent = parent
	}
	if provider, ok := opts["provider"].(string); ok {
		req.Provider = provider
	}
	if protect, ok := opts["protect"].(bool); ok {
		req.Protect = &protect
	}
	if v, ok := opts["version"].(string); ok {
		req.Version = v
	}
	if v, ok := opts["pluginDownloadURL"].(string); ok {
		req.PluginDownloadURL = v
	}
	if v, ok := opts["deleteBeforeReplace"].(bool); ok {
		req.DeleteBeforeReplace = v
		req.DeleteBeforeReplaceDefined = true
	}
	if v, ok := opts["retainOnDelete"].(bool); ok {
		req.RetainOnDelete = &v
	}
	if v, ok := opts["import"].(string); ok {
		req.ImportId = v
	}

	if deps, ok := opts["dependsOn"].([]any); ok {
		for _, d := range deps {
			if s, ok := d.(string); ok {
				req.Dependencies = append(req.Dependencies, s)
			}
		}
	}

	if secrets, ok := opts["additionalSecretOutputs"].([]any); ok {
		for _, s := range secrets {
			if str, ok := s.(string); ok {
				req.AdditionalSecretOutputs = append(req.AdditionalSecretOutputs, str)
			}
		}
	}

	if ignoreChanges, ok := opts["ignoreChanges"].([]any); ok {
		for _, s := range ignoreChanges {
			if str, ok := s.(string); ok {
				req.IgnoreChanges = append(req.IgnoreChanges, str)
			}
		}
	}

	if replaceOnChanges, ok := opts["replaceOnChanges"].([]any); ok {
		for _, s := range replaceOnChanges {
			if str, ok := s.(string); ok {
				req.ReplaceOnChanges = append(req.ReplaceOnChanges, str)
			}
		}
	}

	if aliases, ok := opts["aliases"].([]any); ok {
		for _, a := range aliases {
			switch v := a.(type) {
			case string:
				req.Aliases = append(req.Aliases, &pulumirpc.Alias{
					Alias: &pulumirpc.Alias_Urn{Urn: v},
				})
			case map[string]any:
				spec := &pulumirpc.Alias_Spec{}
				if name, ok := v["name"].(string); ok {
					spec.Name = name
				}
				if typ, ok := v["type"].(string); ok {
					spec.Type = typ
				}
				if stack, ok := v["stack"].(string); ok {
					spec.Stack = stack
				}
				if project, ok := v["project"].(string); ok {
					spec.Project = project
				}
				if parentURN, ok := v["parent"].(string); ok {
					spec.Parent = &pulumirpc.Alias_Spec_ParentUrn{ParentUrn: parentURN}
				}
				if noParent, ok := v["noParent"].(bool); ok && noParent {
					spec.Parent = &pulumirpc.Alias_Spec_NoParent{NoParent: true}
				}
				req.Aliases = append(req.Aliases, &pulumirpc.Alias{
					Alias: &pulumirpc.Alias_Spec_{Spec: spec},
				})
			}
		}
	}

	if deletedWith, ok := opts["deletedWith"].(string); ok {
		req.DeletedWith = deletedWith
	}

	if ct, ok := opts["customTimeouts"].(map[string]any); ok {
		req.CustomTimeouts = &pulumirpc.RegisterResourceRequest_CustomTimeouts{}
		if v, ok := ct["create"].(string); ok {
			req.CustomTimeouts.Create = v
		}
		if v, ok := ct["update"].(string); ok {
			req.CustomTimeouts.Update = v
		}
		if v, ok := ct["delete"].(string); ok {
			req.CustomTimeouts.Delete = v
		}
	}

	if hideDiffs, ok := opts["hideDiffs"].([]any); ok {
		for _, s := range hideDiffs {
			if str, ok := s.(string); ok {
				req.HideDiffs = append(req.HideDiffs, str)
			}
		}
	}

	if replaceWith, ok := opts["replaceWith"].([]any); ok {
		for _, s := range replaceWith {
			if str, ok := s.(string); ok {
				req.ReplaceWith = append(req.ReplaceWith, str)
			}
		}
	}

	if v, ok := opts["replacementTrigger"]; ok {
		val, err := structpb.NewValue(v)
		if err != nil {
			return fmt.Errorf("replacementTrigger: %w", err)
		}
		req.ReplacementTrigger = val
	}

	if envVarMappings, ok := opts["envVarMappings"].(map[string]any); ok {
		req.EnvVarMappings = make(map[string]string, len(envVarMappings))
		for k, v := range envVarMappings {
			if str, ok := v.(string); ok {
				req.EnvVarMappings[k] = str
			}
		}
	}

	return nil
}

// bridgeRegisterOutputs handles the "register-outputs" bridge subcommand.
//
// Usage: bridge register-outputs <urn> <outputs_json>
//
//	bridge register-outputs <urn> --stdin   (read outputs from stdin)
func bridgeRegisterOutputs(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("register-outputs: need urn and outputs")
	}

	urn := args[0]
	outputsJSON := args[1]

	// Support reading outputs from stdin to avoid ARG_MAX limits.
	if outputsJSON == "--stdin" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("register-outputs: read stdin: %w", err)
		}
		outputsJSON = string(data)
	}

	outputs, err := jsonToStruct(outputsJSON)
	if err != nil {
		return fmt.Errorf("register-outputs: parse outputs: %w", err)
	}

	monitor, conn, err := connectMonitor()
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = monitor.RegisterResourceOutputs(context.Background(), &pulumirpc.RegisterResourceOutputsRequest{
		Urn:     urn,
		Outputs: outputs,
	})
	if err != nil {
		return fmt.Errorf("register-outputs: %w", err)
	}

	return nil
}

// bridgeInvoke handles the "invoke" bridge subcommand.
//
// Usage: bridge invoke <token> <args_json> [opts_json]
func bridgeInvoke(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("invoke: need token and args")
	}

	token := args[0]
	argsJSON := args[1]

	invokeArgs, err := jsonToStruct(argsJSON)
	if err != nil {
		return fmt.Errorf("invoke: parse args: %w", err)
	}

	req := &pulumirpc.ResourceInvokeRequest{
		Tok:  token,
		Args: invokeArgs,
	}

	// Parse options if provided.
	if len(args) > 2 && args[2] != "" {
		var opts map[string]any
		if err := json.Unmarshal([]byte(args[2]), &opts); err != nil {
			return fmt.Errorf("invoke: parse options: %w", err)
		}
		if provider, ok := opts["provider"].(string); ok {
			req.Provider = provider
		}
		if v, ok := opts["version"].(string); ok {
			req.Version = v
		}
		if v, ok := opts["pluginDownloadURL"].(string); ok {
			req.PluginDownloadURL = v
		}
	}

	monitor, conn, err := connectMonitor()
	if err != nil {
		return err
	}
	defer conn.Close()

	resp, err := monitor.Invoke(context.Background(), req)
	if err != nil {
		return fmt.Errorf("invoke: %w", err)
	}

	if len(resp.Failures) > 0 {
		var msgs []string
		for _, f := range resp.Failures {
			msgs = append(msgs, f.Reason)
		}
		return fmt.Errorf("invoke %s failed: %s", token, strings.Join(msgs, "; "))
	}

	fmt.Print(structToJSON(resp.Return))
	return nil
}

// bridgeLog handles the "log" bridge subcommand.
//
// Usage: bridge log <severity> <message>
func bridgeLog(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("log: need severity and message")
	}

	severity := args[0]
	message := args[1]

	var sev pulumirpc.LogSeverity
	switch strings.ToLower(severity) {
	case "debug":
		sev = pulumirpc.LogSeverity_DEBUG
	case "info":
		sev = pulumirpc.LogSeverity_INFO
	case "warning":
		sev = pulumirpc.LogSeverity_WARNING
	case "error":
		sev = pulumirpc.LogSeverity_ERROR
	default:
		sev = pulumirpc.LogSeverity_INFO
	}

	engine, conn, err := connectEngine()
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = engine.Log(context.Background(), &pulumirpc.LogRequest{
		Severity: sev,
		Message:  message,
	})
	if err != nil {
		return fmt.Errorf("log: %w", err)
	}

	return nil
}

// bridgeSupportsFeature handles the "supports-feature" bridge subcommand.
//
// Usage: bridge supports-feature <feature>
func bridgeSupportsFeature(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("supports-feature: need feature name")
	}

	feature := args[0]

	monitor, conn, err := connectMonitor()
	if err != nil {
		return err
	}
	defer conn.Close()

	resp, err := monitor.SupportsFeature(context.Background(), &pulumirpc.SupportsFeatureRequest{
		Id: feature,
	})
	if err != nil {
		return fmt.Errorf("supports-feature: %w", err)
	}

	if resp.HasSupport {
		return nil // exit code 0 = supported
	}
	os.Exit(1) // exit code 1 = not supported
	return nil
}

// bridgeCheckVersion handles the "check-version" bridge subcommand.
// It checks if the current Pulumi CLI version satisfies a semver range.
//
// Usage: bridge check-version <range>
func bridgeCheckVersion(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("check-version: need version range")
	}

	versionRange := strings.Trim(args[0], "\"")

	// Parse the expected range.
	expectedRange, err := semver.ParseRange(versionRange)
	if err != nil {
		return fmt.Errorf("check-version: invalid version range %q: %w", versionRange, err)
	}

	// Get the current CLI version from the SDK embedded version.
	currentVersion := sdk.Version

	if !expectedRange(currentVersion) {
		// Log error to engine before exiting.
		engine, conn, engineErr := connectEngine()
		if engineErr == nil {
			defer conn.Close()
			_, _ = engine.Log(context.Background(), &pulumirpc.LogRequest{
				Severity: pulumirpc.LogSeverity_ERROR,
				Message:  fmt.Sprintf("Pulumi CLI version %s does not satisfy the version range %q", currentVersion, versionRange),
			})
		}
		// Exit with code 32 to signal a user-actionable error (bail).
		os.Exit(32)
	}

	return nil
}
