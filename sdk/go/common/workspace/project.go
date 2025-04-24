// Copyright 2016-2022, Pulumi Corporation.
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

package workspace

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/pulumi/esc/ast"
	"github.com/pulumi/esc/eval"
	"github.com/texttheater/golang-levenshtein/levenshtein"

	"github.com/hashicorp/go-multierror"
	"github.com/pgavlin/fx"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"golang.org/x/exp/maps"
	"gopkg.in/yaml.v3"
)

const (
	arrayTypeName   = "array"
	integerTypeName = "integer"
	stringTypeName  = "string"
	booleanTypeName = "boolean"
)

//go:embed project.json
var projectSchema string

var ProjectSchema *jsonschema.Schema

func init() {
	compiler := jsonschema.NewCompiler()
	compiler.LoadURL = func(u string) (io.ReadCloser, error) {
		if u == "blob://project.json" {
			return io.NopCloser(strings.NewReader(projectSchema)), nil
		}
		return jsonschema.LoadURL(u)
	}
	ProjectSchema = compiler.MustCompile("blob://project.json")
}

// Analyzers is a list of analyzers to run on this project.
type Analyzers []tokens.QName

// ProjectTemplate is a Pulumi project template manifest.
type ProjectTemplate = apitype.ProjectTemplate

// ProjectTemplateConfigValue is a config value included in the project template manifest.
type ProjectTemplateConfigValue = apitype.ProjectTemplateConfigValue

// ProjectBackend is the configuration for where the backend state is stored. If unset, will use the
// system's currently logged-in backend.
//
// Use the same URL format that is passed to "pulumi login", see
// https://www.pulumi.com/docs/cli/commands/pulumi_login/
//
// To explicitly use the Pulumi Cloud backend, use URL "https://api.pulumi.com"
type ProjectBackend struct {
	// URL is optional field to explicitly set backend url
	URL string `json:"url,omitempty" yaml:"url,omitempty"`
}

type ProjectOptions struct {
	// Refresh is the ability to always run a refresh as part of a pulumi update / preview / destroy
	Refresh string `json:"refresh,omitempty" yaml:"refresh,omitempty"`
}

type PluginOptions struct {
	Name    string `json:"name" yaml:"name"`
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
	Path    string `json:"path" yaml:"path"`
}

// PackageSpec defines the structured format for a package dependency
type PackageSpec struct {
	Source     string   `json:"source" yaml:"source"`
	Version    string   `json:"version,omitempty" yaml:"version,omitempty"`
	Parameters []string `json:"parameters,omitempty" yaml:"parameters,omitempty"`
}

// packageValue can be either a string or a PackageSpec
// This is a private implementation type that handles the dual-format parsing
type packageValue struct {
	// The underlying value, either a string or a PackageSpec
	value interface{}
}

// Spec returns the package as a PackageSpec if it's a PackageSpec,
// or creates a simple PackageSpec from its string representation.
func (pv *packageValue) Spec() PackageSpec {
	switch v := pv.value.(type) {
	case PackageSpec:
		return v
	case string:
		// Check if the string contains a version specifier in the format "source@version"
		parts := strings.Split(v, "@")
		if len(parts) == 2 {
			return PackageSpec{
				Source:  parts[0],
				Version: parts[1],
			}
		}
		return PackageSpec{Source: v}
	default:
		panic("unexpected type in PackageValue")
	}
}

// UnmarshalJSON implements the json.Unmarshaler interface
func (pv *packageValue) UnmarshalJSON(data []byte) error {
	// First try to unmarshal as a string
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		pv.value = s
		return nil
	}

	// If that fails, try to unmarshal as a PackageSpec
	var spec PackageSpec
	if err := json.Unmarshal(data, &spec); err == nil {
		pv.value = spec
		return nil
	}

	return errors.New("package must be either a string or a package specification object")
}

// MarshalJSON implements the json.Marshaler interface
func (pv packageValue) MarshalJSON() ([]byte, error) {
	return json.Marshal(pv.value)
}

// UnmarshalYAML implements the yaml.Unmarshaler interface
func (pv *packageValue) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// First try to unmarshal as a string
	var s string
	if err := unmarshal(&s); err == nil {
		pv.value = s
		return nil
	}

	// If that fails, try to unmarshal as a PackageSpec
	var spec PackageSpec
	if err := unmarshal(&spec); err == nil {
		pv.value = spec
		return nil
	}

	return errors.New("package must be either a string or a package specification object")
}

// MarshalYAML implements the yaml.Marshaler interface
func (pv packageValue) MarshalYAML() (interface{}, error) {
	return pv.value, nil
}

type Plugins struct {
	Providers []PluginOptions `json:"providers,omitempty" yaml:"providers,omitempty"`
	Languages []PluginOptions `json:"languages,omitempty" yaml:"languages,omitempty"`
	Analyzers []PluginOptions `json:"analyzers,omitempty" yaml:"analyzers,omitempty"`
}

