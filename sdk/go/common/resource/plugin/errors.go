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

package plugin

import (
	"errors"
	"fmt"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// InitError represents a failure to initialize a resource, i.e., the resource has been successfully
// created, but it has failed to initialize.
type InitError struct {
	Reasons []string
}

var _ error = (*InitError)(nil)

func (ie *InitError) Error() string {
	var err error
	for _, reason := range ie.Reasons {
		err = multierror.Append(err, errors.New(reason))
	}
	if err == nil {
		return "resource init failed"
	}
	return err.Error()
}

// AlreadyExistsError represents a create failure caused by a conflicting existing resource.
type AlreadyExistsError struct {
	Cause        string
	ResourceType tokens.Type
	ResourceName string
	ResourceID   resource.ID
}

var _ error = (*AlreadyExistsError)(nil)

func (e *AlreadyExistsError) Error() string {
	message := e.baseMessage()
	if e.ResourceType == "" || e.ResourceName == "" {
		return message
	}

	importID := "<id>"
	if e.ResourceID != "" {
		importID = string(e.ResourceID)
	}

	return fmt.Sprintf(
		"%s\n\nTo resolve this conflict:\n"+
			"  - If this resource should be managed by Pulumi, run `pulumi import %s %s %s`.\n"+
			"  - If Pulumi should create a new resource instead, rename it in your program and rerun `pulumi up`.\n"+
			"  - If the existing resource is no longer needed, delete it in the cloud provider and rerun `pulumi up`.",
		message,
		e.ResourceType,
		e.ResourceName,
		importID,
	)
}

func (e *AlreadyExistsError) baseMessage() string {
	if e == nil || e.Cause == "" {
		return "resource already exists"
	}

	return e.Cause
}
