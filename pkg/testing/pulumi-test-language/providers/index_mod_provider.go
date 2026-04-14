// Copyright 2016, Pulumi Corporation.
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

package providers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/blang/semver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// IndexModProvider is a provider for validating modules that start with "index".
type IndexModProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*IndexModProvider)(nil)

func (p *IndexModProvider) Close() error {
	return nil
}

func (p *IndexModProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *IndexModProvider) Pkg() tokens.Package {
	return "index-mod"
}

func (p *IndexModProvider) version() string {
	return "35.0.0"
}

func (p *IndexModProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	ver := semver.MustParse(p.version())
	return plugin.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *IndexModProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	resourceProperties := map[string]schema.PropertySpec{
		"text": {
			TypeSpec: schema.TypeSpec{
				Type: "string",
			},
		},
	}
	resourceRequired := []string{"text"}

	pkg := schema.PackageSpec{
		Name:    "index-mod",
		Version: p.version(),
		Resources: map[string]schema.ResourceSpec{
			"index-mod:indexMine:Resource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: resourceProperties,
					Required:   resourceRequired,
				},
				InputProperties: resourceProperties,
				RequiredInputs:  resourceRequired,
				Methods: map[string]string{
					"call": "index-mod:indexMine:Resource/call",
				},
			},
			"index-mod:indexMine/nested:Resource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: resourceProperties,
					Required:   resourceRequired,
				},
				InputProperties: resourceProperties,
				RequiredInputs:  resourceRequired,
				Methods: map[string]string{
					"call": "index-mod:indexMine/nested:Resource/call",
				},
			},
		},
		Functions: map[string]schema.FunctionSpec{
			"index-mod:indexMine:concatWorld": {
				Inputs: &schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"value": {
							TypeSpec: schema.TypeSpec{
								Type: "string",
							},
						},
					},
					Required: []string{"value"},
				},
				ReturnType: &schema.ReturnTypeSpec{
					ObjectTypeSpec: &schema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]schema.PropertySpec{
							"result": {
								TypeSpec: schema.TypeSpec{
									Type: "string",
								},
							},
						},
						Required: []string{"result"},
					},
				},
			},
			"index-mod:indexMine/nested:concatWorld": {
				Inputs: &schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"value": {
							TypeSpec: schema.TypeSpec{
								Type: "string",
							},
						},
					},
					Required: []string{"value"},
				},
				ReturnType: &schema.ReturnTypeSpec{
					ObjectTypeSpec: &schema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]schema.PropertySpec{
							"result": {
								TypeSpec: schema.TypeSpec{
									Type: "string",
								},
							},
						},
						Required: []string{"result"},
					},
				},
			},
			"index-mod:indexMine:Resource/call": {
				Inputs: &schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"__self__": {
							TypeSpec: schema.TypeSpec{
								Type: "ref",
								Ref:  "#/resources/index-mod:indexMine:Resource",
							},
						},
						"input": {
							TypeSpec: schema.TypeSpec{
								Type: "string",
							},
						},
					},
					Required: []string{"__self__", "input"},
				},
				ReturnType: &schema.ReturnTypeSpec{
					ObjectTypeSpec: &schema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]schema.PropertySpec{
							"output": {
								TypeSpec: schema.TypeSpec{
									Type: "number",
								},
							},
						},
						Required: []string{"output"},
					},
				},
			},
			"index-mod:indexMine/nested:Resource/call": {
				Inputs: &schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"__self__": {
							TypeSpec: schema.TypeSpec{
								Type: "ref",
								Ref:  "#/resources/index-mod:indexMine/nested:Resource",
							},
						},
						"input": {
							TypeSpec: schema.TypeSpec{
								Type: "string",
							},
						},
					},
					Required: []string{"__self__", "input"},
				},
				ReturnType: &schema.ReturnTypeSpec{
					ObjectTypeSpec: &schema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]schema.PropertySpec{
							"output": {
								TypeSpec: schema.TypeSpec{
									Type: "number",
								},
							},
						},
						Required: []string{"output"},
					},
				},
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *IndexModProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	version, ok := req.News["version"]
	if !ok {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "missing version"),
		}, nil
	}
	if !version.IsString() {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not a string"),
		}, nil
	}
	if version.StringValue() != p.version() {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not "+p.version()),
		}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *IndexModProvider) Invoke(
	_ context.Context, req plugin.InvokeRequest,
) (plugin.InvokeResponse, error) {
	switch req.Tok {
	case "index-mod:indexMine:concatWorld", "index-mod:indexMine/nested:concatWorld":
		value, ok := req.Args["value"]
		if !ok {
			return plugin.InvokeResponse{
				Failures: makeCheckFailure("value", "missing value"),
			}, nil
		}

		if value.IsComputed() {
			return plugin.InvokeResponse{
				Failures: makeCheckFailure("value", "value is unknown when calling function"),
			}, nil
		}

		if !value.IsString() {
			return plugin.InvokeResponse{
				Failures: makeCheckFailure("value", "is not a string"),
			}, nil
		}

		return plugin.InvokeResponse{
			Properties: resource.PropertyMap{
				"result": resource.NewProperty(value.StringValue() + " world"),
			},
		}, nil
	}
	return plugin.InvokeResponse{}, fmt.Errorf("unknown function %v", req.Tok)
}