type ProjectConfigItemsType struct {
	Type  string                  `json:"type,omitempty" yaml:"type,omitempty"`
	Items *ProjectConfigItemsType `json:"items,omitempty" yaml:"items,omitempty"`
}

type ProjectConfigType struct {
	Type        *string                 `json:"type,omitempty" yaml:"type,omitempty"`
	Description string                  `json:"description,omitempty" yaml:"description,omitempty"`
	Items       *ProjectConfigItemsType `json:"items,omitempty" yaml:"items,omitempty"`
	Default     interface{}             `json:"default,omitempty" yaml:"default,omitempty"`
	Value       interface{}             `json:"value,omitempty" yaml:"value,omitempty"`
	Secret      bool                    `json:"secret,omitempty" yaml:"secret,omitempty"`
}

// IsExplicitlyTyped returns whether the project config type is explicitly typed.
// When that is the case, we validate stack config values against this type, given that
// the stack config value is namespaced by the project.
func (configType *ProjectConfigType) IsExplicitlyTyped() bool {
	return configType.Type != nil
}

func (configType *ProjectConfigType) TypeName() string {
	if configType.Type != nil {
		return *configType.Type
	}

	return ""
}

// Project is a Pulumi project manifest.
//
// We explicitly add yaml tags (instead of using the default behavior from https://github.com/ghodss/yaml which works
// in terms of the JSON tags) so we can directly marshall and unmarshall this struct using go-yaml an have the fields
// in the serialized object match the order they are defined in this struct.
//
// TODO[pulumi/pulumi#423]: use DOM based marshalling so we can roundtrip the seralized structure perfectly.
type Project struct {
	// Name is a required fully qualified name.
	Name tokens.PackageName `json:"name" yaml:"name"`
	// Runtime is a required runtime that executes code.
	Runtime ProjectRuntimeInfo `json:"runtime" yaml:"runtime"`
	// Main is an optional override for the program's main entry-point location.
	Main string `json:"main,omitempty" yaml:"main,omitempty"`

	// Description is an optional informational description.
	Description *string `json:"description,omitempty" yaml:"description,omitempty"`
	// Author is an optional author that created this project.
	Author *string `json:"author,omitempty" yaml:"author,omitempty"`
	// Website is an optional website for additional info about this project.
	Website *string `json:"website,omitempty" yaml:"website,omitempty"`
	// License is the optional license governing this project's usage.
	License *string `json:"license,omitempty" yaml:"license,omitempty"`

	// Config has been renamed to StackConfigDir.
	Config map[string]ProjectConfigType `json:"config,omitempty" yaml:"config,omitempty"`

	// StackConfigDir indicates where to store the Pulumi.<stack-name>.yaml files, combined with the folder
	// Pulumi.yaml is in.
	StackConfigDir string `json:"stackConfigDir,omitempty" yaml:"stackConfigDir,omitempty"`

	// Template is an optional template manifest, if this project is a template.
	Template *ProjectTemplate `json:"template,omitempty" yaml:"template,omitempty"`

	// Backend is an optional backend configuration
	Backend *ProjectBackend `json:"backend,omitempty" yaml:"backend,omitempty"`

	// Options is an optional set of project options
	Options *ProjectOptions `json:"options,omitempty" yaml:"options,omitempty"`

	// Packages is a map of package dependencies that can be either strings or PackageSpecs
	Packages map[string]packageValue `json:"packages,omitempty" yaml:"packages,omitempty"`

	Plugins *Plugins `json:"plugins,omitempty" yaml:"plugins,omitempty"`

	// Handle additional keys, albeit in a way that will remove comments and trivia.
	AdditionalKeys map[string]interface{} `yaml:",inline"`

	// The original byte representation of the file, used to attempt trivia-preserving edits
	raw []byte
}

func (proj Project) RawValue() []byte {
	return proj.raw
}

// GetPackageSpecs returns a map of package names to their corresponding PackageSpecs
func (proj *Project) GetPackageSpecs() map[string]PackageSpec {
	if proj.Packages == nil {
		return nil
	}

	result := make(map[string]PackageSpec)
	for name, packageValue := range proj.Packages {
		result[name] = packageValue.Spec()
	}
	return result
}

