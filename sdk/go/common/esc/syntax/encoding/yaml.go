// Copyright 2023, Pulumi Corporation.
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

package encoding

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/esc/syntax"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/rivo/uniseg"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/esc/internal/util"
)

// A TagDecoder decodes tagged YAML nodes. See the documentation on UnmarshalYAML for more details.
type TagDecoder interface {
	// DecodeTag decodes a tagged YAML node.
	DecodeTag(filename string, n *yaml.Node) (syntax.Node, syntax.Diagnostics, bool)
}

// YAMLSyntax is a syntax.Syntax implementation that is backed by a YAML node.
type YAMLSyntax struct {
	*yaml.Node
	rng   *hcl.Range
	path  string
	value interface{}
}

// Range returns the textual range of the YAML node, if any.
func (s YAMLSyntax) Range() *hcl.Range {
	return s.rng
}

// Path returns the path of the YAML node, if any.
func (s YAMLSyntax) Path() string {
	return s.path
}

func (s YAMLSyntax) ScalarRange(start, end int) *hcl.Range {
	if s.rng == nil || s.Kind != yaml.ScalarNode {
		return nil
	}

	// #237: handle other string styles
	if s.rng.Start.Line != s.rng.End.Line || (s.Style != 0 && s.Style != yaml.FlowStyle) {
		return nil
	}

	startPos := s.rng.Start
	startPos.Byte += start
	startPos.Column += uniseg.StringWidth(s.Value[:start])

	endPos := s.rng.Start
	endPos.Byte += end
	endPos.Column += uniseg.StringWidth(s.Value[:end])

	return &hcl.Range{
		Filename: s.rng.Filename,
		Start:    startPos,
		End:      endPos,
	}
}

func (s YAMLSyntax) HeadComment() string {
	return s.Node.HeadComment
}

func (s YAMLSyntax) LineComment() string {
	return s.Node.LineComment
}

func (s YAMLSyntax) FootComment() string {
	return s.Node.FootComment
}

func (s YAMLSyntax) Get(path resource.PropertyPath) (_ *yaml.Node, ok bool) {
	if s.Kind == yaml.DocumentNode {
		return YAMLSyntax{Node: s.Content[0]}.Get(path)
	}

	if len(path) == 0 {
		return s.Node, true
	}

	switch s.Kind {
	case yaml.SequenceNode:
		index, ok := path[0].(int)
		if !ok || index < 0 || index >= len(s.Content) {
			return nil, false
		}
		return YAMLSyntax{Node: s.Content[index]}.Get(path[1:])
	case yaml.MappingNode:
		key, ok := path[0].(string)
		if !ok {
			return nil, false
		}
		for i := 0; i < len(s.Content); i += 2 {
			keyNode, valueNode := s.Content[i], s.Content[i+1]
			if keyNode.Value == key {
				return YAMLSyntax{Node: valueNode}.Get(path[1:])
			}
		}
		return nil, false
	default:
		return nil, false
	}
}

func (s YAMLSyntax) Set(prefix, path resource.PropertyPath, new yaml.Node) (*yaml.Node, error) {
	if s.Kind == yaml.DocumentNode {
		return YAMLSyntax{Node: s.Content[0]}.Set(prefix, path, new)
	}

	if len(path) == 0 {
		s.Content = new.Content
		s.Kind = new.Kind
		s.Tag = new.Tag
		s.Value = new.Value
		return s.Node, nil
	}

	prefix = append(prefix, path[0])
	switch s.Kind {
	case 0:
		switch accessor := path[0].(type) {
		case int:
			s.Kind, s.Tag = yaml.SequenceNode, "!!seq"
		case string:
			s.Kind, s.Tag = yaml.MappingNode, "!!map"
		default:
			contract.Failf("unexpected accessor kind %T", accessor)
			return nil, nil
		}
		return s.Set(prefix[:len(prefix)-1], path, new)
	case yaml.SequenceNode:
		index, ok := path[0].(int)
		if !ok {
			return nil, fmt.Errorf("%v: key for an array must be an int", prefix)
		}
		if index < 0 || index > len(s.Content) {
			return nil, fmt.Errorf("%v: array index out of range", prefix)
		}
		if index == len(s.Content) {
			s.Content = append(s.Content, &yaml.Node{})
		}
		elem := s.Content[index]
		return YAMLSyntax{Node: elem}.Set(prefix, path[1:], new)
	case yaml.MappingNode:
		key, ok := path[0].(string)
		if !ok {
			return nil, fmt.Errorf("%v: key for a map must be a string", prefix)
		}

		var valueNode *yaml.Node
		for i := 0; i < len(s.Content); i += 2 {
			keyNode, value := s.Content[i], s.Content[i+1]
			if keyNode.Value == key {
				valueNode = value
				break
			}
		}
		if valueNode == nil {
			s.Content = append(s.Content, &yaml.Node{
				Kind:  yaml.ScalarNode,
				Value: key,
				Tag:   "!!str",
			})
			s.Content = append(s.Content, &yaml.Node{})
			valueNode = s.Content[len(s.Content)-1]
		}
		return YAMLSyntax{Node: valueNode}.Set(prefix, path[1:], new)
	default:
		return nil, fmt.Errorf("%v: expected an array or an object", prefix)
	}
}

