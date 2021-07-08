//nolint: goconst
package yaml

import (
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"gopkg.in/yaml.v3"
)

// Package describes a Pulumi package.
type Package struct {
	// Name is the unqualified name of the package (e.g. "aws", "azure", "gcp", "kubernetes", "random")
	Name string `yaml:"name"`
	// Version is the version of the package. The version must be valid semver.
	Version string `yaml:"version,omitempty"`
	// Description is the description of the package.
	Description string `yaml:"description,omitempty"`
	// Keywords is the list of keywords that are associated with the package, if any.
	Keywords []string `yaml:"keywords,omitempty"`
	// Homepage is the package's homepage.
	Homepage string `yaml:"homepage,omitempty"`
	// License indicates which license is used for the package's contents.
	License string `yaml:"license,omitempty"`
	// Attribution allows freeform text attribution of derived work, if needed.
	Attribution string `yaml:"attribution,omitempty"`
	// Repository is the URL at which the source for the package can be found.
	Repository string `yaml:"repository,omitempty"`
	// LogoURL is the URL for the package's logo, if any.
	LogoURL string `yaml:"logoUrl,omitempty"`
	// PluginDownloadURL is the URL to use to acquire the provider plugin binary, if any.
	PluginDownloadURL string `yaml:"pluginDownloadURL,omitempty"`

	// Import describes the packages this package imports.
	Imports Imports `yaml:"imports"`
	// Provider describes the provider type for this package.
	Provider *Resource `yaml:"provider"`

	// Members describes the members of the package. A member may be an object type,
	// enum type, resource, component, function, or module. Modules are themselves
	// collections of members.
	Members Members `yaml:"members"`

	// Language specifies additional language-specific data about the package.
	Language map[string]interface{} `yaml:"language"`
}

// Spec translates the Package and its members to a PackageSpec.
func (p *Package) Spec() (spec *schema.PackageSpec, err error) {
	var language map[string]json.RawMessage
	if len(p.Language) != 0 {
		language, err = languageSpec(p.Language)
		if err != nil {
			return nil, err
		}
	}

	spec = &schema.PackageSpec{
		Name:              p.Name,
		Version:           p.Version,
		Description:       p.Description,
		Keywords:          p.Keywords,
		Homepage:          p.Homepage,
		License:           p.License,
		Attribution:       p.Attribution,
		Repository:        p.Repository,
		LogoURL:           p.LogoURL,
		PluginDownloadURL: p.PluginDownloadURL,
		Types:             map[string]schema.ComplexTypeSpec{},
		Resources:         map[string]schema.ResourceSpec{},
		Functions:         map[string]schema.FunctionSpec{},
		Language:          language,
	}

	// TODO: config

	if p.Provider != nil {
		ps, err := p.Provider.spec(p, "", "Provider")
		if err != nil {
			return nil, err
		}
		spec.Provider = *ps
	}

	if err := p.module(spec, "/", p.Members); err != nil {
		return nil, err
	}

	return spec, nil
}

func (p *Package) module(spec *schema.PackageSpec, module string, members map[string]Member) error {
	for name, m := range members {
		token := spec.Name + ":" + module[1:] + ":" + name
		switch d := m.(type) {
		case *Object:
			o, err := d.spec(p, module, false)
			if err != nil {
				return err
			}
			spec.Types[token] = schema.ComplexTypeSpec{ObjectTypeSpec: *o}
		case *Enum:
			spec.Types[token] = d.spec()
		case *Resource:
			r, err := d.spec(p, module, name)
			if err != nil {
				return err
			}
			spec.Resources[token] = *r
		case *Component:
			r, err := d.spec(p, module, name)
			if err != nil {
				return err
			}
			spec.Resources[token] = *r
		case *Function:
			f, err := d.spec(p, module)
			if err != nil {
				return err
			}
			spec.Functions[token] = *f
		case Members:
			if err := p.module(spec, path.Join(module, name), d); err != nil {
				return err
			}
		}
	}
	return nil
}

