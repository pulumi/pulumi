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
	"bytes"
	"encoding/json"
	"log"
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	codegenrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
	"google.golang.org/protobuf/encoding/protojson"
)

func main() {
	coreSchema := getCoreSchema()
	validateCoreSchema(coreSchema)

	var rawJSON []byte
	var err error
	rawJSON, err = protojson.Marshal(coreSchema)
	if err != nil {
		log.Fatalf("cannot marshal proto message to json: %v", err)
	}

	var formattedJSON bytes.Buffer
	err = json.Indent(&formattedJSON, rawJSON, "", "  ")
	if err != nil {
		log.Fatalf("failed to format core JSON: %v", err)
	}

	err = os.WriteFile("../core.json", formattedJSON.Bytes(), 0o755)
	if err != nil {
		log.Fatalf("failed to write core.json: %v", err)
	}
}

// validate checks that the core schema is valid, i.e. every name is set etc.
func validateCoreSchema(core *codegenrpc.Core) {
	for _, t := range core.Sdk.TypeDeclarations {
		switch t := t.Element.(type) {
		case *codegenrpc.TypeDeclaration_Enumeration:
			e := t.Enumeration
			contract.Assertf(e.Name != "", "enumeration declaration has no name: %v", e)
			contract.Assertf(e.Description != "", "enumeration declaration has no description: %v", e)
			for _, v := range e.Values {
				contract.Assertf(v.Name != "", "enumeration value has no name: %v", v)
				contract.Assertf(v.Description != "", "enumeration value has no description: %v", v)
				// If this is a protobuf enum, we should have a protobuf value for each value
				if e.ProtobufEnum != "" {
					contract.Assertf(v.ProtobufValue != "", "enumeration value has no protobuf value: %v", v)
				}
			}
		case *codegenrpc.TypeDeclaration_Record:
			r := t.Record
			contract.Assertf(r.Name != "", "record declaration has no name: %v", r)
			contract.Assertf(r.Description != "", "record declaration has no description: %v", r)
			for _, p := range r.Properties {
				contract.Assertf(p.Name != "", "record property has no name: %v", p)
				contract.Assertf(p.Description != "", "record property has no description: %v", p)
				contract.Assertf(p.Type != nil, "record property has no type: %v", p)
				if r.ProtobufMessage != "" {
					if p.ProtobufField == "" && p.ProtobufMapping == codegenrpc.CustomPropertyMapping_NONE {
						contract.Assertf(false, "record property has no protobuf mapping: %v", p)
					}
				}
				if p.ProtobufField != "" || p.ProtobufMapping != codegenrpc.CustomPropertyMapping_NONE {
					if r.ProtobufMessage == "" {
						contract.Assertf(false, "record property has a protobuf mapping but record has no message: %v", r)
					}
				}
			}
		case *codegenrpc.TypeDeclaration_Interface:
			i := t.Interface
			contract.Assertf(i.Name != "", "interface declaration has no name: %v", i)
			contract.Assertf(i.Description != "", "interface declaration has no description: %v", i)
			for _, m := range i.Methods {
				contract.Assertf(m.Name != "", "interface method has no name: %v", m)
				contract.Assertf(m.Description != "", "interface method has no description: %v", m)
				// By design ResponseType can be "" to indicate the interface returns void/grpc.Empty.
				contract.Assertf(m.Request != nil, "interface method has no request: %v", m)
				if i.GrpcService != "" {
					contract.Assertf(m.GrpcMethod != "", "interface method has no protobuf name: %v", m)
				}
				if m.GrpcMethod != "" && i.GrpcService == "" {
					contract.Assertf(false, "interface method has a grpc method but interface has no service: %v", i)
				}
			}
		}
	}
}

func makeArrayType(element *codegenrpc.TypeReference) *codegenrpc.TypeReference {
	return &codegenrpc.TypeReference{
		Element: &codegenrpc.TypeReference_Array{Array: element},
	}
}

func makeMapType(element *codegenrpc.TypeReference) *codegenrpc.TypeReference {
	return &codegenrpc.TypeReference{
		Element: &codegenrpc.TypeReference_Map{Map: element},
	}
}

func makeRefType(ref string) *codegenrpc.TypeReference {
	return &codegenrpc.TypeReference{
		Element: &codegenrpc.TypeReference_Ref{Ref: ref},
	}
}