func (s YAMLSyntax) Delete(prefix, path resource.PropertyPath) error {
	if s.Kind == yaml.DocumentNode {
		return YAMLSyntax{Node: s.Content[0]}.Delete(prefix, path)
	}

	prefix = append(prefix, path[0])
	switch s.Kind {
	case yaml.SequenceNode:
		index, ok := path[0].(int)
		if !ok {
			return fmt.Errorf("%v: key for an array must be an int", prefix)
		}
		if index < 0 || index >= len(s.Content) {
			return fmt.Errorf("%v: array index out of range", prefix)
		}
		if len(path) == 1 {
			s.Content = append(s.Content[:index], s.Content[index+1:]...)
			return nil
		}
		elem := s.Content[index]
		return YAMLSyntax{Node: elem}.Delete(prefix, path[1:])
	case yaml.MappingNode:
		key, ok := path[0].(string)
		if !ok {
			return fmt.Errorf("%v: key for a map must be a string", prefix)
		}

		i := 0
		for ; i < len(s.Content); i += 2 {
			if s.Content[i].Value == key {
				break
			}
		}
		if len(path) == 1 {
			if i != len(s.Content) {
				s.Content = append(s.Content[:i], s.Content[i+2:]...)
			}
			return nil
		}
		valueNode := s.Content[i+1]
		return YAMLSyntax{Node: valueNode}.Delete(prefix, path[1:])
	default:
		return fmt.Errorf("%v: expected an array or an object", prefix)
	}
}

type linePosition struct {
	offset int
	ascii  bool
	line   []byte
}

type positionIndex struct {
	lines []linePosition
	path  []any
}

func (p positionIndex) pathString() string {
	var pathString string
	for _, s := range p.path {
		switch s := s.(type) {
		case int:
			pathString = fmt.Sprintf("%s[%d]", pathString, s)
		case string:
			pathString = util.JoinKey(pathString, s)
		}
	}
	return pathString
}

// isASCII returns true if s only contains ASCII bytes. ASCII bytes are in the range [0x00,0x7f]. Any byte outside this
// range (i.e. any byte with the high bit set) is non-ASCII.
func isASCII(s []byte) bool {
	for _, b := range s {
		if b&0x80 != 0 {
			return false
		}
	}
	return true
}

func newPositionIndex(yaml []byte) positionIndex {
	offset, lines, path := 0, []linePosition(nil), []any(nil)
	for {
		line, rest, found := bytes.Cut(yaml, []byte{'\n'})

		lines = append(lines, linePosition{offset: offset, ascii: isASCII(line), line: line})
		if !found {
			return positionIndex{lines, path}
		}
		offset, yaml = offset+len(line)+1, rest
	}
}

func (p positionIndex) pos(line, column int) hcl.Pos {
	if line < 0 || line >= len(p.lines) {
		return hcl.Pos{Line: line, Column: column}
	}

	l := p.lines[line-1]
	if l.ascii {
		b := l.offset + column - 1
		return hcl.Pos{Byte: b, Line: line, Column: column}
	}

	b, rest, state, c := l.offset, l.line, -1, 1
	for len(rest) > 0 && c < column {
		cluster, r, w, s := uniseg.Step(rest, state)
		b, c = b+len(cluster), c+w>>uniseg.ShiftWidth
		rest, state = r, s
	}
	return hcl.Pos{Byte: b, Line: line, Column: column}
}