type Imports map[string]Import

func (i *Imports) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("imports must be a mapping")
	}

	result := Imports{}
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]

		var name string
		if err := key.Decode(&name); err != nil {
			return err
		}

		var imp Import
		if err := value.Decode(&imp); err != nil {
			return err
		}
		if imp.Package == "" {
			imp.Package = name
		}

		result[name] = imp
	}

	*i = result
	return nil
}

type importData struct {
	Path    string `yaml:"path"`
	Package string `yaml:"package"`
}

type Import importData

func (i *Import) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.MappingNode {
		return node.Decode((*importData)(i))
	}
	return node.Decode(&i.Path)
}

type Members map[string]Member

func (m *Members) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("members must be a mapping")
	}

	result := Members{}
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]

		var name string
		if err := key.Decode(&name); err != nil {
			return err
		}

		var member Member
		switch value.Tag {
		case "!Object":
			member = &Object{}
		case "!Enum":
			member = &Enum{}
		case "!Resource":
			member = &Resource{}
		case "!Component":
			member = &Component{}
		case "!Function":
			member = &Function{}
		case "!Module":
			m := Members{}
			member = &m
		default:
			return fmt.Errorf("unrecognized member tag '%v'", value.Tag)
		}
		if err := value.Decode(member); err != nil {
			return err
		}
		member.setDescription(commentText(key.HeadComment))

		result[name] = member
	}

	*m = result
	return nil
}

func (m Members) setDescription(string) {
}

type Member interface {
	setDescription(description string)
}

type Object struct {
	Description string
	Properties  Properties
}

func (o *Object) spec(pkg *Package, module string, outputs bool) (*schema.ObjectTypeSpec, error) {
	props, required, err := properties(pkg, module, o.Properties, outputs)
	if err != nil {
		return nil, err
	}
	return &schema.ObjectTypeSpec{
		Description: o.Description,
		Type:        "object",
		Properties:  props,
		Required:    required,
	}, nil
}

func (o *Object) UnmarshalYAML(node *yaml.Node) error {
	return node.Decode(&o.Properties)
}

func (o *Object) setDescription(d string) {
	o.Description = d
}

type Enum struct {
	Description string     `yaml:"-"`
	Type        string     `yaml:"type"`
	Values      EnumValues `yaml:"values"`
}

func (e *Enum) spec() schema.ComplexTypeSpec {
	values := make([]*schema.EnumValueSpec, 0, len(e.Values))
	for _, v := range e.Values {
		values = append(values, v.spec())
	}
	return schema.ComplexTypeSpec{
		ObjectTypeSpec: schema.ObjectTypeSpec{
			Description: e.Description,
			Type:        e.Type,
		},
		Enum: values,
	}
}

func (e *Enum) setDescription(d string) {
	e.Description = d
}

type EnumValues []EnumValue

func (e *EnumValues) UnmarshalYAML(node *yaml.Node) error {
	var result EnumValues
	switch node.Kind {
	case yaml.SequenceNode:
		result = make(EnumValues, len(node.Content))
		for i, value := range node.Content {
			var enumValue EnumValue
			if err := value.Decode(&enumValue); err != nil {
				return err
			}
			enumValue.Description = propertyDescription(value, value)

			result[i] = enumValue
		}
	case yaml.MappingNode:
		result = make(EnumValues, len(node.Content)/2)
		for i := 0; i < len(node.Content); i += 2 {
			key, value := node.Content[i], node.Content[i+1]

			var name string
			if err := key.Decode(&name); err != nil {
				return err
			}

			var enumValue EnumValue
			if err := value.Decode(&enumValue); err != nil {
				return err
			}
			enumValue.Name = name
			enumValue.Description = propertyDescription(key, value)

			result[i/2] = enumValue
		}
		sort.Slice(result, func(i, j int) bool {
			return result[i].Name < result[j].Name
		})
	default:
		return fmt.Errorf("enum must be a mapping or sequence")
	}

	*e = result
	return nil
}

