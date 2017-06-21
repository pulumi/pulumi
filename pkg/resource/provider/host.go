// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/pulumi/lumi/pkg/diag"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/resource/plugin"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
)

// HostClient is a client interface into the host's engine RPC interface.
type HostClient struct {
	conn   *grpc.ClientConn
	client lumirpc.EngineClient
}

// NewHostClient dials the target address, connects over gRPC, and returns a client interface.
func NewHostClient(addr string) (*HostClient, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	return &HostClient{
		conn:   conn,
		client: lumirpc.NewEngineClient(conn),
	}, nil
}

// Close closes and renders the connection and client unusable.
func (host *HostClient) Close() error {
	return host.conn.Close()
}

// Log logs a global message, including errors and warnings.
func (host *HostClient) Log(sev diag.Severity, msg string) error {
	var rpcsev lumirpc.LogSeverity
	switch sev {
	case diag.Debug:
		rpcsev = lumirpc.LogSeverity_DEBUG
	case diag.Info:
		rpcsev = lumirpc.LogSeverity_INFO
	case diag.Warning:
		rpcsev = lumirpc.LogSeverity_WARNING
	case diag.Error:
		rpcsev = lumirpc.LogSeverity_ERROR
	default:
		contract.Failf("Unrecognized log severity type: %v", sev)
	}
	_, err := host.client.Log(context.TODO(), &lumirpc.LogRequest{
		Severity: rpcsev,
		Message:  msg,
	})
	return err
}

// ReadLocation reads the value from a static or module property.
func (host *HostClient) ReadLocation(tok tokens.Token) (resource.PropertyValue, error) {
	req := &lumirpc.ReadLocationRequest{Token: string(tok)}
	resp, err := host.client.ReadLocation(context.TODO(), req)
	if err != nil {
		return resource.PropertyValue{}, nil
	}
	return plugin.UnmarshalPropertyValue(nil, resp, plugin.MarshalOptions{}), nil
}

// ReadBoolLocation reads the value from a static or module property, and returns it as a bool.
func (host *HostClient) ReadBoolLocation(tok tokens.Token) (bool, error) {
	v, err := host.ReadLocation(tok)
	if err != nil {
		return false, err
	} else if !v.IsBool() {
		return false, errors.Errorf("Expected %v to be a bool; got %v instead", tok, v)
	}
	return v.BoolValue(), nil
}

// ReadNumberLocation reads the value from a static or module property, and returns it as a string.
func (host *HostClient) ReadNumberLocation(tok tokens.Token) (float64, error) {
	v, err := host.ReadLocation(tok)
	if err != nil {
		return float64(0), err
	} else if !v.IsNumber() {
		return float64(0), errors.Errorf("Expected %v to be a number; got %v instead", tok, v)
	}
	return v.NumberValue(), nil
}

// ReadStringLocation reads the value from a static or module property, and returns it as a string.
func (host *HostClient) ReadStringLocation(tok tokens.Token) (string, error) {
	v, err := host.ReadLocation(tok)
	if err != nil {
		return "", err
	} else if !v.IsString() {
		return "", errors.Errorf("Expected %v to be a string; got %v instead", tok, v)
	}
	return v.StringValue(), nil
}

// ReadArrayLocation reads the value from a static or module property, and returns it as an array.
func (host *HostClient) ReadArrayLocation(tok tokens.Token) ([]resource.PropertyValue, error) {
	v, err := host.ReadLocation(tok)
	if err != nil {
		return nil, err
	} else if !v.IsArray() {
		return nil, errors.Errorf("Expected %v to be an array; got %v instead", tok, v)
	}
	return v.ArrayValue(), nil
}

// ReadObjectLocation reads the value from a static or module property, and returns it as an object.
func (host *HostClient) ReadObjectLocation(tok tokens.Token) (resource.PropertyMap, error) {
	v, err := host.ReadLocation(tok)
	if err != nil {
		return nil, err
	} else if !v.IsObject() {
		return nil, errors.Errorf("Expected %v to be an object; got %v instead", tok, v)
	}
	return v.ObjectValue(), nil
}