func (proj *Project) AddPackage(name string, spec PackageSpec) {
	if proj.Packages == nil {
		proj.Packages = make(map[string]packageValue)
	}

	// We default to using the simple string format, but if the package
	// - has parameters or
	// - if all existing packages use the PackageSpec format or
	// - if this same package was already added using the PackageSpec format
	// we use the PackageSpec format.
	useStringFormat := true
	if len(spec.Parameters) > 0 {
		// Simple string format does not support parameters
		useStringFormat = false
	} else if existingSpec, ok := proj.Packages[name]; ok {
		if _, ok := existingSpec.value.(PackageSpec); ok {
			// If the existing spec is a PackageSpec, keep its format
			useStringFormat = false
		}
	} else if len(proj.Packages) > 0 {
		// Check if all existing packages are PackageSpec
		// If all packages are already using the PackageSpec format, maintain consistency
		allPackageSpecs := true
		for _, existingPackage := range proj.Packages {
			if _, ok := existingPackage.value.(PackageSpec); !ok {
				allPackageSpecs = false
				break
			}
		}
		if allPackageSpecs {
			useStringFormat = false
		}
	}

	// Add the package to the project
	if useStringFormat {
		specString := spec.Source
		if spec.Version != "" {
			specString = fmt.Sprintf("%s@%s", spec.Source, spec.Version)
		}
		proj.Packages[name] = packageValue{value: specString}
	} else {
		proj.Packages[name] = packageValue{value: spec}
	}
}

func isPrimitiveValue(value interface{}) bool {
	switch value.(type) {
	case string, int, bool:
		return true
	default:
		return false
	}
}

func isArray(value interface{}) bool {
	_, ok := value.([]interface{})
	return ok
}

// RewriteConfigPathIntoStackConfigDir checks if the project is using the old "config" property
// to declare a path to the stack configuration directory. If that is the case, we rewrite it
// such that the value in config: {value} is moved to stackConfigDir: {value}.
// if the user defines both values as strings, we error out.
func RewriteConfigPathIntoStackConfigDir(project map[string]interface{}) (map[string]interface{}, error) {
	config, hasConfig := project["config"]
	_, hasStackConfigDir := project["stackConfigDir"]

	if hasConfig {
		configText, configIsText := config.(string)
		if configIsText && hasStackConfigDir {
			return nil, errors.New("Should not use both config and stackConfigDir to define the stack directory. " +
				"Use only stackConfigDir instead.")
		} else if configIsText && !hasStackConfigDir {
			// then we have config: {value}. Move this to stackConfigDir: {value}
			project["stackConfigDir"] = configText
			// reset the config property
			project["config"] = nil
			return project, nil
		}
	}

	return project, nil
}

// RewriteShorthandConfigValues rewrites short-hand version of configuration into a configuration type
// for example the following config block definition:
//
//	config:
//		  instanceSize: t3.mirco
//		  aws:region: us-west-2
//
// will be rewritten into a typed value:
//
//	config:
//	   instanceSize:
//	      default: t3.micro
//	   aws:region:
//	      value: us-west-2
//
// Note that short-hand values without namespaces (project config) are turned into a type
// where as short-hand values with namespaces (such as aws:region) are turned into a value.
func RewriteShorthandConfigValues(project map[string]interface{}) map[string]interface{} {
	configMap, foundConfig := project["config"]
	projectName := project["name"].(string)
	if !foundConfig {
		// no config defined, return as is
		return project
	}

	config, ok := configMap.(map[string]interface{})

	if !ok {
		return project
	}

	for key, value := range config {
		if isPrimitiveValue(value) || isArray(value) {
			configTypeDefinition := make(map[string]interface{})
			if configKeyIsNamespacedByProject(projectName, key) {
				// then this is a project namespaced config _type_ with a default value
				configTypeDefinition["default"] = value
			} else {
				// then this is a non-project namespaced config _value_
				configTypeDefinition["value"] = value
			}

			config[key] = configTypeDefinition
			continue
		}
	}

	return project
}

// Cast any map[interface{}] from the yaml decoder to map[string]
func SimplifyMarshalledValue(raw interface{}) (interface{}, error) {
	var cast func(value interface{}) (interface{}, error)
	cast = func(value interface{}) (interface{}, error) {
		if objMap, ok := value.(map[interface{}]interface{}); ok {
			strMap := make(map[string]interface{})
			for key, value := range objMap {
				if strKey, ok := key.(string); ok {
					innerValue, err := cast(value)
					if err != nil {
						return nil, err
					}
					strMap[strKey] = innerValue
				} else {
					return nil, fmt.Errorf("expected only string keys, got '%s'", key)
				}
			}
			return strMap, nil
		} else if objArray, ok := value.([]interface{}); ok {
			strArray := make([]interface{}, len(objArray))
			for key, value := range objArray {
				innerValue, err := cast(value)
				if err != nil {
					return nil, err
				}
				strArray[key] = innerValue
			}
			return strArray, nil
		}
		return value, nil
	}

	return cast(raw)
}

func SimplifyMarshalledProject(raw interface{}) (map[string]interface{}, error) {
	result, err := SimplifyMarshalledValue(raw)
	if err != nil {
		return nil, err
	}

	var ok bool
	var obj map[string]interface{}
	if obj, ok = result.(map[string]interface{}); !ok {
		return nil, fmt.Errorf("expected project to be an object, was '%T'", result)
	}

	return obj, nil
}