// yamlEndPos calculates the end position of a YAML node.
//
// For simple scalars, this is reasonably accurate: the end position is (start line + the number of lines, start
// column + the length of the last line).
//
// For sequences and mappings, the end position of the last node in the sequence or mapping is used as the end position
// of the sequence or mapping itself. This works well for block-style sequences/mappings, but misses the closing token
// for flow-style sequences/mappings.
func (p positionIndex) yamlEndPos(n *yaml.Node) hcl.Pos {
	switch n.Kind {
	case yaml.DocumentNode, yaml.SequenceNode, yaml.MappingNode:
		if len(n.Content) != 0 {
			return p.yamlEndPos(n.Content[len(n.Content)-1])
		}
		return p.pos(n.Line, n.Column)
	default:
		line, col, s := n.Line, n.Column, n.Value
		switch n.Style {
		case yaml.LiteralStyle:
			for {
				nl := strings.IndexByte(s, '\n')
				if nl == -1 {
					break
				}
				line, s = line+1, s[nl+1:]
			}
		case yaml.TaggedStyle:
			col += len(n.Tag) + 1
		}
		return p.pos(line, col+len(s))
	}
}

// yamlNodeRange returns a Range for the given YAML node.
func (p positionIndex) yamlNodeRange(filename string, n *yaml.Node) *hcl.Range {
	startPos := p.pos(n.Line, n.Column)
	endPos := p.yamlEndPos(n)
	return &hcl.Range{Filename: filename, Start: startPos, End: endPos}
}

// UnmarshalYAMLNode unmarshals the given YAML node into a syntax Node. UnmarshalYAMLNode does _not_ use the tag decoder
// for the node itself, though it does use the tag decoder for the node's children. This allows tag decoders to call
// UnmarshalYAMLNode without infinitely recurring on the same node. See UnmarshalYAML for more details.
func UnmarshalYAMLNode(filename string, n *yaml.Node, tags TagDecoder) (syntax.Node, syntax.Diagnostics) {
	return unmarshalYAMLNode(filename, positionIndex{}, n, tags)
}

func unmarshalYAMLNode(filename string, positions positionIndex, n *yaml.Node, tags TagDecoder) (syntax.Node, syntax.Diagnostics) {
	rng := positions.yamlNodeRange(filename, n)
	path := positions.pathString()

	var diags syntax.Diagnostics
	switch n.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		var elements []syntax.Node
		if len(n.Content) != 0 {
			elements = make([]syntax.Node, len(n.Content))
			for i, v := range n.Content {
				pos := positions
				pos.path = append(pos.path, i)
				e, ediags := unmarshalYAML(filename, pos, v, tags)
				diags.Extend(ediags...)

				elements[i] = e
			}
		}
		return syntax.ArraySyntax(YAMLSyntax{n, rng, path, nil}, elements...), diags
	case yaml.MappingNode:
		var entries []syntax.ObjectPropertyDef
		if len(n.Content) != 0 {
			// mappings are represented as a sequence of the form [key_0, value_0, ... key_n, value_n]
			numEntries := len(n.Content) / 2
			entries = make([]syntax.ObjectPropertyDef, numEntries)
			for i := range entries {
				keyNode, valueNode := n.Content[2*i], n.Content[2*i+1]

				pos := positions
				accessor := keyNode.Value
				pos.path = append(pos.path, accessor)

				keyn, kdiags := unmarshalYAML(filename, pos, keyNode, tags)
				diags.Extend(kdiags...)

				key, ok := keyn.(*syntax.StringNode)
				if !ok {
					keyRange := keyn.Syntax().Range()
					diags.Extend(syntax.Error(keyRange, "mapping keys must be strings", keyn.Syntax().Path()))
				}

				value, vdiags := unmarshalYAML(filename, pos, valueNode, tags)
				diags.Extend(vdiags...)

				entries[i] = syntax.ObjectPropertySyntax(YAMLSyntax{keyNode, rng, pos.pathString(), nil}, key, value)
			}
		}
		return syntax.ObjectSyntax(YAMLSyntax{n, rng, path, nil}, entries...), diags
	case yaml.ScalarNode:
		var v interface{}
		if err := n.Decode(&v); err != nil {
			diags.Extend(syntax.Error(rng, err.Error(), path))
			return nil, diags
		}
		if v == nil {
			return syntax.NullSyntax(YAMLSyntax{n, rng, path, nil}), nil
		}

		switch v := v.(type) {
		case bool:
			return syntax.BooleanSyntax(YAMLSyntax{n, rng, path, v}, v), nil
		case float64:
			nv := syntax.AsNumber(v)
			return syntax.NumberSyntax(YAMLSyntax{n, rng, path, nv}, nv), nil
		case int:
			nv := syntax.AsNumber(v)
			return syntax.NumberSyntax(YAMLSyntax{n, rng, path, nv}, nv), nil
		case int64:
			nv := syntax.AsNumber(v)
			return syntax.NumberSyntax(YAMLSyntax{n, rng, path, nv}, nv), nil
		case uint64:
			nv := syntax.AsNumber(v)
			return syntax.NumberSyntax(YAMLSyntax{n, rng, path, nv}, nv), nil
		default:
			return syntax.StringSyntax(YAMLSyntax{n, rng, path, v}, n.Value), nil
		}
	case yaml.AliasNode:
		return nil, syntax.Diagnostics{syntax.Error(rng, "alias nodes are not supported", path)}
	default:
		return nil, syntax.Diagnostics{syntax.Error(rng, fmt.Sprintf("unexpected node kind %v", n.Kind), path)}
	}
}

