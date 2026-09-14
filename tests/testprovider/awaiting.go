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
//go:build !all
// +build !all

package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	rpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"google.golang.org/protobuf/types/known/emptypb"
)

func init() {
	providerSchema.Resources["testprovider:index:Awaiting"] = pschema.ResourceSpec{
		ObjectTypeSpec: pschema.ObjectTypeSpec{
			Type:     "object",
			Required: []string{"release", "outputOnly"},
			Properties: map[string]pschema.PropertySpec{
				"release":    {TypeSpec: pschema.TypeSpec{Ref: "pulumi.json#/Any"}},
				"outputOnly": {TypeSpec: pschema.TypeSpec{Type: "string"}},
			},
		},
		InputProperties: map[string]pschema.PropertySpec{
			"ready":   {TypeSpec: pschema.TypeSpec{Type: "boolean"}},
			"version": {TypeSpec: pschema.TypeSpec{Type: "string"}},
			"release": {TypeSpec: pschema.TypeSpec{Ref: "pulumi.json#/Any"}},
		},
		RequiredInputs: []string{"ready", "version", "release"},
	}
	providerSchema.Resources["testprovider:index:RequiresKnown"] = pschema.ResourceSpec{
		ObjectTypeSpec: pschema.ObjectTypeSpec{
			Type:     "object",
			Required: []string{"source", "outputOnly"},
			Properties: map[string]pschema.PropertySpec{
				"source":     {TypeSpec: pschema.TypeSpec{Type: "string"}},
				"outputOnly": {TypeSpec: pschema.TypeSpec{Type: "string"}},
				"generation": {TypeSpec: pschema.TypeSpec{Type: "number"}},
			},
		},
		InputProperties: map[string]pschema.PropertySpec{
			"source":     {TypeSpec: pschema.TypeSpec{Type: "string"}},
			"outputOnly": {TypeSpec: pschema.TypeSpec{Type: "string"}},
		},
		RequiredInputs: []string{"source", "outputOnly"},
	}
	testProviders["testprovider:index:Awaiting"] = &awaitingProvider{}
	testProviders["testprovider:index:RequiresKnown"] = &requiresKnownProvider{}
}

type awaitingProvider struct{}

func (*awaitingProvider) Check(_ context.Context, req *rpc.CheckRequest) (*rpc.CheckResponse, error) {
	return &rpc.CheckResponse{Inputs: req.GetNews()}, nil
}

func (*awaitingProvider) Diff(_ context.Context, req *rpc.DiffRequest) (*rpc.DiffResponse, error) {
	oldInputs, err := plugin.UnmarshalProperties(req.GetOldInputs(), plugin.MarshalOptions{KeepUnknowns: true})
	if err != nil {
		return nil, err
	}
	olds, err := plugin.UnmarshalProperties(req.GetOlds(), plugin.MarshalOptions{KeepUnknowns: true})
	if err != nil {
		return nil, err
	}
	news, err := plugin.UnmarshalProperties(req.GetNews(), plugin.MarshalOptions{KeepUnknowns: true})
	if err != nil {
		return nil, err
	}
	if oldInputs["version"].DeepEquals(news["version"]) {
		if strings.HasSuffix(req.GetUrn(), "::production") && controllerGeneration() == 2 {
			return &rpc.DiffResponse{Changes: rpc.DiffResponse_DIFF_SOME}, nil
		}
		if generation := controllerGeneration(); generation > 0 &&
			olds["release"].ObjectValue()["generation"].NumberValue() != float64(generation) {
			return &rpc.DiffResponse{Changes: rpc.DiffResponse_DIFF_SOME}, nil
		}
		return &rpc.DiffResponse{Changes: rpc.DiffResponse_DIFF_NONE}, nil
	}
	return &rpc.DiffResponse{Changes: rpc.DiffResponse_DIFF_SOME, Replaces: []string{"version"}}, nil
}

func (*awaitingProvider) Create(_ context.Context, req *rpc.CreateRequest) (*rpc.CreateResponse, error) {
	inputs, err := plugin.UnmarshalProperties(req.GetProperties(), plugin.MarshalOptions{KeepUnknowns: true})
	if err != nil {
		return nil, err
	}
	traceAwaiting("create " + inputs["version"].StringValue())
	if state := os.Getenv("PULUMI_TEST_CONTROLLER_READY"); state != "" {
		if _, err := os.Stat(state); os.IsNotExist(err) {
			outputs, marshalErr := plugin.MarshalProperties(resource.PropertyMap{
				"release": resource.NewProperty(resource.PropertyMap{
					"id":         resource.NewProperty(""),
					"generation": resource.NewProperty(0.0),
					"parts":      resource.NewProperty(resource.PropertyMap{}),
					"artifacts":  resource.NewProperty(resource.PropertyMap{}),
				}),
				"outputOnly": resource.NewProperty(""),
			}, plugin.MarshalOptions{KeepUnknowns: true})
			if marshalErr != nil {
				return nil, marshalErr
			}
			return &rpc.CreateResponse{Id: inputs["version"].StringValue(), Properties: outputs}, nil
		}
	}
	if !inputs["ready"].BoolValue() {
		return &rpc.CreateResponse{Awaiting: true, AwaitingReason: "fixture not ready"}, nil
	}
	release := inputs["release"].ObjectValue().Copy()
	if generation := controllerGeneration(); generation > 0 {
		release["generation"] = resource.NewProperty(float64(generation))
	}
	outputs, err := plugin.MarshalProperties(resource.PropertyMap{
		"release":    resource.NewProperty(release),
		"outputOnly": resource.NewProperty("resolved-" + inputs["version"].StringValue()),
	}, plugin.MarshalOptions{KeepUnknowns: true})
	if err != nil {
		return nil, err
	}
	return &rpc.CreateResponse{Id: inputs["version"].StringValue(), Properties: outputs}, nil
}