func ValidateProject(raw interface{}) error {
	project, err := SimplifyMarshalledProject(raw)
	if err != nil {
		return err
	}

	// Manually validate keys that need more validation than the raw JSON schema
	// can provide.
	name, ok := project["name"]
	if !ok {
		closest := findClosestKey("name", project, maxValidationAttributeDistance)
		if closest != "" {
			return fmt.Errorf(
				"project is missing a 'name' attribute; found '%s' instead",
				closest,
			)
		}

		return errors.New("project is missing a 'name' attribute")
	}

	if strName, ok := name.(string); !ok || strName == "" {
		return errors.New("project is missing a non-empty string 'name' attribute")
	}

	if _, ok := project["runtime"]; !ok {
		closest := findClosestKey("runtime", project, maxValidationAttributeDistance)
		if closest != "" {
			return fmt.Errorf(
				"project is missing a 'runtime' attribute; found '%s' instead",
				closest,
			)
		}
		return errors.New("project is missing a 'runtime' attribute")
	}

	// We'll catch everything else with JSON schema, though we'll still try to
	// suggest fixes for common mistakes.
	if err = ProjectSchema.Validate(project); err == nil {
		return nil
	}
	validationError, ok := err.(*jsonschema.ValidationError)
	if !ok {
		return err
	}

	notAllowedRe := regexp.MustCompile(`'(\w[a-zA-Z0-9_]*)' not allowed$`)

	var errs *multierror.Error
	var appendError func(err *jsonschema.ValidationError)
	appendError = func(err *jsonschema.ValidationError) {
		if err.InstanceLocation != "" && err.Message != "" {
			errorf := func(path, message string, args ...interface{}) error {
				contract.Requiref(path != "", "path", "path must not be empty")
				return fmt.Errorf("%s: %s", path, fmt.Sprintf(message, args...))
			}

			msg := err.Message

			if match := notAllowedRe.FindStringSubmatch(msg); match != nil {
				attrName := match[1]
				attributes := getSchemaPathAttributes(err.InstanceLocation)

				closest := findClosestKey(attrName, attributes, maxValidationAttributeDistance)
				if closest != "" {
					msg = fmt.Sprintf("%s; did you mean '%s'?", msg, closest)
				} else if len(attributes) > 0 {
					valid := make([]string, 0, len(attributes))
					for k := range attributes {
						valid = append(valid, "'"+k+"'")
					}
					if len(valid) > 1 {
						sort.StringSlice.Sort(valid)
						msg = fmt.Sprintf("%s; the allowed attributes are %v and %s",
							msg, strings.Join(valid[:len(valid)-1], ", "), valid[len(valid)-1])
					} else {
						msg = fmt.Sprintf("%s; the only allowed attribute is %s", msg, valid[0])
					}
				}
			}
			errs = multierror.Append(errs, errorf("#"+err.InstanceLocation, "%v", msg))
		}
		for _, err := range err.Causes {
			appendError(err)
		}
	}
	appendError(validationError)

	return errs
}

// maxValidationAttributeDistance is the maximum Levenshtein distance we'll
// tolerate when searching for attribute names the user might have meant to
// type.
const maxValidationAttributeDistance = 2

// findClosestKey finds the closest attribute name in the given map to the
// supplied needle, where "closest" means the name with the smallest Levenshtein
// distance from the needle. The haystack will be sorted so that in the event
// multiple attributes have the same distance, the result will be deterministic
// (and be the first alphabetically).
func findClosestKey(
	needle string,
	haystack map[string]interface{},
	maxDistance int,
) string {
	match := ""
	closest := maxDistance + 1

	keys := maps.Keys(haystack)
	slices.Sort(keys)

	for _, key := range keys {
		d := levenshtein.DistanceForStrings(
			[]rune(strings.ToLower(needle)),
			[]rune(strings.ToLower(key)),
			levenshtein.DefaultOptionsWithSub,
		)

		if d == 0 {
			// We can't do better than 0 so we can short circuit in this case. Note
			// that a distance of 0 is possible since we lowercase the strings prior
			// to checking the Levenshtein distance, so e.g. "Name" and "NAME" will
			// become "name"/"name" and yield a distance of 0.
			return key
		} else if d < closest {
			closest = d
			match = key
		} else {
			continue
		}
	}

	return match
}