// UnmarshalYAML unmarshals a YAML node into a syntax node.
//
// Nodes are decoded as follows:
// - Scalars are decoded as the corresponding literal type (null -> nullNode, bool -> BooleanNode, etc.)
// - Sequences are decoded as array nodes
// - Mappings are decoded as object nodes
//
// Tagged nodes are decoded using the given TagDecoder. To avoid infinite recursion, the TagDecoder must call
// UnmarshalYAMLNode if it needs to unmarshal the node it is processing.
func UnmarshalYAML(filename string, n *yaml.Node, tags TagDecoder) (syntax.Node, syntax.Diagnostics) {
	return unmarshalYAML(filename, positionIndex{}, n, tags)
}

func unmarshalYAML(filename string, positions positionIndex, n *yaml.Node, tags TagDecoder) (syntax.Node, syntax.Diagnostics) {
	if tags != nil {
		if s, diags, ok := tags.DecodeTag(filename, n); ok {
			return s, diags
		}
	}
	return unmarshalYAMLNode(filename, positions, n, tags)
}

// MarshalYAML marshals a syntax node into a YAML node. If a syntax node has an associated YAMLSyntax annotation,
// the tag, style, and comments on the result will be pulled from the YAMLSyntax. The marshaling process otherwise
// follows the inverse of the unmarshaling process described in the documentation for UnmarshalYAML.
func MarshalYAML(n syntax.Node) (*yaml.Node, syntax.Diagnostics) {
	if n == nil {
		return &yaml.Node{}, syntax.Diagnostics{syntax.Error(nil, "nil nodes are not supported", "")}
	}

	var yamlNode yaml.Node
	var originalValue interface{}
	switch s := n.Syntax().(type) {
	case YAMLSyntax:
		yamlNode.Tag = s.Tag
		yamlNode.Value = s.Value
		yamlNode.Style = s.Style
		yamlNode.HeadComment = s.Node.HeadComment
		yamlNode.LineComment = s.Node.LineComment
		yamlNode.FootComment = s.Node.FootComment

		originalValue = s.value
	case syntax.Trivia:
		yamlNode.HeadComment = s.HeadComment()
		yamlNode.LineComment = s.LineComment()
		yamlNode.FootComment = s.FootComment()
	}

	var diags syntax.Diagnostics
	switch n := n.(type) {
	case *syntax.NullNode:
		yamlNode.Kind = yaml.ScalarNode
		if yamlNode.Tag != "" && yamlNode.Tag != "!!null" {
			yamlNode.Tag = "!!null"
		}
		switch yamlNode.Value {
		case "null", "Null", "NULL", "~":
			// OK
		default:
			yamlNode.Value = "null"
		}
	case *syntax.BooleanNode:
		yamlNode.Kind = yaml.ScalarNode
		if yamlNode.Tag != "" && yamlNode.Tag != "!!bool" {
			yamlNode.Tag = "!!bool"
		}
		if originalValue != n.Value() {
			yamlNode.Value = strconv.FormatBool(n.Value())
		}
	case *syntax.NumberNode:
		yamlNode.Kind = yaml.ScalarNode

		if yamlNode.Tag != "" && yamlNode.Tag != "!!int" && yamlNode.Tag != "!!float" {
			if _, err := n.Value().Int64(); err == nil {
				yamlNode.Tag = "!!int"
			} else {
				yamlNode.Tag = "!!float"
			}
		}
		if originalValue != n.Value() {
			yamlNode.Value = string(n.Value())
		}
	case *syntax.StringNode:
		value := n.Value()
		yamlNode.Kind = yaml.ScalarNode
		if yamlNode.Tag != "" && yamlNode.Tag != "!!str" {
			yamlNode.Tag = "!!str"
		}
		if _, err := strconv.ParseFloat(value, 32); err == nil || value == "true" || value == "false" {
			yamlNode.Style = yaml.SingleQuotedStyle
		}
		if originalValue != value {
			yamlNode.Value = value
		}
	case *syntax.ArrayNode:
		if yamlNode.Kind != yaml.SequenceNode && yamlNode.Kind != yaml.DocumentNode {
			yamlNode.Kind = yaml.SequenceNode
		}

		var content []*yaml.Node
		if n.Len() != 0 {
			content = make([]*yaml.Node, n.Len())
			for i := range content {
				e, ediags := MarshalYAML(n.Index(i))
				diags.Extend(ediags...)

				content[i] = e
			}
		}
		yamlNode.Content = content
	case *syntax.ObjectNode:
		yamlNode.Kind = yaml.MappingNode

		var content []*yaml.Node
		if n.Len() != 0 {
			content = make([]*yaml.Node, 2*n.Len())
			for i := 0; i < n.Len(); i++ {
				kvp := n.Index(i)

				k, kdiags := MarshalYAML(kvp.Key)
				diags.Extend(kdiags...)

				v, vdiags := MarshalYAML(kvp.Value)
				diags.Extend(vdiags...)

				content[2*i], content[2*i+1] = k, v
			}
		}
		yamlNode.Content = content
	}

	return &yamlNode, diags
}