func (p *IndexModProvider) Call(
	ctx context.Context,
	req plugin.CallRequest,
) (plugin.CallResponse, error) {
	conn, err := grpc.NewClient(
		req.Info.MonitorAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		return plugin.CallResponse{}, fmt.Errorf("connect to resource monitor: %w", err)
	}
	defer conn.Close()

	monitor := pulumirpc.NewResourceMonitorClient(conn)
	switch req.Tok {
	case "index-mod:indexMine:Resource/call", "index-mod:indexMine/nested:Resource/call":
		value, ok := req.Args["input"]
		if !ok {
			return plugin.CallResponse{
				Failures: makeCheckFailure("input", "missing input"),
			}, nil
		}

		if value.IsComputed() {
			return plugin.CallResponse{
				Failures: makeCheckFailure("input", "input is unknown when calling function"),
			}, nil
		}

		if !value.IsString() {
			return plugin.CallResponse{
				Failures: makeCheckFailure("input", "is not a string"),
			}, nil
		}

		selfRef := req.Args["__self__"].ResourceReferenceValue()

		selfRes, err := monitor.Invoke(ctx, &pulumirpc.ResourceInvokeRequest{
			Tok: "pulumi:pulumi:getResource",
			Args: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"urn": structpb.NewStringValue(string(selfRef.URN)),
				},
			},
			AcceptResources: true,
		})
		if err != nil {
			return plugin.CallResponse{}, fmt.Errorf("hydrating __self__ resource reference: %w", err)
		}

		text := selfRes.Return.Fields["state"].GetStructValue().Fields["text"]

		return plugin.CallResponse{
			Return: resource.PropertyMap{
				"output": resource.NewProperty(float64(len(value.StringValue()) + len(text.GetStringValue()))),
			},
		}, nil
	}

	return plugin.CallResponse{}, fmt.Errorf("unknown function %v", req.Tok)
}

func (p *IndexModProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	switch req.URN.Type().String() {
	case "index-mod:indexMine:Resource", "index-mod:indexMine/nested:Resource":
	default:
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("expected exactly one property: %v", req.News)),
		}, nil
	}

	text, ok := req.News["text"]
	if !ok {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("text", "missing required property 'text'"),
		}, nil
	}

	if !text.IsString() && !text.IsComputed() {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("text", "text is not a string"),
		}, nil
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *IndexModProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	switch req.URN.Type().String() {
	case "index-mod:indexMine:Resource", "index-mod:indexMine/nested:Resource":
	default:
		return plugin.CreateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

	id := "id"
	if req.Preview {
		id = ""
	}

	return plugin.CreateResponse{
		ID: resource.ID(id),
		Properties: resource.PropertyMap{
			"text": req.Properties["text"],
		},
		Status: resource.StatusOK,
	}, nil
}