// getSchemaPathAttributes walks the given path into the project schema and
// returns a list of attributes that can be subsequently traversed at the end of
// that path.
func getSchemaPathAttributes(path string) map[string]interface{} {
	elements := strings.Split(path, "/")
	isNumber := regexp.MustCompile(`^\d+$`)

	curr := ProjectSchema
	for len(elements) > 0 {
		attr := elements[0]
		elements = elements[1:]

		if attr == "" {
			continue
		}

		// If this schema node references another, continue from there.
		if curr.Ref != nil {
			curr = curr.Ref
		}

		// Check properties for matching attributes.
		if schema, ok := curr.Properties[attr]; ok {
			curr = schema
			continue
		}

		// Check additional properties.
		if curr.AdditionalProperties != nil {
			if additional, ok := curr.AdditionalProperties.(map[string]*jsonschema.Schema); ok {
				if schema, ok := additional[attr]; ok {
					curr = schema
					continue
				}
			}
		}

		// If the attribute is numeric, check for an array that can be indexed.
		if isNumber.MatchString(attr) && curr.Items2020 != nil {
			curr = curr.Items2020
			continue
		}

		// In all other cases we can't traverse the supplied path.
		return nil
	}

	// If we end on a reference, resolve it to the actual element before
	// enumerating attributes.
	if curr.Ref != nil {
		curr = curr.Ref
	}

	knownProperties := make(map[string]interface{})
	for k, v := range curr.Properties {
		knownProperties[k] = v
	}

	if curr.AdditionalProperties != nil {
		if additional, ok := curr.AdditionalProperties.(map[string]*jsonschema.Schema); ok {
			for k, v := range additional {
				knownProperties[k] = v
			}
		}
	}

	return knownProperties
}

func InferFullTypeName(typeName string, itemsType *ProjectConfigItemsType) string {
	if itemsType != nil {
		return fmt.Sprintf("array<%v>", InferFullTypeName(itemsType.Type, itemsType.Items))
	}

	return typeName
}

// ValidateConfig validates the config value against its config type definition.
// We use this to validate the default config values alongside their type definition but
// also to validate config values coming from individual stacks.
func ValidateConfigValue(typeName string, itemsType *ProjectConfigItemsType, value interface{}) bool {
	if typeName == stringTypeName {
		_, ok := value.(string)
		return ok
	}

	if typeName == integerTypeName {
		_, ok := value.(int)
		if ok {
			return true
		}
		// Config values come from YAML which by default will return floats not int. If it's a whole number
		// we'll allow it here though
		f, ok := value.(float64)
		if ok && f == math.Trunc(f) {
			return true
		}
		// Allow strings here if they parse as integers
		valueAsText, isText := value.(string)
		if isText {
			_, integerParseError := strconv.Atoi(valueAsText)
			return integerParseError == nil
		}

		return false
	}

	if typeName == booleanTypeName {
		// check to see if the value is a literal string "true" | "false"
		literalValue, ok := value.(string)
		if ok && (literalValue == "true" || literalValue == "false") {
			return true
		}

		_, ok = value.(bool)
		return ok
	}

	items, isArray := value.([]interface{})

	if !isArray || itemsType == nil {
		return false
	}

	// validate each item
	for _, item := range items {
		itemType := itemsType.Type
		underlyingItems := itemsType.Items
		if !ValidateConfigValue(itemType, underlyingItems, item) {
			return false
		}
	}

	return true
}

func configKeyIsNamespacedByProject(projectName string, configKey string) bool {
	return !strings.Contains(configKey, ":") || strings.HasPrefix(configKey, projectName+":")
}

func (proj *Project) Validate() error {
	if proj.Name == "" {
		return errors.New("project is missing a 'name' attribute")
	}
	if proj.Runtime.Name() == "" {
		return errors.New("project is missing a 'runtime' attribute")
	}

	projectName := proj.Name.String()
	for configKey, configType := range proj.Config {
		if configType.Default != nil && configType.Value != nil {
			return fmt.Errorf("project config '%v' cannot have both a 'default' and 'value' attribute", configKey)
		}

		configTypeName := configType.TypeName()

		if configKeyIsNamespacedByProject(projectName, configKey) {
			// namespaced by project
			if configType.IsExplicitlyTyped() && configType.TypeName() == arrayTypeName && configType.Items == nil {
				return fmt.Errorf("The configuration key '%v' declares an array "+
					"but does not specify the underlying type via the 'items' attribute", configKey)
			}

			// when we have a config _type_ with a schema
			if configType.IsExplicitlyTyped() && configType.Default != nil {
				if !ValidateConfigValue(configTypeName, configType.Items, configType.Default) {
					inferredTypeName := InferFullTypeName(configTypeName, configType.Items)
					return fmt.Errorf("The default value specified for configuration key '%v' is not of the expected type '%v'",
						configKey,
						inferredTypeName)
				}
			}
		} else {
			// when not namespaced by project, there shouldn't be a type, only a value
			if configType.IsExplicitlyTyped() {
				return fmt.Errorf("Configuration key '%v' is not namespaced by the project and should not define a type",
					configKey)
			}

			// default values are part of a type schema
			// when not namespaced by project, there is no type schema, only a value
			if configType.Default != nil {
				return fmt.Errorf("Configuration key '%v' is not namespaced by the project and "+
					"should not define a default value. "+
					"Did you mean to use the 'value' attribute instead of 'default'?", configKey)
			}

			// when not namespaced by project, there should be a value
			if configType.Value == nil {
				return fmt.Errorf("Configuration key '%v' is namespaced and must provide an attribute 'value'", configKey)
			}
		}
	}

	return nil
}

