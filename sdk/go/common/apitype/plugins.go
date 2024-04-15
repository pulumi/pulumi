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

// Package apitype contains the full set of "exchange types" that are serialized and sent across separately versionable
// boundaries, including service APIs, plugins, and file formats.  As a result, we must consider the versioning impacts
// for each change we make to types within this package.  In general, this means the following:
//
//  1. DO NOT take anything away
//  2. DO NOT change processing rules
//  3. DO NOT make optional things required
//  4. DO make anything new be optional
//
// In the event that this is not possible, a breaking change is implied.  The preferred approach is to never make
// breaking changes.  If that isn't possible, the next best approach is to support both the old and new formats
// side-by-side (for instance, by using a union type for the property in question).
//
//nolint:lll
package apitype

// apitype.PluginKind represents a kind of a plugin that may be dynamically loaded and used by Pulumi.
type PluginKind string

const (
	// AnalyzerPlugin is a plugin that can be used as a resource analyzer.
	AnalyzerPlugin PluginKind = "analyzer"
	// LanguagePlugin is a plugin that can be used as a language host.
	LanguagePlugin PluginKind = "language"
	// ResourcePlugin is a plugin that can be used as a resource provider for custom CRUD operations.
	ResourcePlugin PluginKind = "resource"
	// ConverterPlugin is a plugin that can be used to convert from other ecosystems to Pulumi.
	ConverterPlugin PluginKind = "converter"
	// ToolPlugin is an arbitrary plugin that can be run as a tool.
	ToolPlugin PluginKind = "tool"
)

// IsPluginKind returns true if k is a valid plugin kind, and false otherwise.
func IsPluginKind(k string) bool {
	switch PluginKind(k) {
	case AnalyzerPlugin, LanguagePlugin, ResourcePlugin, ConverterPlugin, ToolPlugin:
		return true
	default:
		return false
	}
}

// We currently bundle some plugins with "pulumi" and thus expect them to be next to the pulumi binary.
// Eventually we want to fix this so new plugins are true plugins in the plugin cache.
func IsPluginBundled(kind PluginKind, name string) bool {
	return (kind == LanguagePlugin && name == "nodejs") ||
		(kind == LanguagePlugin && name == "go") ||
		(kind == LanguagePlugin && name == "python") ||
		(kind == LanguagePlugin && name == "dotnet") ||
		(kind == LanguagePlugin && name == "yaml") ||
		(kind == LanguagePlugin && name == "java") ||
		(kind == ResourcePlugin && name == "pulumi-nodejs") ||
		(kind == ResourcePlugin && name == "pulumi-python") ||
		(kind == AnalyzerPlugin && name == "policy") ||
		(kind == AnalyzerPlugin && name == "policy-python")
}