type enumValueData struct {
	Description        string      `yaml:"-"`
	DeprecationMessage string      `yaml:"deprecationMessage"`
	Name               string      `yaml:"name"`
	Value              interface{} `yaml:"value"`
}

type EnumValue enumValueData

func (e *EnumValue) spec() *schema.EnumValueSpec {
	return &schema.EnumValueSpec{
		Name:        e.Name,
		Description: e.Description,
		Value:       e.Value,
	}
}

func (e *EnumValue) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.MappingNode {
		return node.Decode((*enumValueData)(e))
	}
	return node.Decode(&e.Value)
}

type resourceData struct {
	Description string              `yaml:"-"`
	Inputs      Properties          `yaml:"inputs"`
	Outputs     Properties          `yaml:"outputs"`
	Inouts      Properties          `yaml:"inouts"`
	StateInputs Properties          `yaml:"stateInputs"`
	Aliases     []Alias             `yaml:"aliases"`
	Methods     map[string]Function `yaml:"methods"`
}

func (r *resourceData) spec(pkg *Package, module, name string,
	isComponent bool) (spec *schema.ResourceSpec, err error) {

	// TODO: methods, aliases

	inputs := Properties{}
	for name, prop := range r.Inputs {
		inputs[name] = prop
	}
	outputs := Properties{}
	for name, prop := range r.Outputs {
		outputs[name] = prop
	}
	for name, prop := range r.Inouts {
		inputs[name], outputs[name] = prop, prop
	}

	var state *schema.ObjectTypeSpec
	if len(r.StateInputs) != 0 {
		state, err = (&Object{Properties: r.StateInputs}).spec(pkg, module, false)
		if err != nil {
			return nil, err
		}
	}

	inputsSpec, requiredInputs, err := properties(pkg, module, inputs, false)
	if err != nil {
		return nil, err
	}
	outputsSpec, requiredOutputs, err := properties(pkg, module, outputs, true)
	if err != nil {
		return nil, err
	}
	return &schema.ResourceSpec{
		ObjectTypeSpec: schema.ObjectTypeSpec{
			Description: r.Description,
			Properties:  outputsSpec,
			Required:    requiredOutputs,
		},
		InputProperties: inputsSpec,
		RequiredInputs:  requiredInputs,
		StateInputs:     state,
		IsComponent:     isComponent,
	}, nil
}

type Resource resourceData

func (r *Resource) spec(pkg *Package, module, name string) (*schema.ResourceSpec, error) {
	return (*resourceData)(r).spec(pkg, module, name, false)
}

func (r *Resource) setDescription(d string) {
	r.Description = d
}

type Component resourceData

func (c *Component) spec(pkg *Package, module, name string) (*schema.ResourceSpec, error) {
	return (*resourceData)(c).spec(pkg, module, name, true)
}

func (c *Component) setDescription(d string) {
	c.Description = d
}

type Alias struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"`
}

type Function struct {
	Description string     `yaml:"-"`
	Parameters  Properties `yaml:"parameters"`
	Returns     Properties `yaml:"returns"`
}

func (f *Function) spec(pkg *Package, module string) (spec *schema.FunctionSpec, err error) {
	var inputs *schema.ObjectTypeSpec
	if len(f.Parameters) != 0 {
		inputs, err = (&Object{Properties: f.Parameters}).spec(pkg, module, false)
		if err != nil {
			return nil, err
		}
	}
	var outputs *schema.ObjectTypeSpec
	if len(f.Returns) != 0 {
		outputs, err = (&Object{Properties: f.Returns}).spec(pkg, module, true)
		if err != nil {
			return nil, err
		}
	}
	return &schema.FunctionSpec{
		Description: f.Description,
		Inputs:      inputs,
		Outputs:     outputs,
	}, nil
}

func (f *Function) setDescription(d string) {
	f.Description = d
}

type Properties map[string]Property

