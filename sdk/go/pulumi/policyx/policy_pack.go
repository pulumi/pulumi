// Copyright 2025, Pulumi Corporation.
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

package policyx

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumix"
)

// HandshakeRequest is sent by the engine to the analyzer to establish a connection and provide
// necessary information for the analyzer to start up.
type HandshakeRequest struct {
	// The "host" of the analyzer. This is the engine that is running the analyzer.
	Host pulumix.Engine
	// A *root directory* where the analyzer's binary, `PulumiPolicy.yaml`, or other identifying source code is located.
	// In the event that the analyzer is *not* being booted by the engine (e.g. in the case that the engine has been
	// asked to attach to an existing running analyzer instance via a host/port number), this field will be empty.
	RootDirectory *string
	// A *program directory* in which the analyzer should execute. This is generally a subdirectory of the root
	// directory, though this is not required. In the event that the analyzer is *not* being booted by the engine (e.g.
	// in the case that the engine has been asked to attach to an existing running analyzer instance via a host/port
	// number), this field will be empty.
	ProgramDirectory *string
}

// HandshakeResponse is sent by the analyzer to the engine in response to a HandshakeRequest. Currently, it is empty,
type HandshakeResponse struct{}

// PolicyConfig provides configuration for a policy.
type PolicyConfig struct {
	// Enforcement level of the policy.
	EnforcementLevel EnforcementLevel
	// Configuration properties of the policy.
	Properties map[string]any
}

// ConfigureRequest provides configuration information to the analyzer.
type ConfigureRequest struct {
	// Map of policy name to config.
	PolicyConfig map[string]PolicyConfig
}

// PolicyPack defines a collection of policies that can be used to enforce rules and guidelines.
type PolicyPack interface {
	// Name returns the name of the policy pack.
	Name() string
	// Version returns the version of the policy pack.
	Version() semver.Version
	// Handshake is called by the engine to establish a connection with the analyzer.
	Handshake(context.Context, HandshakeRequest) (HandshakeResponse, error)
	// Policies returns the list of policies in the policy pack.
	Policies() []Policy
}

type policyPack struct {
	name      string
	version   semver.Version
	handshake func(context.Context, HandshakeRequest) (HandshakeResponse, error)
	policies  []Policy
}

var policyPackNameRE = regexp.MustCompile(`^[a-zA-Z0-9-_.]{1,100}$`)

// NewPolicyPack creates a new PolicyPack with the given name, version, enforcement level, handshake function, and
// policies.
func NewPolicyPack(
	name string, version semver.Version, enforcementLevel EnforcementLevel,
	handshake func(context.Context, HandshakeRequest) (HandshakeResponse, error),
	policies []Policy,
) (PolicyPack, error) {
	if name == "" || !policyPackNameRE.MatchString(name) {
		return nil, fmt.Errorf("invalid policy pack name: %q", name)
	}

	for _, policy := range policies {
		if policy.Name() == "all" {
			return nil, fmt.Errorf("invalid policy name %[1]q. %[1]q is a reserved name", policy.Name())
		}

		schema := policy.ConfigSchema()

		if schema != nil {
			if _, ok := schema.Properties["enforcementLevel"]; ok {
				return nil, errors.New("enforcementLevel cannot be explicitly specified in configSchema properties." +
					" enforcementLevel is a reserved property")
			}
			for _, req := range schema.Required {
				if req == "enforcementLevel" {
					return nil, errors.New("enforcementLevel cannot be required in configSchema." +
						" enforcementLevel is a reserved property")
				}
			}
		}
	}

	return &policyPack{
		name:      name,
		version:   version,
		policies:  policies,
		handshake: handshake,
	}, nil
}

func (p *policyPack) Name() string { return p.name }

func (p *policyPack) Version() semver.Version { return p.version }

func (p *policyPack) Handshake(
	ctx context.Context,
	req HandshakeRequest,
) (HandshakeResponse, error) {
	if p.handshake != nil {
		return p.handshake(ctx, req)
	}
	return HandshakeResponse{}, nil
}

func (p *policyPack) Policies() []Policy {
	return p.policies
}
