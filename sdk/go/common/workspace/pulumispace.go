// Copyright 2016-2026, Pulumi Corporation.
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
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Pulumispace represents a named collection of stacks that should be deployed together.
type Pulumispace struct {
	Name        string             `json:"name" yaml:"name"`
	Description string             `json:"description,omitempty" yaml:"description,omitempty"`
	Stacks      []PulumispaceStack `json:"stacks" yaml:"stacks"`
}

// PulumispaceStack represents a single stack entry in a Pulumispace file.
type PulumispaceStack struct {
	Path  string `json:"path" yaml:"path"`
	Stack string `json:"stack,omitempty" yaml:"stack,omitempty"`
}

// LoadPulumispace loads a Pulumispace file from the given path.
func LoadPulumispace(path string) (*Pulumispace, error) {
	contract.Requiref(path != "", "path", "must not be empty")

	m, err := marshallerForPath(path)
	if err != nil {
		return nil, fmt.Errorf("can not read '%s': %w", path, err)
	}

	b, err := readFileStripUTF8BOM(path)
	if err != nil {
		return nil, fmt.Errorf("could not read '%s': %w", path, err)
	}

	return loadPulumispaceFromBytes(b, path, m)
}

// loadPulumispaceFromBytes loads a Pulumispace from raw bytes using the provided marshaler.
func loadPulumispaceFromBytes(b []byte, path string, m encoding.Marshaler) (*Pulumispace, error) {
	var ps Pulumispace
	if err := m.Unmarshal(b, &ps); err != nil {
		return nil, fmt.Errorf("could not unmarshal '%s': %w", path, err)
	}

	if err := ps.Validate(); err != nil {
		return nil, fmt.Errorf("could not validate '%s': %w", path, err)
	}

	return &ps, nil
}

// Resolve resolves variable references (like ${STACK}) in a Pulumispace.
// The stackName parameter provides the value for ${STACK} substitution.
// If stackName is empty and any stack entry uses ${STACK}, an error is returned.
// Resolve returns a new Pulumispace and does not mutate the receiver.
func (ps *Pulumispace) Resolve(stackName string) (*Pulumispace, error) {
	resolved := &Pulumispace{
		Name:        ps.Name,
		Description: ps.Description,
		Stacks:      make([]PulumispaceStack, len(ps.Stacks)),
	}

	for i, s := range ps.Stacks {
		stack := s.Stack
		if strings.Contains(stack, "${STACK}") {
			if stackName == "" {
				return nil, fmt.Errorf(
					"stack entry %q uses ${STACK} but no stack name was provided", s.Path,
				)
			}
			stack = strings.ReplaceAll(stack, "${STACK}", stackName)
		}

		resolved.Stacks[i] = PulumispaceStack{
			Path:  s.Path,
			Stack: stack,
		}
	}

	return resolved, nil
}

// Validate checks that the Pulumispace is well-formed.
func (ps *Pulumispace) Validate() error {
	var errs []error

	if ps.Name == "" {
		errs = append(errs, errors.New("pulumispace is missing a 'name' attribute"))
	}

	if len(ps.Stacks) == 0 {
		errs = append(errs, errors.New("pulumispace must have at least one stack entry"))
	}

	seen := make(map[string]bool)
	for i, s := range ps.Stacks {
		if s.Path == "" {
			errs = append(errs, fmt.Errorf("stack entry %d is missing a 'path' attribute", i))
			continue
		}

		if filepath.IsAbs(s.Path) {
			errs = append(errs, fmt.Errorf("stack entry %d has an absolute path %q; paths must be relative", i, s.Path))
		}

		cleaned := filepath.Clean(s.Path)
		if seen[cleaned] {
			errs = append(errs, fmt.Errorf("duplicate stack path %q", s.Path))
		}
		seen[cleaned] = true
	}

	return errors.Join(errs...)
}