type yamlValue struct {
	filename  string
	positions positionIndex
	node      syntax.Node
	tags      TagDecoder
	diags     syntax.Diagnostics
}

func (v *yamlValue) UnmarshalYAML(n *yaml.Node) error {
	v.node, v.diags = unmarshalYAML(v.filename, v.positions, n, v.tags)
	return nil
}

// DecodeYAMLBytes decodes a YAML value from the given decoder into a syntax node. See UnmarshalYAML for mode details on the
// decoding process.
func DecodeYAMLBytes(filename string, bytes []byte, tags TagDecoder) (syntax.Node, syntax.Diagnostics) {
	// If this is an empty file, return an empty object node.
	if len(bytes) == 0 {
		return &syntax.ObjectNode{}, nil
	}
	v := yamlValue{filename: filename, positions: newPositionIndex(bytes), tags: tags}
	if err := yaml.Unmarshal(bytes, &v); err != nil {
		return nil, syntax.Diagnostics{syntax.Error(nil, err.Error(), "")}
	}
	return v.node, v.diags
}

// DecodeYAML decodes a YAML value from the given decoder into a syntax node. See UnmarshalYAML for mode details on the
// decoding process.
func DecodeYAML(filename string, d *yaml.Decoder, tags TagDecoder) (syntax.Node, syntax.Diagnostics) {
	v := yamlValue{filename: filename, tags: tags}
	if err := d.Decode(&v); err != nil {
		if errors.Is(err, io.EOF) {
			return &syntax.ObjectNode{}, v.diags
		}
		return nil, syntax.Diagnostics{syntax.Error(nil, err.Error(), "")}
	}
	return v.node, v.diags
}

// EncodeYAML encodes a syntax node into YAML text using the given encoder. See MarshalYAML for mode details on the
// encoding process.
func EncodeYAML(e *yaml.Encoder, n syntax.Node) syntax.Diagnostics {
	yamlNode, diags := MarshalYAML(n)
	if err := e.Encode(yamlNode); err != nil {
		diags.Extend(syntax.Error(nil, err.Error(), ""))
	}
	return diags
}
