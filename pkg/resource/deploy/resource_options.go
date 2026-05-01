// Copyright 2026, Pulumi Corporation.
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

package deploy

import (
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func resourceOptionsFromState(state *resource.State) *pulumirpc.ResourceOptions {
	if state == nil {
		return nil
	}

	resourceOptions := &pulumirpc.ResourceOptions{
		DependsOn:        urnsToStrings(state.Dependencies),
		IgnoreChanges:    append([]string(nil), state.IgnoreChanges...),
		ReplaceOnChanges: append([]string(nil), state.ReplaceOnChanges...),
		Provider:         state.Provider,
		DeletedWith:      string(state.DeletedWith),
		Import:           string(state.ImportID),
		HideDiff:         propertyPathsToStrings(state.HideDiff),
		ReplaceWith:      urnsToStrings(state.ReplaceWith),
		Hooks:            hookBindingsToProto(state.ResourceHooks),
		Parent:           string(state.Parent),
	}

	if len(state.Aliases) > 0 {
		resourceOptions.Aliases = make([]*pulumirpc.Alias, 0, len(state.Aliases))
		for _, alias := range state.Aliases {
			resourceOptions.Aliases = append(resourceOptions.Aliases, &pulumirpc.Alias{
				Alias: &pulumirpc.Alias_Urn{Urn: string(alias)},
			})
		}
	}

	if state.CustomTimeouts.IsNotEmpty() {
		resourceOptions.CustomTimeouts = &pulumirpc.RegisterResourceRequest_CustomTimeouts{
			Create: secondsToDurationString(state.CustomTimeouts.Create),
			Update: secondsToDurationString(state.CustomTimeouts.Update),
			Delete: secondsToDurationString(state.CustomTimeouts.Delete),
		}
	}

	if len(state.AdditionalSecretOutputs) > 0 {
		resourceOptions.AdditionalSecretOutputs = make([]string, 0, len(state.AdditionalSecretOutputs))
		for _, output := range state.AdditionalSecretOutputs {
			resourceOptions.AdditionalSecretOutputs = append(resourceOptions.AdditionalSecretOutputs, string(output))
		}
	}

	resourceOptions.Protect = &state.Protect
	resourceOptions.RetainOnDelete = &state.RetainOnDelete

	return resourceOptions
}

func hookBindingsToProto(
	bindings map[resource.HookType][]string,
) *pulumirpc.RegisterResourceRequest_ResourceHooksBinding {
	if len(bindings) == 0 {
		return nil
	}
	return &pulumirpc.RegisterResourceRequest_ResourceHooksBinding{
		BeforeCreate: append([]string(nil), bindings[resource.BeforeCreate]...),
		AfterCreate:  append([]string(nil), bindings[resource.AfterCreate]...),
		BeforeUpdate: append([]string(nil), bindings[resource.BeforeUpdate]...),
		AfterUpdate:  append([]string(nil), bindings[resource.AfterUpdate]...),
		BeforeDelete: append([]string(nil), bindings[resource.BeforeDelete]...),
		AfterDelete:  append([]string(nil), bindings[resource.AfterDelete]...),
		OnError:      append([]string(nil), bindings[resource.OnError]...),
	}
}

func urnsToStrings(urns []resource.URN) []string {
	if len(urns) == 0 {
		return nil
	}
	values := make([]string, 0, len(urns))
	for _, urn := range urns {
		values = append(values, string(urn))
	}
	return values
}

func propertyPathsToStrings(paths []resource.PropertyPath) []string {
	if len(paths) == 0 {
		return nil
	}
	values := make([]string, 0, len(paths))
	for _, path := range paths {
		values = append(values, path.String())
	}
	return values
}

func secondsToDurationString(seconds float64) string {
	if seconds == 0 {
		return ""
	}
	return time.Duration(seconds * float64(time.Second)).String()
}