func getCoreSchema() *codegenrpc.Core {
	boolType := &codegenrpc.TypeReference{
		Element: &codegenrpc.TypeReference_Primitive{Primitive: codegenrpc.PrimitiveType_BOOL},
	}
	byteType := &codegenrpc.TypeReference{
		Element: &codegenrpc.TypeReference_Primitive{Primitive: codegenrpc.PrimitiveType_BYTE},
	}
	stringType := &codegenrpc.TypeReference{
		Element: &codegenrpc.TypeReference_Primitive{Primitive: codegenrpc.PrimitiveType_STRING},
	}
	propertyValueType := &codegenrpc.TypeReference{
		Element: &codegenrpc.TypeReference_Primitive{Primitive: codegenrpc.PrimitiveType_PROPERTY_VALUE},
	}
	durationType := &codegenrpc.TypeReference{
		Element: &codegenrpc.TypeReference_Primitive{Primitive: codegenrpc.PrimitiveType_DURATION},
	}
	propertyMapType := makeMapType(propertyValueType)

	return &codegenrpc.Core{
		Sdk: &codegenrpc.SDK{
			TypeDeclarations: []*codegenrpc.TypeDeclaration{
				{
					Element: &codegenrpc.TypeDeclaration_Enumeration{
						Enumeration: &codegenrpc.Enumeration{
							Name:         "pulumi.experimental.providers.diff_changes",
							ProtobufEnum: "pulumirpc.DiffResponse.DiffChanges",
							Description:  "TODO",
							Values: []*codegenrpc.EnumerationValue{
								{
									Name:          "unknown",
									ProtobufValue: "DIFF_UNKNOWN",
									Description:   "Unknown whether there are changes or not (legacy behavior).",
								},
								{
									Name:          "none",
									ProtobufValue: "DIFF_NONE",
									Description:   "The diff was performed, and no changes were detected that require an update.",
								},
								{
									Name:          "some",
									ProtobufValue: "DIFF_SOME",
									Description:   "The diff was performed, and changes were detected that require an update or replacement.",
								},
							},
						},
					},
				},
				{
					Element: &codegenrpc.TypeDeclaration_Enumeration{
						Enumeration: &codegenrpc.Enumeration{
							Name:         "pulumi.experimental.providers.property_diff_kind",
							ProtobufEnum: "pulumirpc.PropertyDiff.Kind",
							Description:  "TODO",
							Values: []*codegenrpc.EnumerationValue{
								{
									Name:          "add",
									ProtobufValue: "ADD",
									Description:   "This property was added.",
								},
								{
									Name:          "add_replace",
									ProtobufValue: "ADD_REPLACE",
									Description:   "This property was added, and this change requires a replace.",
								},
								{
									Name:          "delete",
									ProtobufValue: "DELETE",
									Description:   "This property was removed.",
								},
								{
									Name:          "delete_replace",
									ProtobufValue: "DELETE_REPLACE",
									Description:   "This property was removed, and this change requires a replace.",
								},
								{
									Name:          "update",
									ProtobufValue: "UPDATE",
									Description:   "This property's value was changed.",
								},
								{
									Name:          "update_replace",
									ProtobufValue: "UPDATE_REPLACE",
									Description:   "This property's value was changed, and this change requires a replace.",
								},
							},
						},
					},
				},
				{
					Element: &codegenrpc.TypeDeclaration_Enumeration{
						Enumeration: &codegenrpc.Enumeration{
							Name:         "pulumi.experimental.providers.log_severity",
							ProtobufEnum: "pulumirpc.LogSeverity",
							Description:  "The severity level of a log message. Errors are fatal; all others are informational.",
							Values: []*codegenrpc.EnumerationValue{
								{
									Name:          "debug",
									ProtobufValue: "DEBUG",
									Description:   "A debug-level message not displayed to end-users (the default).",
								},
								{
									Name:          "info",
									ProtobufValue: "INFO",
									Description:   "An informational message printed to output during resource operations.",
								},
								{
									Name:          "warning",
									ProtobufValue: "WARNING",
									Description:   "A warning to indicate that something went wrong.",
								},
								{
									Name:          "error",
									ProtobufValue: "ERROR",
									Description:   "A fatal error indicating that the tool should stop processing subsequent resource operations.",
								},
							},
						},
					},
				},
				{
					Element: &codegenrpc.TypeDeclaration_Record{
						Record: &codegenrpc.Record{
							Name:            "pulumi.experimental.providers.log_message",
							ProtobufMessage: "pulumirpc.LogRequest",
							Description:     "A log message to be sent to the Pulumi engine.",
							Properties: []*codegenrpc.Property{
								{
									Name:          "severity",
									ProtobufField: "severity",
									Description:   "The logging level of this message.",
									Type:          makeRefType("pulumi.experimental.providers.log_severity"),
								},
								{
									Name:          "message",
									ProtobufField: "message",
									Description:   "The contents of the logged message.",
									Type:          stringType,
								},
								{
									Name:          "URN",
									ProtobufField: "urn",
									Description:   "The (optional) resource urn this log is associated with.",
									Type:          stringType,
								},
								{
									Name:          "stream_id",
									ProtobufField: "streamId",
									Description: "The (optional) stream id that a stream of log messages can be associated with. This allows" +
										" clients to not have to buffer a large set of log messages that they all want to be" +
										" conceptually connected.  Instead the messages can be sent as chunks (with the same stream id)" +
										" and the end display can show the messages as they arrive, while still stitching them together" +
										" into one total log message. 0 means do not associate with any stream.",
									Type: stringType,
								},
								{
									Name:          "ephemeral",
									ProtobufField: "ephemeral",
									Description:   "Optional value indicating whether this is a status message.",
									Type:          boolType,
								},
							},
						},
					},
				},
				{
					Element: &codegenrpc.TypeDeclaration_Interface{
						Interface: &codegenrpc.Interface{
							Name:        "pulumi.experimental.providers.host",
							GrpcService: "pulumirpc.Engine",
							GrpcKind:    codegenrpc.GrpcKind_KIND_CLIENT,
							Description: "An interface to the engine host running this plugin.",
							Methods: []*codegenrpc.Method{
								{
									Name:         "log",
									GrpcMethod:   "Log",
									Description:  "Send a log message to the host.",
									ResponseType: "",
									Request: &codegenrpc.Request{
										Name:        "message",
										Description: "TODO",
										Type:        "pulumi.experimental.providers.log_message",
									},
								},
							},
						},
					},
				},
				{
					Element: &codegenrpc.TypeDeclaration_Record{
						Record: &codegenrpc.Record{
							Name:            "pulumi.experimental.providers.check_request",
							ProtobufMessage: "pulumirpc.CheckRequest",
							Description:     "A request to validate the inputs for a resource.",
							Properties: []*codegenrpc.Property{
								{
									Name:          "URN",
									ProtobufField: "urn",
									Description:   "The Pulumi URN for this resource.",
									Type:          stringType,
								},
								{
									Name:            "type",
									ProtobufMapping: codegenrpc.CustomPropertyMapping_URN_TYPE,
									Description:     "The Pulumi type for this resource.",
									Type:            stringType,
								},
								{
									Name:            "name",
									ProtobufMapping: codegenrpc.CustomPropertyMapping_URN_NAME,
									Description:     "The Pulumi name for this resource.",
									Type:            stringType,
								},
								{
									Name:          "olds",
									ProtobufField: "olds",
									Description:   "The old Pulumi inputs for this resource, if any.",
									Type:          propertyMapType,
								},
								{
									Name:          "news",
									ProtobufField: "news",
									Description:   "The new Pulumi inputs for this resource.",
									Type:          propertyMapType,
								},
								{
									Name:          "random_seed",
									ProtobufField: "randomSeed",
									Description:   "A deterministically random hash, primarily intended for global unique naming.",
									Type:          makeArrayType(byteType),
								},
							},
						},
					},
				},
				{
					Element: &codegenrpc.TypeDeclaration_Record{
						Record: &codegenrpc.Record{
							Name:            "pulumi.experimental.providers.check_failure",
							ProtobufMessage: "pulumirpc.CheckFailure",
							Description:     "Indicates that a call to check failed; it contains the property and reason for the failure.",
							Properties: []*codegenrpc.Property{
								{
									Name:          "property",
									ProtobufField: "property",
									Description:   "The property that failed validation.",
									Type:          stringType,
								},
								{
									Name:          "reason",
									ProtobufField: "reason",
									Description:   "The reason that the property failed validation.",
									Type:          stringType,
								},
							},
						},
					},
				},
				{
					Element: &codegenrpc.TypeDeclaration_Record{
						Record: &codegenrpc.Record{
							Name:            "pulumi.experimental.providers.check_response",
							ProtobufMessage: "pulumirpc.CheckResponse",
							Description:     "The response from checking a resource.",
							Properties: []*codegenrpc.Property{
								{
									Name:          "inputs",
									ProtobufField: "inputs",
									Description:   "The provider inputs for this resource.",
									Type:          propertyMapType,
								},
								{
									Name:          "failures",
									ProtobufField: "failures",
									Description:   "Any validation failures that occurred.",
									Type:          makeArrayType(makeRefType("pulumi.experimental.providers.check_failure")),
								},
							},
						},
					},
				},
				{
					Element: &codegenrpc.TypeDeclaration_Record{
						Record: &codegenrpc.Record{
							Name:            "pulumi.experimental.providers.diff_request",
							ProtobufMessage: "pulumirpc.DiffRequest",
							Description:     "A request to diff the inputs for a resource.",
							Properties: []*codegenrpc.Property{
								{
									Name:          "id",
									ProtobufField: "id",
									Description:   "The ID of the resource to diff.",
									Type:          stringType,
								},
								{
									Name:          "URN",
									ProtobufField: "urn",
									Description:   "The Pulumi URN for this resource.",
									Type:          stringType,
								},
								{
									Name:            "type",
									ProtobufMapping: codegenrpc.CustomPropertyMapping_URN_TYPE,
									Description:     "The Pulumi type for this resource.",
									Type:            stringType,
								},
								{
									Name:            "name",
									ProtobufMapping: codegenrpc.CustomPropertyMapping_URN_NAME,
									Description:     "The Pulumi name for this resource.",
									Type:            stringType,
								},
								{
									Name:          "olds",
									ProtobufField: "olds",
									Description:   "The old values of provider inputs to diff.",
									Type:          propertyMapType,
								},
								{
									Name:          "news",
									ProtobufField: "news",
									Description:   "The new values of provider inputs to diff.",
									Type:          propertyMapType,
								},
								{
									Name:          "ignore_changes",
									ProtobufField: "ignoreChanges",
									Description:   "A set of property paths that should be treated as unchanged.",
									Type:          makeArrayType(stringType),
								},
							},
						},
					},
				},
				{
					Element: &codegenrpc.TypeDeclaration_Record{
						Record: &codegenrpc.Record{
							Name:            "pulumi.experimental.providers.property_diff",
							ProtobufMessage: "pulumirpc.PropertyDiff",
							Description:     "TODO",
							Properties: []*codegenrpc.Property{
								{
									Name:          "kind",
									ProtobufField: "kind",
									Description:   "The kind of diff associated with this property.",
									Type:          stringType,
								},
								{
									Name:          "input_diff",
									ProtobufField: "inputDiff",
									Description:   "The difference is between old and new inputs, not old and new state.",
									Type:          boolType,
								},
							},
						},
					},
				},
				{
					Element: &codegenrpc.TypeDeclaration_Record{
						Record: &codegenrpc.Record{
							Name:            "pulumi.experimental.providers.diff_response",
							ProtobufMessage: "pulumirpc.DiffResponse",
							Description:     "The response from diffing a resource.",
							Properties: []*codegenrpc.Property{
								{
									Name:          "replaces",
									ProtobufField: "replaces",
									Description:   "If this update requires a replacement, the set of properties triggering it.",
									Type:          makeArrayType(stringType),
								},
								{
									Name:          "stables",
									ProtobufField: "stables",
									Description:   "An optional list of properties that will not ever change.",
									Type:          makeArrayType(stringType),
								},
								{
									Name:          "delete_before_replace",
									ProtobufField: "deleteBeforeReplace",
									Description:   "If true, this resource must be deleted before replacing it.",
									Type:          boolType,
								},
								{
									Name:          "changes",
									ProtobufField: "changes",
									Description:   "If true, this diff represents an actual difference and thus requires an update.",
									Type:          makeRefType("pulumi.experimental.providers.diff_changes"),
								},
								{
									Name:          "diffs",
									ProtobufField: "diffs",
									Description:   "A list of the properties that changed.",
									Type:          makeArrayType(stringType),
								},
								{
									Name:                  "detailed_diff",
									ProtobufField:         "detailedDiff",
									ProtobufPresenceField: "hasDetailedDiff",
									Description:           "A detailed diff appropriate for display.",
									Type:                  makeMapType(makeRefType("pulumi.experimental.providers.property_diff")),
								},
							},
						},
					},
				},
				{
					Element: &codegenrpc.TypeDeclaration_Record{
						Record: &codegenrpc.Record{
							Name:            "pulumi.experimental.providers.configure_request",
							ProtobufMessage: "pulumirpc.ConfigureRequest",
							Description:     "A request to configure a provider.",
							Properties: []*codegenrpc.Property{
								{
									Name:          "variables",
									ProtobufField: "variables",
									Description:   "A map of configuration keys to values.",
									Type:          makeMapType(stringType),
								},
								{
									Name:          "args",
									ProtobufField: "args",
									Description:   "The input properties for the provider. Only filled in for newer providers.",
									Type:          propertyMapType,
								},
								{
									Name:          "accept_secrets",
									ProtobufField: "acceptSecrets",
									Description:   "When true, operations should return secrets as strongly typed.",
									Type:          boolType,
								},
								{
									Name:          "accept_resources",
									ProtobufField: "acceptResources",
									Description:   "When true, operations should return resources as strongly typed values to the provider.",
									Type:          boolType,
								},
							},
						},
					},
				},
				{
					Element: &codegenrpc.TypeDeclaration_Record{
						Record: &codegenrpc.Record{
							Name:            "pulumi.experimental.providers.configure_response",
							ProtobufMessage: "pulumirpc.ConfigureResponse",
							Description:     "TODO",
							Properties: []*codegenrpc.Property{
								{
									Name:          "accept_secrets",
									ProtobufField: "acceptSecrets",
									Description:   "When true, the engine should pass secrets as strongly typed values to the provider.",
									Type:          boolType,
								},
								{
									Name:          "supports_preview",
									ProtobufField: "supportsPreview",
									Description:   "When true, the engine should invoke create and update with preview=true during previews.",
									Type:          boolType,
								},
								{
									Name:          "accept_resources",
									ProtobufField: "acceptResources",
									Description:   "When true, the engine should pass resources as strongly typed values to the provider.",
									Type:          boolType,
								},
								{
									Name:          "accept_outputs",
									ProtobufField: "acceptOutputs",
									Description:   "When true, the engine should pass output values to the provider.",
									Type:          boolType,
								},
							},
						},
					},
				},
				{
					Element: &codegenrpc.TypeDeclaration_Record{
						Record: &codegenrpc.Record{
							Name:            "pulumi.experimental.providers.create_request",
							ProtobufMessage: "pulumirpc.CreateRequest",
							Description:     "A request to create a resource.",
							Properties: []*codegenrpc.Property{
								{
									Name:          "URN",
									ProtobufField: "urn",
									Description:   "The Pulumi URN for this resource.",
									Type:          stringType,
								},
								{
									Name:            "type",
									ProtobufMapping: codegenrpc.CustomPropertyMapping_URN_TYPE,
									Description:     "The Pulumi type for this resource.",
									Type:            stringType,
								},
								{
									Name:            "name",
									ProtobufMapping: codegenrpc.CustomPropertyMapping_URN_NAME,
									Description:     "The Pulumi name for this resource.",
									Type:            stringType,
								},
								{
									Name:          "properties",
									ProtobufField: "properties",
									Description:   "The provider inputs to set during creation.",
									Type:          propertyMapType,
								},
								{
									Name:          "timeout",
									ProtobufField: "timeout",
									Description:   "The create request timeout.",
									Type:          durationType,
								},
								{
									Name:          "preview",
									ProtobufField: "preview",
									Description:   "true if this is a preview and the provider should not actually create the resource.",
									Type:          boolType,
								},
							},
						},
					},
				},
				{
					Element: &codegenrpc.TypeDeclaration_Record{
						Record: &codegenrpc.Record{
							Name:            "pulumi.experimental.providers.create_response",
							ProtobufMessage: "pulumirpc.CreateResponse",
							Description:     "The response from creating a resource.",
							Properties: []*codegenrpc.Property{
								{
									Name:          "id",
									ProtobufField: "id",
									Description:   "The ID of the created resource.",
									Type:          stringType,
								},
								{
									Name:          "properties",
									ProtobufField: "properties",
									Description:   "Any any properties that were computed during creation.",
									Type:          propertyMapType,
								},
							},
						},
					},
				},
				{
					Element: &codegenrpc.TypeDeclaration_Record{
						Record: &codegenrpc.Record{
							Name:            "pulumi.experimental.providers.read_request",
							ProtobufMessage: "pulumirpc.ReadRequest",
							Description:     "A request to read a resource.",
							Properties: []*codegenrpc.Property{
								{
									Name:          "id",
									ProtobufField: "id",
									Description:   "The ID of the resource to read.",
									Type:          stringType,
								},
								{
									Name:          "URN",
									ProtobufField: "urn",
									Description:   "The Pulumi URN for this resource.",
									Type:          stringType,
								},
								{
									Name:            "type",
									ProtobufMapping: codegenrpc.CustomPropertyMapping_URN_TYPE,
									Description:     "The Pulumi type for this resource.",
									Type:            stringType,
								},
								{
									Name:            "name",
									ProtobufMapping: codegenrpc.CustomPropertyMapping_URN_NAME,
									Description:     "The Pulumi name for this resource.",
									Type:            stringType,
								},
								{
									Name:          "properties",
									ProtobufField: "properties",
									Description:   "The current state (sufficiently complete to identify the resource).",
									Type:          propertyMapType,
								},
								{
									Name:          "inputs",
									ProtobufField: "inputs",
									Description:   "The current inputs, if any (only populated during refresh).",
									Type:          propertyMapType,
								},
							},
						},
					},
				},
				{
					Element: &codegenrpc.TypeDeclaration_Record{
						Record: &codegenrpc.Record{
							Name:            "pulumi.experimental.providers.read_response",
							ProtobufMessage: "pulumirpc.ReadResponse",
							Description:     "The response from reading a resource.",
							Properties: []*codegenrpc.Property{
								{
									Name:          "id",
									ProtobufField: "id",
									Description:   "The ID of the resource read back (or empty if missing).",
									Type:          stringType,
								},
								{
									Name:          "properties",
									ProtobufField: "properties",
									Description:   "The state of the resource read from the live environment.",
									Type:          propertyMapType,
								},
								{
									Name:          "inputs",
									ProtobufField: "inputs",
									Description:   "The inputs for this resource that would be returned from Check.",
									Type:          propertyMapType,
								},
							},
						},
					},
				},
				{
					Element: &codegenrpc.TypeDeclaration_Record{
						Record: &codegenrpc.Record{
							Name:            "pulumi.experimental.providers.update_request",
							ProtobufMessage: "pulumirpc.UpdateRequest",
							Description:     "A request to update a resource.",
							Properties: []*codegenrpc.Property{
								{
									Name:          "id",
									ProtobufField: "id",
									Description:   "The ID of the resource to update.",
									Type:          stringType,
								},
								{
									Name:          "URN",
									ProtobufField: "urn",
									Description:   "The Pulumi URN for this resource.",
									Type:          stringType,
								},
								{
									Name:            "type",
									ProtobufMapping: codegenrpc.CustomPropertyMapping_URN_TYPE,
									Description:     "The Pulumi type for this resource.",
									Type:            stringType,
								},
								{
									Name:            "name",
									ProtobufMapping: codegenrpc.CustomPropertyMapping_URN_NAME,
									Description:     "The Pulumi name for this resource.",
									Type:            stringType,
								},
								{
									Name:          "old",
									ProtobufField: "old",
									Description:   "The old values of provider inputs for the resource to update.",
									Type:          propertyMapType,
								},
								{
									Name:          "new",
									ProtobufField: "new",
									Description:   "The new values of provider inputs for the resource to update.",
									Type:          propertyMapType,
								},
								{
									Name:          "timeout",
									ProtobufField: "timeout",
									Description:   "The create request timeout.",
									Type:          durationType,
								},
								{
									Name:          "ignore_changes",
									ProtobufField: "ignoreChanges",
									Description:   "A set of property paths that should be treated as unchanged.",
									Type:          makeArrayType(stringType),
								},
								{
									Name:          "preview",
									ProtobufField: "preview",
									Description:   "true if this is a preview and the provider should not actually update the resource.",
									Type:          boolType,
								},
							},
						},
					},
				},
				{
					Element: &codegenrpc.TypeDeclaration_Record{
						Record: &codegenrpc.Record{
							Name:            "pulumi.experimental.providers.update_response",
							ProtobufMessage: "pulumirpc.UpdateResponse",
							Description:     "TODO",
							Properties: []*codegenrpc.Property{
								{
									Name:          "properties",
									ProtobufField: "properties",
									Description:   "Any properties that were computed during updating.",
									Type:          propertyMapType,
								},
							},
						},
					},
				},
				{
					Element: &codegenrpc.TypeDeclaration_Record{
						Record: &codegenrpc.Record{
							Name:            "pulumi.experimental.providers.delete_request",
							ProtobufMessage: "pulumirpc.DeleteRequest",
							Description:     "A request to delete a resource.",
							Properties: []*codegenrpc.Property{
								{
									Name:          "id",
									ProtobufField: "id",
									Description:   "The ID of the resource to delete.",
									Type:          stringType,
								},
								{
									Name:          "URN",
									ProtobufField: "urn",
									Description:   "The Pulumi URN for this resource.",
									Type:          stringType,
								},
								{
									Name:            "type",
									ProtobufMapping: codegenrpc.CustomPropertyMapping_URN_TYPE,
									Description:     "The Pulumi type for this resource.",
									Type:            stringType,
								},
								{
									Name:            "name",
									ProtobufMapping: codegenrpc.CustomPropertyMapping_URN_NAME,
									Description:     "The Pulumi name for this resource.",
									Type:            stringType,
								},
							},
						},
					},
				},
				{
					Element: &codegenrpc.TypeDeclaration_Interface{
						Interface: &codegenrpc.Interface{
							Name:        "pulumi.experimental.providers.provider",
							GrpcService: "pulumirpc.ResourceProvider",
							GrpcKind:    codegenrpc.GrpcKind_KIND_SERVER,
							Description: "Provider presents a simple interface for orchestrating resource create, read, update, and delete operations. Each" +
								" provider understands how to handle all of the resource types within a single package.\n" +
								"\n" +
								"It is important to note that provider operations are not transactional (Some providers might decide to offer" +
								" transactional semantics, but such a provider is a rare treat). As a result, failures in the operations below can" +
								" range from benign to catastrophic (possibly leaving behind a corrupt resource). It is up to the provider to make a" +
								" best effort to ensure catastrophes do not occur. The errors returned from mutating operations indicate both the" +
								" underlying error condition in addition to a bit indicating whether the operation was successfully rolled back.",
							Methods: []*codegenrpc.Method{
								{
									Name:         "check_config",
									GrpcMethod:   "CheckConfig",
									Description:  "Validates the configuration for this resource provider.",
									ResponseType: "pulumi.experimental.providers.check_response",
									Request: &codegenrpc.Request{
										Name: "request",
										Type: "pulumi.experimental.providers.check_request",
									},
								},
								{
									Name:         "diff_config",
									GrpcMethod:   "DiffConfig",
									Description:  "Checks what impacts a hypothetical change to this provider's configuration will have on the provider.",
									ResponseType: "pulumi.experimental.providers.diff_response",
									Request: &codegenrpc.Request{
										Name: "request",
										Type: "pulumi.experimental.providers.diff_request",
									},
								},
								{
									Name:         "configure",
									GrpcMethod:   "Configure",
									Description:  "Configures the resource provider with \"globals\" that control its behavior.",
									ResponseType: "pulumi.experimental.providers.configure_response",
									Request: &codegenrpc.Request{
										Name: "request",
										Type: "pulumi.experimental.providers.configure_request",
									},
								},
								{
									Name:       "check",
									GrpcMethod: "Check",
									Description: "Validates that the given property bag is valid for a resource of the given type and returns the inputs" +
										" that should be passed to successive calls to Diff, Create, or Update for this resource. As a rule, the provider" +
										" inputs returned by a call to Check should preserve the original representation of the properties as present in" +
										" the program inputs. Though this rule is not required for correctness, violations thereof can negatively impact" +
										" the end-user experience, as the provider inputs are using for detecting and rendering diffs.",
									ResponseType: "pulumi.experimental.providers.check_response",
									Request: &codegenrpc.Request{
										Name: "request",
										Type: "pulumi.experimental.providers.check_request",
									},
								},
								{
									Name:         "diff",
									GrpcMethod:   "Diff",
									Description:  "Checks what impacts a hypothetical update will have on the resource's properties.",
									ResponseType: "pulumi.experimental.providers.diff_response",
									Request: &codegenrpc.Request{
										Name: "request",
										Type: "pulumi.experimental.providers.diff_request",
									},
								},
								{
									Name:       "create",
									GrpcMethod: "Create",
									Description: "Allocates a new instance of the provided resource and returns its unique ID afterwards. (The input ID" +
										" must be blank.)  If this call fails, the resource must not have been created (i.e., it is \"transactional\").",
									ResponseType: "pulumi.experimental.providers.create_response",
									Request: &codegenrpc.Request{
										Name: "request",
										Type: "pulumi.experimental.providers.create_request",
									},
								},
								{
									Name:         "update",
									GrpcMethod:   "Update",
									Description:  "Updates an existing resource with new values.",
									ResponseType: "pulumi.experimental.providers.update_response",
									Request: &codegenrpc.Request{
										Name: "request",
										Type: "pulumi.experimental.providers.update_request",
									},
								},
								{
									Name:        "delete",
									GrpcMethod:  "Delete",
									Description: "Tears down an existing resource with the given ID. If it fails, the resource is assumed to still exist.",
									Request: &codegenrpc.Request{
										Name: "request",
										Type: "pulumi.experimental.providers.delete_request",
									},
								},
								{
									Name:       "read",
									GrpcMethod: "Read",
									Description: "Reads the current live state associated with a resource. Enough state must be include in the inputs to uniquely" +
										" identify the resource; this is typically just the resource ID, but may also include some properties.",
									ResponseType: "pulumi.experimental.providers.read_response",
									Request: &codegenrpc.Request{
										Name: "request",
										Type: "pulumi.experimental.providers.read_request",
									},
								},
							},
						},
					},
				},
				{
					Element: &codegenrpc.TypeDeclaration_Record{
						Record: &codegenrpc.Record{
							Name:            "pulumi.experimental.codegen.get_mapping_request",
							ProtobufMessage: "pulumirpc.codegen.GetMappingRequest",
							Description: "Allows the engine to return ecosystem specific information to allow " +
								"the converter to be able to convert provider types from a source markup to Pulumi.",
							Properties: []*codegenrpc.Property{
								{
									Name:          "provider",
									ProtobufField: "provider",
									Description:   "The provider name for the mapping being requested.",
									Type:          stringType,
								},
							},
						},
					},
				},
				{
					Element: &codegenrpc.TypeDeclaration_Record{
						Record: &codegenrpc.Record{
							Name:            "pulumi.experimental.codegen.get_mapping_response",
							ProtobufMessage: "pulumirpc.codegen.GetMappingResponse",
							Description: "Returns converter plugin specific data for the requested provider. This will normally be human " +
								"readable JSON, but the engine doesn't mandate any form.",
							Properties: []*codegenrpc.Property{
								{
									Name:          "data",
									ProtobufField: "data",
									Description:   "The conversion plugin specific data (if any).",
									Type:          makeArrayType(byteType),
								},
							},
						},
					},
				},
				{
					Element: &codegenrpc.TypeDeclaration_Interface{
						Interface: &codegenrpc.Interface{
							Name:        "pulumi.experimental.codegen.mapper",
							GrpcService: "pulumirpc.codegen.Mapper",
							GrpcKind:    codegenrpc.GrpcKind_KIND_CLIENT,
							Description: "Mapper is a service for getting mappings from other ecosystems to Pulumi. This is currently unstable and experimental.",
							Methods: []*codegenrpc.Method{
								{
									Name:         "get_mapping",
									GrpcMethod:   "GetMapping",
									Description:  "Tries to find a mapping for the given provider.",
									ResponseType: "pulumi.experimental.codegen.get_mapping_response",
									Request: &codegenrpc.Request{
										Name: "request",
										Type: "pulumi.experimental.providers.get_mapping_request",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
