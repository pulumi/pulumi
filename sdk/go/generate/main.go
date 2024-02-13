// Copyright 2016-2023, Pulumi Corporation.
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
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	codegenrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
	"google.golang.org/protobuf/encoding/protojson"
)

func format(fullname string) {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("failed to get current working directory: %v", err)
	}

	abs, err := filepath.Abs(fullname)
	if err != nil {
		log.Fatalf("failed to get absolute path for %q: %v", fullname, err)
	}

	parent := filepath.Dir(cwd)
	rel, err := filepath.Rel(parent, abs)
	if err != nil {
		log.Fatalf("failed to get relative path for %q from %q: %v", abs, parent, err)
	}

	gofmt := exec.Command("gofumpt", "-w", rel)
	gofmt.Dir = parent

	stderr, err := gofmt.StderrPipe()
	if err != nil {
		log.Fatalf("failed to pipe stderr from gofmt: %v", err)
	}
	go func() {
		_, err := io.Copy(os.Stderr, stderr)
		if err != nil {
			panic(fmt.Sprintf("unexpected error running gofmt: %v", err))
		}
	}()
	if err := gofmt.Run(); err != nil {
		log.Printf("failed to gofmt %v: %v", fullname, err)
	}
}

func allUpper(name string) bool {
	for _, c := range name {
		if c >= 'a' && c <= 'z' {
			return false
		}
	}
	return true
}

func protobufNameToGoName(name string) string {
	// A name looks like "separate_word" or "separateWord", either way this should become "SeparateWord".

	nameParts := strings.Split(name, "_")
	goName := ""
	for _, part := range nameParts {
		isFirst := true
		goName += strings.Map(func(r rune) rune {
			if isFirst {
				isFirst = false
				return unicode.ToUpper(r)
			}
			return r
		}, part)
	}
	return goName
}

func protobufTypeToGoType(name string) string {
	// A protobuf type might look like pulumirpc.codegen.Mapper, in this case we want to take the last part as
	// the type name, but the first part needs mapping to the go module name.
	parts := strings.Split(name, ".")
	module := strings.Join(parts[:len(parts)-1], ".")
	switch module {
	case "pulumirpc":
		return "pulumirpc." + parts[len(parts)-1]
	case "pulumirpc.codegen":
		return "codegenrpc." + parts[len(parts)-1]
	default:
		panic(fmt.Sprintf("unexpected protobuf module: %s", module))
	}
}

func pulumiNameToGoName(name string) string {
	// A name looks like "separate_word_AKA", that is each part is separated by "_" and acronyms are
	// uppercase.

	nameParts := strings.Split(name, "_")
	goName := ""
	titleCaser := cases.Title(language.English)
	for _, part := range nameParts {
		if allUpper(part) {
			goName += part
			continue
		}
		goName += titleCaser.String(part)
	}
	return goName
}

func refToGoType(ref string) string {
	parts := strings.Split(ref, ".")
	return pulumiNameToGoName(parts[len(parts)-1])
}

func pulumiTypeToGoType(typ *codegenrpc.TypeReference) string {
	switch e := typ.Element.(type) {
	case *codegenrpc.TypeReference_Primitive:
		switch e.Primitive {
		case codegenrpc.PrimitiveType_BOOL:
			return "bool"
		case codegenrpc.PrimitiveType_BYTE:
			return "byte"
		case codegenrpc.PrimitiveType_INT:
			return "int"
		case codegenrpc.PrimitiveType_STRING:
			return "string"
		case codegenrpc.PrimitiveType_DURATION:
			return "time.Duration"
		case codegenrpc.PrimitiveType_PROPERTY_VALUE:
			return "resource.PropertyValue"
		}
	case *codegenrpc.TypeReference_Ref:
		return refToGoType(e.Ref)
	case *codegenrpc.TypeReference_Map:
		// Special case for PropertyMap.
		if e.Map.GetPrimitive() == codegenrpc.PrimitiveType_PROPERTY_VALUE {
			return "resource.PropertyMap"
		}
		return "map[string]" + pulumiTypeToGoType(e.Map)
	case *codegenrpc.TypeReference_Array:
		return "[]" + pulumiTypeToGoType(e.Array)
	}

	log.Fatalf("unhandled type: %v", typ)
	return ""
}