func (p *Properties) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("properties must be a mapping")
	}

	result := Properties{}
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]

		var name string
		if err := key.Decode(&name); err != nil {
			return err
		}

		var property Property

		var err error
		if value.Kind != yaml.MappingNode {
			err = value.Decode(&property.Type)
		} else {
			err = value.Decode(&property)
		}
		if err != nil {
			return err
		}

		property.Description = propertyDescription(key, value)
		result[name] = property
	}

	*p = result
	return nil
}

type Property struct {
	Description string      `yaml:"-"`
	Type        TypeRef     `yaml:"type"`
	Const       interface{} `yaml:"const"`
	Default     *Default    `yaml:"default"`
	Secret      bool        `yaml:"secret"`
}

func (p *Property) spec(pkg *Package, module string,
	output bool) (required bool, spec *schema.PropertySpec, err error) {

	required = true

	typ := p.Type
	if typ.Constructor == "!Optional" {
		if len(typ.Args) != 1 {
			return false, nil, fmt.Errorf("!Optional accepts a single type argument")
		}
		required, typ = false, typ.Args[0]
	}
	typeSpec, err := typ.spec(pkg, module, output)
	if err != nil {
		return false, nil, err
	}

	defaultValue, defaultInfo := p.Default.spec()
	return required, &schema.PropertySpec{
		TypeSpec:    *typeSpec,
		Description: p.Description,
		Const:       p.Const,
		Default:     defaultValue,
		DefaultInfo: defaultInfo,
		Secret:      p.Secret,
	}, nil
}

type TypeRef struct {
	Constructor string
	Args        []TypeRef
	Spec        *schema.TypeSpec
	Object      *Object
}

func (t *TypeRef) UnmarshalYAML(node *yaml.Node) error {
	t.Constructor = node.Tag

	switch node.Tag {
	case "!!str":
		return node.Decode(&t.Constructor)
	case "!Spec":
		var spec schema.TypeSpec
		if err := node.Decode(&spec); err != nil {
			return err
		}
		t.Spec = &spec
		return nil
	case "!Object":
		var object Object
		if err := node.Decode(&object); err != nil {
			return err
		}
		t.Object = &object
		return nil
	default:
		if err := node.Decode(&t.Args); err != nil {
			return err
		}
		return nil
	}
}

