package main

import (
	"fmt"
	"strings"

	"github.com/yoheimuta/go-protoparser/v4/parser"
)

type StringEnumField struct {
	Name        string
	Description string
	JsonField   string
}

type StringEnum struct {
	Name        string
	Description string
	Fields      []StringEnumField
}

type FieldInfo struct {
	Name        string
	Description string
	Type        FieldType
	Required    bool
	JsonField   string
}

type SharedTypeDefinition struct {
	Name        string
	Description string
	Fields      map[string]FieldInfo
	BaseType    *string
	Abstract    bool
}

type ModuleInfo struct {
	Name    string
	Imports []string
	Enums   []StringEnum
	Types   []SharedTypeDefinition
}

func readComments(comments []*parser.Comment) string {
	fullComment := ""
	for i, comment := range comments {
		fullComment += strings.TrimPrefix(comment.Raw, "//")
		if i < len(comments)-1 {
			fullComment += "\n"
		}
	}
	return fullComment
}

func trimPrefixSuffix(s string, prefix string, suffix string) string {
	return strings.TrimSuffix(strings.TrimPrefix(s, prefix), suffix)
}

func parseType(fieldType string) FieldType {
	switch trimPrefixSuffix(fieldType, " ", " ") {
	case "string":
		return &StringType{}
	case "int32":
		return &IntType{}
	case "float", "double":
		return &FloatType{}
	case "bool":
		return &BoolType{}
	case "google.protobuf.Any":
		return &AnyType{}
	case "google.protobuf.Struct":
		return &MapType{
			KeyType:   &StringType{},
			ValueType: &AnyType{},
		}
	default:
		if strings.HasPrefix(fieldType, "map") {
			elementTypes := strings.Split(trimPrefixSuffix(fieldType, "map<", ">"), ",")
			if len(elementTypes) != 2 {
				panic("Expected exactly two element types in map type")
			}
			keyType := parseType(elementTypes[0])
			valueType := parseType(elementTypes[1])
			return &MapType{KeyType: keyType, ValueType: valueType}
		}

		return &RefType{Reference: fieldType}
	}
}

func readModuleInfo(proto *parser.Proto) ModuleInfo {
	moduleInfo := ModuleInfo{}
	for _, part := range proto.ProtoBody {
		switch bodyPart := part.(type) {
		case *parser.Package:
			moduleInfo.Name = bodyPart.Name
		case *parser.Enum:
			enum := StringEnum{}
			enum.Name = bodyPart.EnumName
			enum.Description = readComments(bodyPart.Comments)
			for _, enumPart := range bodyPart.EnumBody {
				if enumField, ok := enumPart.(*parser.EnumField); ok {
					field := StringEnumField{}
					enumFieldName := enumField.Ident
					if strings.HasPrefix(enumFieldName, fmt.Sprintf("%s_", enum.Name)) {
						field.Name = strings.TrimPrefix(enumFieldName, fmt.Sprintf("%s_", enum.Name))
					} else {
						field.Name = enumField.Ident
					}

					field.Name = enumField.Ident
					field.Description = readComments(enumField.Comments)
					for _, option := range enumField.EnumValueOptions {
						if option.OptionName == "options.string_enum" {
							field.JsonField = option.Constant
						}
					}
					enum.Fields = append(enum.Fields, field)
				}
			}

			moduleInfo.Enums = append(moduleInfo.Enums, enum)
		case *parser.Message:
			sharedType := SharedTypeDefinition{}
			sharedType.Name = bodyPart.MessageName
			sharedType.Description = readComments(bodyPart.Comments)
			sharedType.Fields = make(map[string]FieldInfo)
			for _, messagePart := range bodyPart.MessageBody {
				if option, ok := messagePart.(*parser.Option); ok {
					if option.OptionName == "options.base_type" {
						sharedType.BaseType = &option.Constant
					}
					if option.OptionName == "options.abstract" && option.Constant == "true" {
						sharedType.Abstract = true
					}
				}

				if field, ok := messagePart.(*parser.Field); ok {
					fieldInfo := FieldInfo{}
					fieldInfo.Name = field.FieldName
					fieldInfo.Description = readComments(field.Comments)
					if field.IsRepeated {
						fieldInfo.Type = &ListType{ElementType: parseType(field.Type)}
					} else {
						fieldInfo.Type = parseType(field.Type)
					}

					fieldInfo.Required = !field.IsOptional
					fieldInfo.JsonField = field.FieldName

					for _, option := range field.FieldOptions {
						if option.OptionName == "options.json_field" {
							fieldInfo.JsonField = option.Constant
						}
					}
					sharedType.Fields[field.FieldName] = fieldInfo
				}
			}

			moduleInfo.Types = append(moduleInfo.Types, sharedType)
		}
	}

	return moduleInfo
}