// Save writes a project definition to a file.
func (proj *Project) Save(path string) error {
	contract.Requiref(path != "", "path", "must not be empty")
	contract.Requiref(proj != nil, "proj", "must not be nil")

	err := proj.Validate()
	contract.Requiref(err == nil, "proj", "Validate(): %v", err)

	return save(path, proj, false /*mkDirAll*/)
}

type PolicyPackProject struct {
	// Runtime is a required runtime that executes code.
	Runtime ProjectRuntimeInfo `json:"runtime" yaml:"runtime"`
	// Version specifies the version of the policy pack. If set, it will override the
	// version specified in `package.json` for Node.js policy packs.
	Version string `json:"version,omitempty" yaml:"version,omitempty"`

	// Main is an optional override for the program's main entry-point location.
	Main string `json:"main,omitempty" yaml:"main,omitempty"`

	// Description is an optional informational description.
	Description *string `json:"description,omitempty" yaml:"description,omitempty"`
	// Author is an optional author that created this project.
	Author *string `json:"author,omitempty" yaml:"author,omitempty"`
	// Website is an optional website for additional info about this project.
	Website *string `json:"website,omitempty" yaml:"website,omitempty"`
	// License is the optional license governing this project's usage.
	License *string `json:"license,omitempty" yaml:"license,omitempty"`

	// The original byte representation of the file, used to attempt trivia-preserving edits
	raw []byte
}

func (proj PolicyPackProject) RawValue() []byte {
	return proj.raw
}

func (proj *PolicyPackProject) Validate() error {
	if proj.Runtime.Name() == "" {
		return errors.New("project is missing a 'runtime' attribute")
	}

	return nil
}

// Save writes a project definition to a file.
func (proj *PolicyPackProject) Save(path string) error {
	contract.Requiref(path != "", "path", "must not be empty")
	contract.Requiref(proj != nil, "proj", "must not be nil")
	contract.Requiref(proj.Validate() == nil, "proj", "Validate()")
	return save(path, proj, false /*mkDirAll*/)
}

type PluginProject struct {
	// Runtime is a required runtime that executes code.
	Runtime ProjectRuntimeInfo `json:"runtime" yaml:"runtime"`
}

func (proj *PluginProject) Validate() error {
	if proj.Runtime.Name() == "" {
		return errors.New("project is missing a 'runtime' attribute")
	}

	return nil
}

type Environment struct {
	envs    []string
	message json.RawMessage
	node    *yaml.Node
}

func NewEnvironment(envs []string) *Environment {
	return &Environment{envs: envs}
}

func (e *Environment) Definition() []byte {
	switch {
	case e == nil:
		// If there's no environment, return nil.
		return nil
	case len(e.envs) != 0:
		// If the environment was a list of environments, create an anonymous environment and return it.
		bytes, err := json.Marshal(map[string]any{"imports": e.envs})
		if err != nil {
			return nil
		}
		return bytes
	case e.message != nil:
		// If the environment was encoded as JSON, return the raw JSON.
		return e.message
	case e.node != nil:
		// Re-encode the YAML and return it.
		bytes, err := yaml.Marshal(e.node)
		if err != nil {
			return nil
		}
		return bytes
	default:
		return nil
	}
}

func (e *Environment) Imports() []string {
	def, diags, err := eval.LoadYAMLBytes("yaml", e.Definition())
	if err != nil || len(diags) != 0 || def == nil {
		return nil
	}
	names := fx.ToSlice(fx.Map(fx.IterSlice(def.Imports.GetElements()), func(imp *ast.ImportDecl) string {
		return imp.Environment.GetValue()
	}))
	if len(def.Values.GetEntries()) != 0 {
		names = append(names, "yaml")
	}
	return names
}