func needsMarshal(typ *codegenrpc.TypeReference) bool {
	switch e := typ.Element.(type) {
	case *codegenrpc.TypeReference_Primitive:
		switch e.Primitive {
		case codegenrpc.PrimitiveType_PROPERTY_VALUE:
			return true
		}
	case *codegenrpc.TypeReference_Ref:
		return true
	}

	return false
}

func toTemplateData(typeName string, data map[string]interface{}) string {
	parts := strings.Split(typeName, ".")

	data["Name"] = pulumiNameToGoName(parts[len(parts)-1])
	if len(parts) > 1 {
		data["Package"] = parts[len(parts)-2]
	} else {
		data["Package"] = "pulumi"
	}

	path := filepath.Join(parts[:len(parts)-1]...)
	fullname := filepath.Join("..", path, parts[len(parts)-1]+".go")
	return fullname
}

func executeTemplate(templates *template.Template, templateName, templatePath string, data map[string]interface{}) {
	log.Printf("Writing %v with template %s using data %v", templatePath, templateName, data)
	if err := os.MkdirAll(filepath.Dir(templatePath), 0755); err != nil {
		log.Fatalf("create directory %q: %v", filepath.Dir(templatePath), err)
	}

	f, err := os.Create(templatePath)
	if err != nil {
		log.Fatalf("create %q: %v", templatePath, err)
	}
	if err := templates.ExecuteTemplate(f, templateName, data); err != nil {
		log.Fatalf("execute %q: %v", templateName, err)
	}
	f.Close()

	format(templatePath)
}

