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
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/santhosh-tekuri/jsonschema/v5"
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
type ProjectTemplate struct {
	// Description is an optional description of the template.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// Quickstart contains optional text to be displayed after template creation.
	Quickstart string `json:"quickstart,omitempty" yaml:"quickstart,omitempty"`
	// Config is an optional template config.
	Config map[string]ProjectTemplateConfigValue `json:"config,omitempty" yaml:"config,omitempty"`
	// Important indicates the template is important and should be listed by default.
	Important bool `json:"important,omitempty" yaml:"important,omitempty"`
}

// ProjectTemplateConfigValue is a config value included in the project template manifest.
type ProjectTemplateConfigValue struct {
	// Description is an optional description for the config value.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// Default is an optional default value for the config value.
	Default string `json:"default,omitempty" yaml:"default,omitempty"`
	// Secret may be set to true to indicate that the config value should be encrypted.
	Secret bool `json:"secret,omitempty" yaml:"secret,omitempty"`
}

// ProjectBackend is a configuration for backend used by project
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

type Plugins struct {
	Providers []PluginOptions `json:"providers,omitempty" yaml:"providers,omitempty"`
	Languages []PluginOptions `json:"languages,omitempty" yaml:"languages,omitempty"`
	Analyzers []PluginOptions `json:"analyzers,omitempty" yaml:"analyzers,omitempty"`
}

type ProjectConfigItemsType struct {
	Type  string                  `json:"type" yaml:"type"`
	Items *ProjectConfigItemsType `json:"items" yaml:"items"`
}

type ProjectConfigType struct {
	Type        string                  `json:"type" yaml:"type"`
	Description string                  `json:"description" yaml:"description"`
	Items       *ProjectConfigItemsType `json:"items" yaml:"items"`
	Default     interface{}             `json:"default" yaml:"default"`
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

	Plugins *Plugins `json:"plugins,omitempty" yaml:"plugins,omitempty"`

	// Handle additional keys, albeit in a way that will remove comments and trivia.
	AdditionalKeys map[string]interface{} `yaml:",inline"`
}

func isPrimitiveValue(value interface{}) (string, bool) {
	_, isLiteralBoolean := value.(bool)
	if isLiteralBoolean {
		return "boolean", true
	}

	_, isLiteralInt := value.(int)
	if isLiteralInt {
		return "integer", true
	}

	_, isLiteralString := value.(string)
	if isLiteralString {
		return "string", true
	}

	return "", false
}

// RewriteShorthandConfigValues rewrites short-hand version of configuration into a configuration type
// for example the following config block definition:
//
//     config:
//        instanceSize: t3.mirco
//
// will be rewritten into a typed value:
//
//     config:
//       instanceSize:
//         type: string
//         default: t3.mirco
//
func RewriteShorthandConfigValues(project map[string]interface{}) map[string]interface{} {
	configMap, foundConfig := project["config"]

	if !foundConfig {
		// no config defined, return as is
		return project
	}

	config, ok := configMap.(map[string]interface{})

	if !ok {
		return project
	}

	for key, value := range config {
		typeName, isLiteral := isPrimitiveValue(value)
		if isLiteral {
			configTypeDefinition := make(map[string]interface{})
			configTypeDefinition["type"] = typeName
			configTypeDefinition["default"] = value
			config[key] = configTypeDefinition
		}
	}

	return project
}

// Cast any map[interface{}] from the yaml decoder to map[string]
func SimplifyMarshalledProject(raw interface{}) (map[string]interface{}, error) {
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
	result, err := cast(raw)
	if err != nil {
		return nil, err
	}

	var ok bool
	var obj map[string]interface{}
	if obj, ok = result.(map[string]interface{}); !ok {
		return nil, fmt.Errorf("expected an object")
	}

	return obj, nil
}