func (e *Environment) Append(envs ...string) *Environment {
	switch {
	case e == nil:
		// The stack has no environment block. Create one that imports the named environments.
		return NewEnvironment(envs)
	case e.message != nil:
		// The environment definition is inline JSON. Append the named environments to the import list,
		// creating the list if necessary.
		var m map[string]any
		if err := json.Unmarshal(e.message, &m); err == nil {
			imports, _ := m["imports"].([]any)
			anys := fx.ToSlice(fx.Map(fx.IterSlice(envs), func(e string) any { return e }))
			m["imports"] = append(imports, anys...)
			if new, err := json.Marshal(m); err == nil {
				e.message = new
			}
		}
		return e
	case e.node != nil:
		// The environment definition is inline YAML.
		// - If there is no import list, add one, then append the named envs to the import list
		// - If there is an import list, append the named envs to the import list
		root := e.node
		if root.Kind == yaml.MappingNode {
			var imports *yaml.Node
			for i := 0; i < len(root.Content); i += 2 {
				key := root.Content[i]
				if key.Kind == yaml.ScalarNode && key.Value == "imports" {
					imports = root.Content[i+1]
					break
				}
			}
			if imports == nil {
				root.Content = append([]*yaml.Node{
					{
						Kind:  yaml.ScalarNode,
						Style: root.Style,
						Tag:   "!!str",
						Value: "imports",
					},
					{
						Kind:  yaml.SequenceNode,
						Style: root.Style,
					},
				}, root.Content...)
				imports = root.Content[1]
			}
			if imports.Kind == yaml.SequenceNode {
				nodes := fx.ToSlice(fx.Map(fx.IterSlice(envs), func(env string) *yaml.Node {
					return &yaml.Node{
						Kind:  yaml.ScalarNode,
						Style: imports.Style,
						Tag:   "!!str",
						Value: env,
					}
				}))
				imports.Content = append(imports.Content, nodes...)
				return e
			}
		}
		return e
	default:
		// The environment definition is just a list of environments. Append to the list.
		e.envs = append(e.envs, envs...)
		return e
	}
}

func (e *Environment) Remove(env string) *Environment {
	switch {
	case e == nil:
		// There is no environment block, so there's nothing to remove.
		return nil
	case e.message != nil:
		// The environment definition is inline JSON. Find the last occurrence of the named environment in the import
		// list and remove it.
		var m map[string]any
		if err := json.Unmarshal(e.message, &m); err == nil {
			if imports, ok := m["imports"].([]any); ok {
				for i := len(imports) - 1; i >= 0; i-- {
					match := false
					switch e := imports[i].(type) {
					case string:
						match = e == env
					case map[string]any:
						match = len(e) == 1 && maps.Keys(e)[0] == env
					}
					if match {
						m["imports"] = append(imports[:i], imports[i+1:]...)
						if new, err := json.Marshal(m); err == nil {
							e.message = new
						}
						return e
					}
				}
			}
		}
		return e
	case e.node != nil:
		// The environment definition is inline YAML. Find the last occurrence of the named environment in the import
		// list and remove it.
		root := e.node
		if root.Kind == yaml.MappingNode {
			for i := 0; i < len(root.Content); i += 2 {
				key := root.Content[i]
				if key.Kind == yaml.ScalarNode && key.Value == "imports" {
					value := root.Content[i+1]
					if value.Kind == yaml.SequenceNode {
						for j := len(value.Content) - 1; j >= 0; j-- {
							n := value.Content[j]

							match := false
							switch n.Kind {
							case yaml.ScalarNode:
								match = n.Value == env
							case yaml.MappingNode:
								match = len(n.Content) == 2 && n.Content[0].Value == env
							case yaml.SequenceNode, yaml.AliasNode, yaml.DocumentNode:
								// These nodes never match, so we can ignore them here.
							}
							if match {
								value.Content = append(value.Content[:j], value.Content[j+1:]...)
								if len(value.Content) == 0 {
									root.Content = append(root.Content[:i], root.Content[i+2:]...)
								}
								return e
							}
						}
					}
				}
			}
		}
		return e
	default:
		// The environment definition is just a list of environments. Find the last occurrence of the named environment
		// in the list and remove it.
		for i := len(e.envs) - 1; i >= 0; i-- {
			n := e.envs[i]
			if n == env {
				e.envs = append(e.envs[:i], e.envs[i+1:]...)
				if len(e.envs) == 0 {
					return nil
				}
				return e
			}
		}
		return e
	}
}

func (e Environment) MarshalJSON() ([]byte, error) {
	if e.message == nil {
		return json.Marshal(e.envs)
	}
	return json.Marshal(e.message)
}

func (e *Environment) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &e.envs); err == nil {
		return nil
	}
	return json.Unmarshal(b, &e.message)
}

func (e Environment) MarshalYAML() (any, error) {
	if e.node == nil {
		return e.envs, nil
	}
	return e.node, nil
}

func (e *Environment) UnmarshalYAML(n *yaml.Node) error {
	if err := n.Decode(&e.envs); err == nil {
		return nil
	}
	e.node = n
	return nil
}

// ProjectStack holds stack specific information about a project.
type ProjectStack struct {
	// SecretsProvider is this stack's secrets provider.
	SecretsProvider string `json:"secretsprovider,omitempty" yaml:"secretsprovider,omitempty"`
	// EncryptedKey is the KMS-encrypted ciphertext for the data key used for secrets encryption.
	// Only used for cloud-based secrets providers.
	EncryptedKey string `json:"encryptedkey,omitempty" yaml:"encryptedkey,omitempty"`
	// EncryptionSalt is this stack's base64 encoded encryption salt.  Only used for
	// passphrase-based secrets providers.
	EncryptionSalt string `json:"encryptionsalt,omitempty" yaml:"encryptionsalt,omitempty"`
	// Config is an optional config bag.
	Config config.Map `json:"config,omitempty" yaml:"config,omitempty"`
	// Environment is an optional environment definition or list of environments.
	Environment *Environment `json:"environment,omitempty" yaml:"environment,omitempty"`

	// The original byte representation of the file, used to attempt trivia-preserving edits
	raw []byte
}