// Generate the code for a Pulumi package.
func main() {
	coreSchemaJson, err := os.ReadFile("../../../proto/core.json")
	if err != nil {
		log.Fatalf("read core.json: %v", err)
	}

	var core codegenrpc.Core
	if err := protojson.Unmarshal(coreSchemaJson, &core); err != nil {
		log.Fatalf("parse core schema: %v", err)
	}

	templates := template.New("templates")
	templates.Funcs(template.FuncMap{
		"lower": strings.ToLower,
		"split": strings.Split,
	})

	templates, err = templates.ParseGlob("./templates/*")
	if err != nil {
		log.Fatalf("failed to parse templates: %v", err)
	}

	for _, typ := range core.Sdk.TypeDeclarations {
		log.Printf("Generating %v", typ)

		switch e := typ.Element.(type) {
		case *codegenrpc.TypeDeclaration_Record:
			properties := make([]interface{}, 0)
			for _, prop := range e.Record.Properties {
				goName := pulumiNameToGoName(prop.Name)
				protobufGoName := protobufNameToGoName(prop.ProtobufField)

				var marshalCode, unmarshalCode string
				if protobufGoName == "" {
					// This must have a custom mapping
					switch prop.ProtobufMapping {
					case codegenrpc.CustomPropertyMapping_URN_NAME:
						unmarshalCode = fmt.Sprintf("s.%s = string(resource.URN(data.Urn).Name())", goName)
					case codegenrpc.CustomPropertyMapping_URN_TYPE:
						unmarshalCode = fmt.Sprintf("s.%s = string(resource.URN(data.Urn).Type())", goName)
					}
				} else {
					if needsMarshal(prop.Type) {
						marshalCode = fmt.Sprintf("%s: s.%s.marshal()", protobufGoName, goName)
						unmarshalCode = fmt.Sprintf("s.%s.unmarshal(data.%s)", goName, protobufGoName)
					} else {
						marshalCode = fmt.Sprintf("%s: s.%s", protobufGoName, goName)
						unmarshalCode = fmt.Sprintf("s.%s = data.%s", goName, protobufGoName)
					}
				}
				templateProp := map[string]interface{}{
					"Name":                  goName,
					"Description":           strings.Split(prop.Description, "\n"),
					"Type":                  pulumiTypeToGoType(prop.Type),
					"Marshal":               marshalCode,
					"Unmarshal":             unmarshalCode,
					"ProtobufPresenceField": protobufNameToGoName(prop.ProtobufPresenceField),
				}

				properties = append(properties, templateProp)
			}

			data := map[string]interface{}{
				"Description":     strings.Split(e.Record.Description, "\n"),
				"Properties":      properties,
				"ProtobufMessage": protobufTypeToGoType(e.Record.ProtobufMessage),
			}

			templatePath := toTemplateData(e.Record.Name, data)
			executeTemplate(templates, "record.go.template", templatePath, data)

		case *codegenrpc.TypeDeclaration_Enumeration:
			values := make([]interface{}, 0)
			for _, value := range e.Enumeration.Values {
				templateValue := map[string]interface{}{
					"Name":          pulumiNameToGoName(value.Name),
					"Description":   strings.Split(value.Description, "\n"),
					"ProtobufValue": value.ProtobufValue,
				}

				values = append(values, templateValue)
			}

			// e.Enumeration.ProtobufEnum may be nested e.g. "pulumirpc.DiffResponse.Kind", for such a name the Go type
			// is "pulumirpc.DiffResponse_Kind" and the namespace before the enum values is "pulumirpc.DiffResponse". For a root
			// type e.g. "pulumirpc.LogSeverity" the type and namespace are the same.
			parts := strings.Split(e.Enumeration.ProtobufEnum, ".")
			var protobufType, protobufNamespace string
			if len(parts) == 2 {
				protobufType = fmt.Sprintf("%s.%s", parts[0], parts[1])
				protobufNamespace = parts[1]
			} else if len(parts) == 3 {
				protobufType = fmt.Sprintf("%s.%s_%s", parts[0], parts[1], parts[2])
				protobufNamespace = parts[1]
			} else {
				panic("unexpected protobuf enum name: " + e.Enumeration.ProtobufEnum)
			}

			data := map[string]interface{}{
				"Description":       strings.Split(e.Enumeration.Description, "\n"),
				"Values":            values,
				"ProtobufType":      protobufType,
				"ProtobufNamespace": protobufNamespace,
			}

			templatePath := toTemplateData(e.Enumeration.Name, data)
			executeTemplate(templates, "enum.go.template", templatePath, data)

		case *codegenrpc.TypeDeclaration_Interface:
			methods := make([]interface{}, 0)
			for _, method := range e.Interface.Methods {
				param := map[string]interface{}{
					"Name":        method.Request.Name,
					"Description": strings.Split(method.Request.Description, "\n"),
					"Type":        refToGoType(method.Request.Type),
				}

				templateMethod := map[string]interface{}{
					"Name":        pulumiNameToGoName(method.Name),
					"Description": strings.Split(method.Description, "\n"),
					"Parameter":   param,
					"GrpcMethod":  method.GrpcMethod,
				}
				if method.ResponseType != "" {
					templateMethod["ReturnType"] = refToGoType(method.ResponseType)
				}

				methods = append(methods, templateMethod)
			}

			data := map[string]interface{}{
				"Description": strings.Split(e.Interface.Description, "\n"),
				"Methods":     methods,
				"GrpcService": protobufTypeToGoType(e.Interface.GrpcService),
			}

			templatePath := toTemplateData(e.Interface.Name, data)
			executeTemplate(templates, "interface.go.template", templatePath, data)

			// If we need grpc server/client add them
			if e.Interface.GrpcKind == codegenrpc.GrpcKind_KIND_BOTH ||
				e.Interface.GrpcKind == codegenrpc.GrpcKind_KIND_SERVER {
				templatePath := filepath.Join(filepath.Dir(templatePath), "server_"+filepath.Base(templatePath))
				executeTemplate(templates, "grpc_server.go.template", templatePath, data)
			}
			if e.Interface.GrpcKind == codegenrpc.GrpcKind_KIND_BOTH ||
				e.Interface.GrpcKind == codegenrpc.GrpcKind_KIND_CLIENT {
				templatePath := filepath.Join(filepath.Dir(templatePath), "client_"+filepath.Base(templatePath))
				executeTemplate(templates, "grpc_client.go.template", templatePath, data)
			}

		default:
			log.Fatalf("unexpected type declaration: %v", typ.Element)
		}
	}
}