func (t *TypeRef) spec(p *Package, module string, output bool) (*schema.TypeSpec, error) {
	switch t.Constructor {
	case "!Input":
		if len(t.Args) != 1 {
			return nil, fmt.Errorf("!Input accepts a single type argument")
		}
		spec, err := t.Args[0].spec(p, module, output)
		if err != nil {
			return nil, err
		}
		spec.Plain = false
		return spec, nil
	case "!Array":
		if len(t.Args) != 1 {
			return nil, fmt.Errorf("!Array accepts a single type argument")
		}
		element, err := t.Args[0].spec(p, module, output)
		if err != nil {
			return nil, err
		}
		return &schema.TypeSpec{Type: "array", Items: element, Plain: !output}, nil
	case "!Map":
		if len(t.Args) != 1 {
			return nil, fmt.Errorf("!Map accepts a single type argument")
		}
		element, err := t.Args[0].spec(p, module, output)
		if err != nil {
			return nil, err
		}
		return &schema.TypeSpec{Type: "object", AdditionalProperties: element, Plain: !output}, nil
	case "!Object":
		return nil, fmt.Errorf("NYI: anonymous object types")
	case "!Union":
		if len(t.Args) < 2 {
			return nil, fmt.Errorf("!Union requires at least two type arguments")
		}
		oneOf := make([]schema.TypeSpec, len(t.Args))
		for i, e := range t.Args {
			element, err := e.spec(p, module, output)
			if err != nil {
				return nil, err
			}
			oneOf[i] = *element
		}
		return &schema.TypeSpec{OneOf: oneOf, Plain: !output}, nil
	case "!Spec":
		return t.Spec, nil
	case "boolean", "string", "integer", "number":
		return &schema.TypeSpec{Type: t.Constructor, Plain: !output}, nil
	default:
		refPath := t.Constructor
		if !path.IsAbs(refPath) {
			refPath = path.Clean(path.Join(module, t.Constructor))
		}
		components := strings.Split(refPath, "/")[1:]

		members := p.Members
		if len(components) > 1 {
			if _, ok := p.Members[components[0]]; !ok {
				components = strings.Split(t.Constructor, "/")[1:]
				if imp, ok := p.Imports[components[0]]; ok {
					section := components[1]
					if len(components) == 2 {
						return &schema.TypeSpec{Ref: fmt.Sprintf("%s#/%s", imp.Path, section)}, nil
					}
					module := components[2 : len(components)-1]
					name := components[len(components)-1]
					token := fmt.Sprintf("%s:%s:%s", imp.Package, strings.Join(module, "/"), name)
					return &schema.TypeSpec{Ref: fmt.Sprintf("%s#/%s/%s", imp.Path, section, token)}, nil
				}
			}

			for _, c := range components[:len(components)-1] {
				next, ok := members[c]
				if !ok {
					return nil, fmt.Errorf("unknown member %v in %v", c, refPath)
				}
				mod, ok := next.(Members)
				if !ok {
					return nil, fmt.Errorf("member %v in %v is not a module", c, refPath)
				}
				members = mod
			}
		}

		leaf := components[len(components)-1]
		ref, ok := members[leaf]
		if !ok {
			return nil, fmt.Errorf("unknown member %v in %v", leaf, refPath)
		}

		kind := ""
		switch ref.(type) {
		case *Enum, *Object:
			kind = "types"
		case *Resource, *Component:
			kind = "resources"
		default:
			return nil, fmt.Errorf("types may only reference other types or resources")
		}

		moduleRef := ""
		if len(components) > 1 {
			moduleRef = strings.Join(components[:len(components)-1], "/")
		}

		return &schema.TypeSpec{Ref: fmt.Sprintf("#/%s/%s:%s:%s", kind, p.Name, moduleRef, leaf), Plain: !output}, nil
	}
}

type DefaultData struct {
	Value interface{} `yaml:"value"`
	Env   []string    `yaml:"env"`
}

type Default struct {
	DefaultData
}

func (d *Default) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.MappingNode {
		return node.Decode(&d.DefaultData)
	}
	return node.Decode(&d.Value)
}

func (d *Default) spec() (value interface{}, spec *schema.DefaultSpec) {
	if d != nil {
		value = d.Value
		if len(d.Env) != 0 {
			spec = &schema.DefaultSpec{Environment: d.Env}
		}
	}
	return
}

func properties(pkg *Package, module string, props Properties,
	output bool) (specs map[string]schema.PropertySpec, required []string, err error) {

	specs = map[string]schema.PropertySpec{}
	for name, prop := range props {
		isRequired, spec, err := prop.spec(pkg, module, output)
		if err != nil {
			return nil, nil, err
		}
		if isRequired {
			required = append(required, name)
		}
		specs[name] = *spec
	}
	sort.Strings(required)
	return
}

func commentText(comment string) string {
	if comment == "" {
		return ""
	}

	lines := strings.Split(comment, "\n")

	var text strings.Builder
	for _, l := range lines {
		text.WriteString(strings.TrimSpace(strings.TrimPrefix(l, "#")))
		text.WriteRune('\n')
	}

	return text.String()
}

func propertyDescription(key, value *yaml.Node) string {
	description := ""
	if key.HeadComment != "" {
		description = key.HeadComment
	}
	if value.LineComment != "" {
		if description != "" {
			description += "\n"
		}
		description += value.LineComment
	}
	return commentText(description)
}

func languageSpec(language map[string]interface{}) (map[string]json.RawMessage, error) {
	result := make(map[string]json.RawMessage)
	for lang, value := range language {
		bytes, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		result[lang] = json.RawMessage(bytes)
	}
	return result, nil
}