func (ps ProjectStack) EnvironmentBytes() []byte {
	return ps.Environment.Definition()
}

func (ps ProjectStack) RawValue() []byte {
	return ps.raw
}

// Save writes a project definition to a file.
func (ps *ProjectStack) Save(path string) error {
	contract.Requiref(path != "", "path", "must not be empty")
	contract.Requiref(ps != nil, "ps", "must not be nil")
	return save(path, ps, true /*mkDirAll*/)
}

type ProjectRuntimeInfo struct {
	name    string
	options map[string]interface{}
}

type ProjectStackDeployment struct {
	DeploymentSettings apitype.DeploymentSettings `json:"settings" yaml:"settings"`
}

func (psd *ProjectStackDeployment) Save(path string) error {
	contract.Requiref(path != "", "path", "must not be empty")
	contract.Requiref(psd != nil, "ps", "must not be nil")
	return save(path, psd, true /*mkDirAll*/)
}

func NewProjectRuntimeInfo(name string, options map[string]interface{}) ProjectRuntimeInfo {
	return ProjectRuntimeInfo{
		name:    name,
		options: options,
	}
}

func (info *ProjectRuntimeInfo) Name() string {
	return info.name
}

func (info *ProjectRuntimeInfo) Options() map[string]interface{} {
	return info.options
}

func (info *ProjectRuntimeInfo) SetOption(key string, value interface{}) {
	if info.options == nil {
		info.options = make(map[string]interface{})
	}
	info.options[key] = value
}

func (info ProjectRuntimeInfo) MarshalYAML() (interface{}, error) {
	if len(info.options) == 0 {
		return info.name, nil
	}

	return map[string]interface{}{
		"name":    info.name,
		"options": info.options,
	}, nil
}

func (info ProjectRuntimeInfo) MarshalJSON() ([]byte, error) {
	if len(info.options) == 0 {
		return json.Marshal(info.name)
	}

	return json.Marshal(map[string]interface{}{
		"name":    info.name,
		"options": info.options,
	})
}

func (info *ProjectRuntimeInfo) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &info.name); err == nil {
		return nil
	}

	var payload struct {
		Name    string                 `json:"name"`
		Options map[string]interface{} `json:"options"`
	}

	if err := json.Unmarshal(data, &payload); err == nil {
		info.name = payload.Name
		info.options = payload.Options
		return nil
	}

	return errors.New("runtime section must be a string or an object with name and options attributes")
}

func (info *ProjectRuntimeInfo) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := unmarshal(&info.name); err == nil {
		return nil
	}

	var payload struct {
		Name    string                 `yaml:"name"`
		Options map[string]interface{} `yaml:"options"`
	}

	if err := unmarshal(&payload); err == nil {
		info.name = payload.Name
		info.options = payload.Options
		return nil
	}

	return errors.New("runtime section must be a string or an object with name and options attributes")
}

func marshallerForPath(path string) (encoding.Marshaler, error) {
	ext := filepath.Ext(path)
	m, has := encoding.Marshalers[ext]
	if !has {
		return nil, fmt.Errorf("no marshaler found for file format '%v'", ext)
	}

	return m, nil
}

func save(path string, value interface{}, mkDirAll bool) error {
	contract.Requiref(path != "", "path", "must not be empty")
	contract.Requiref(value != nil, "value", "must not be nil")

	m, err := marshallerForPath(path)
	if err != nil {
		return err
	}

	b, err := m.Marshal(value)
	if err != nil {
		return err
	}

	if mkDirAll {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
	}

	//nolint:gosec
	return os.WriteFile(path, b, 0o644)
}

// To mitigate an import cycle, we define this here.
const PulumiTagsConfigKey = "pulumi:tags"

// AddConfigStackTags sets the project tags config to the given map of tags.
func (proj *Project) AddConfigStackTags(tags map[string]string) {
	if proj.Config == nil {
		proj.Config = map[string]ProjectConfigType{}
	}
	configTags, has := proj.Config["pulumi:tags"]
	if !has {
		configTags = ProjectConfigType{
			Value: map[string]string{},
		}
	}
	if configTags.Value == nil {
		configTags.Value = map[string]string{}
	}

	tagMap, ok := configTags.Value.(map[string]string)
	if !ok {
		logging.Warningf("overwriting non-object `%s` project config", "pulumi:tags")
		tagMap = map[string]string{}
	}
	for k, v := range tags {
		tagMap[k] = v
	}
	proj.Config["pulumi:tags"] = configTags
}