func ValidateProject(raw interface{}) error {

	project, err := SimplifyMarshalledProject(raw)
	if err != nil {
		return err
	}

	// Couple of manual errors to match Validate
	name, ok := project["name"]
	if !ok {
		return errors.New("project is missing a 'name' attribute")
	}
	if strName, ok := name.(string); !ok || strName == "" {
		return errors.New("project is missing a non-empty string 'name' attribute")
	}
	if _, ok := project["runtime"]; !ok {
		return errors.New("project is missing a 'runtime' attribute")
	}

	project = RewriteShorthandConfigValues(project)

	// Let everything else be caught by jsonschema
	if err = ProjectSchema.Validate(project); err == nil {
		return nil
	}
	validationError, ok := err.(*jsonschema.ValidationError)
	if !ok {
		return err
	}

	var errs *multierror.Error
	var appendError func(err *jsonschema.ValidationError)
	appendError = func(err *jsonschema.ValidationError) {
		if err.InstanceLocation != "" && err.Message != "" {
			errorf := func(path, message string, args ...interface{}) error {
				contract.Require(path != "", "path")
				return fmt.Errorf("%s: %s", path, fmt.Sprintf(message, args...))
			}

			errs = multierror.Append(errs, errorf("#"+err.InstanceLocation, "%v", err.Message))
		}
		for _, err := range err.Causes {
			appendError(err)
		}
	}
	appendError(validationError)

	return errs
}

func InferFullTypeName(typeName string, itemsType *ProjectConfigItemsType) string {
	if itemsType != nil {
		return fmt.Sprintf("array<%v>", InferFullTypeName(itemsType.Type, itemsType.Items))
	}

	return typeName
}

func ValidateConfigValue(typeName string, itemsType *ProjectConfigItemsType, value interface{}) bool {

	if typeName == "string" {
		_, ok := value.(string)
		return ok
	}

	if typeName == "integer" {
		_, ok := value.(int)
		if !ok {
			valueAsText, isText := value.(string)
			if isText {
				_, integerParseError := strconv.Atoi(valueAsText)
				return integerParseError == nil
			}
		}
		return ok
	}

	if typeName == "boolean" {
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

func (proj *Project) Validate() error {
	if proj.Name == "" {
		return errors.New("project is missing a 'name' attribute")
	}
	if proj.Runtime.Name() == "" {
		return errors.New("project is missing a 'runtime' attribute")
	}

	for configKey, configType := range proj.Config {
		if configType.Default != nil {
			// when the default value is specified, validate it against the type
			if !ValidateConfigValue(configType.Type, configType.Items, configType.Default) {
				inferredTypeName := InferFullTypeName(configType.Type, configType.Items)
				return errors.Errorf("The default value specified for configuration key '%v' is not of the expected type '%v'",
					configKey,
					inferredTypeName)
			}
		}
	}

	return nil
}

// TrustResourceDependencies returns whether or not this project's runtime can be trusted to accurately report
// dependencies. All languages supported by Pulumi today do this correctly. This option remains useful when bringing
// up new Pulumi languages.
func (proj *Project) TrustResourceDependencies() bool {
	return true
}

// Save writes a project definition to a file.
func (proj *Project) Save(path string) error {
	contract.Require(path != "", "path")
	contract.Require(proj != nil, "proj")
	contract.Requiref(proj.Validate() == nil, "proj", "Validate()")
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
}

func (proj *PolicyPackProject) Validate() error {
	if proj.Runtime.Name() == "" {
		return errors.New("project is missing a 'runtime' attribute")
	}

	return nil
}

// Save writes a project definition to a file.
func (proj *PolicyPackProject) Save(path string) error {
	contract.Require(path != "", "path")
	contract.Require(proj != nil, "proj")
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
}

// Save writes a project definition to a file.
func (ps *ProjectStack) Save(path string) error {
	contract.Require(path != "", "path")
	contract.Require(ps != nil, "ps")
	return save(path, ps, true /*mkDirAll*/)
}

type ProjectRuntimeInfo struct {
	name    string
	options map[string]interface{}
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
	if info.options == nil || len(info.options) == 0 {
		return info.name, nil
	}

	return map[string]interface{}{
		"name":    info.name,
		"options": info.options,
	}, nil
}

func (info ProjectRuntimeInfo) MarshalJSON() ([]byte, error) {
	if info.options == nil || len(info.options) == 0 {
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
		return nil, errors.Errorf("no marshaler found for file format '%v'", ext)
	}

	return m, nil
}

func save(path string, value interface{}, mkDirAll bool) error {
	contract.Require(path != "", "path")
	contract.Require(value != nil, "value")

	m, err := marshallerForPath(path)
	if err != nil {
		return err
	}

	b, err := m.Marshal(value)
	if err != nil {
		return err
	}

	if mkDirAll {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
	}

	//nolint: gosec
	return ioutil.WriteFile(path, b, 0644)
}