func (p *awaitingProvider) Update(ctx context.Context, req *rpc.UpdateRequest) (*rpc.UpdateResponse, error) {
	created, err := p.Create(ctx, &rpc.CreateRequest{Urn: req.GetUrn(), Properties: req.GetNews()})
	if err != nil {
		return nil, err
	}
	return &rpc.UpdateResponse{Properties: created.Properties, Awaiting: created.Awaiting, AwaitingReason: created.AwaitingReason}, nil
}

func (*awaitingProvider) Read(_ context.Context, req *rpc.ReadRequest) (*rpc.ReadResponse, error) {
	return &rpc.ReadResponse{Id: req.GetId(), Properties: req.GetProperties()}, nil
}

func (*awaitingProvider) Delete(_ context.Context, req *rpc.DeleteRequest) (*emptypb.Empty, error) {
	traceAwaiting("delete " + req.GetId())
	return &emptypb.Empty{}, nil
}

func (*awaitingProvider) Invoke(context.Context, *rpc.InvokeRequest) (*rpc.InvokeResponse, error) {
	return nil, fmt.Errorf("unsupported")
}
func (*awaitingProvider) Call(context.Context, *rpc.CallRequest) (*rpc.CallResponse, error) {
	return nil, fmt.Errorf("unsupported")
}

type requiresKnownProvider struct{ awaitingProvider }

func (*requiresKnownProvider) Check(_ context.Context, req *rpc.CheckRequest) (*rpc.CheckResponse, error) {
	inputs, err := plugin.UnmarshalProperties(req.GetNews(), plugin.MarshalOptions{KeepUnknowns: true})
	if err != nil {
		return nil, err
	}
	traceAwaiting("dependent-check")
	if inputs["source"].ContainsUnknowns() {
		return nil, fmt.Errorf("dependent Check received an unknown")
	}
	return &rpc.CheckResponse{Inputs: req.GetNews()}, nil
}

func (*requiresKnownProvider) Diff(_ context.Context, req *rpc.DiffRequest) (*rpc.DiffResponse, error) {
	olds, err := plugin.UnmarshalProperties(req.GetOlds(), plugin.MarshalOptions{KeepUnknowns: true})
	if err != nil {
		return nil, err
	}
	if generation := controllerGeneration(); generation > 0 &&
		olds["generation"].NumberValue() != float64(generation) {
		return &rpc.DiffResponse{Changes: rpc.DiffResponse_DIFF_SOME}, nil
	}
	return &rpc.DiffResponse{Changes: rpc.DiffResponse_DIFF_NONE}, nil
}

func (*requiresKnownProvider) Create(_ context.Context, req *rpc.CreateRequest) (*rpc.CreateResponse, error) {
	inputs, err := plugin.UnmarshalProperties(req.GetProperties(), plugin.MarshalOptions{KeepUnknowns: true})
	if err != nil {
		return nil, err
	}
	inputs["generation"] = resource.NewProperty(float64(controllerGeneration()))
	properties, err := plugin.MarshalProperties(inputs, plugin.MarshalOptions{KeepUnknowns: true})
	return &rpc.CreateResponse{Id: "dependent", Properties: properties}, err
}

func (*requiresKnownProvider) Update(_ context.Context, req *rpc.UpdateRequest) (*rpc.UpdateResponse, error) {
	traceAwaiting("dependent-update")
	created, err := (&requiresKnownProvider{}).Create(context.Background(), &rpc.CreateRequest{Properties: req.GetNews()})
	if err != nil {
		return nil, err
	}
	if os.Getenv("PULUMI_TEST_SELECTOR_MIGRATION") != "" && controllerGeneration() == 2 {
		return &rpc.UpdateResponse{Properties: created.Properties, Awaiting: true,
			AwaitingReason: "waiting for pipeline shape activation"}, nil
	}
	return &rpc.UpdateResponse{Properties: created.Properties}, nil
}

func controllerGeneration() int {
	contents, err := os.ReadFile(os.Getenv("PULUMI_TEST_CONTROLLER_READY"))
	if err != nil {
		return 0
	}
	generation, _ := strconv.Atoi(strings.TrimSpace(string(contents)))
	return generation
}

func (*requiresKnownProvider) Delete(_ context.Context, _ *rpc.DeleteRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func traceAwaiting(line string) {
	if path := os.Getenv("PULUMI_TEST_AWAITING_TRACE"); path != "" {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err == nil {
			defer f.Close()
			_, _ = fmt.Fprintln(f, line)
		}
	}
}
