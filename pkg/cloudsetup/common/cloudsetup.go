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

// Package cloudsetup contains shared types and utilities for cloud setup operations
package cloudsetup

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type SetupRequest struct {
	Provider string `json:"provider"`
}

// CloudSetupResult is the result of a cloud setup operation.
type CloudSetupResult struct {
	// Success indicates whether the setup operation succeeded.
	Success bool `json:"success" yaml:"success"`
	// Resources created or managed during setup.
	Resources []CloudSetupResource `json:"resources" yaml:"resources"`
	// Message is an optional message about the setup operation.
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
}

// CloudSetupResource is a cloud resource that was created or managed during setup.
type CloudSetupResource struct {
	// Type of the resource.
	Type string `json:"type" yaml:"type"`
	// ID is the unique identifier of the resource.
	ID string `json:"id" yaml:"id"`
	// Name of the resource.
	Name string `json:"name" yaml:"name"`
	// Status of the resource operation.
	Status string `json:"status" yaml:"status"`
	// Error message if the resource operation failed.
	Error string `json:"error,omitempty" yaml:"error,omitempty"`
	// Properties holds additional properties of the resource.
	Properties map[string]string `json:"properties,omitempty" yaml:"properties,omitempty"`
}

// CloudAccount describes a cloud account, subscription, or project.
type CloudAccount struct {
	// ID of the account, subscription, or project.
	ID string `json:"id" yaml:"id"`
	// Name of the account, subscription, or project.
	Name string `json:"name" yaml:"name"`
	// Roles assumable within the account.
	Roles []string `json:"roles,omitempty" yaml:"roles,omitempty"`
	// Number is the project number (GCP only).
	Number int64 `json:"number,omitzero" yaml:"number,omitempty"`
}

// AzureEnvironmentInfo is the Azure environment configuration for a single subscription.
type AzureEnvironmentInfo struct {
	// SubscriptionID is the Azure subscription ID.
	SubscriptionID string `json:"subscriptionID" yaml:"subscriptionID"`
	// RoleID is the Azure role ID.
	RoleID string `json:"roleID" yaml:"roleID"`
	// ProjectName is the ESC project name.
	ProjectName string `json:"projectName" yaml:"projectName"`
	// EnvironmentName is the ESC environment name.
	EnvironmentName string `json:"environmentName" yaml:"environmentName"`
}

const (
	ResourceStatusCreated  = "created"
	ResourceStatusExisting = "existing"
	ResourceStatusFailed   = "failed"
)

// SetupError represents an error that occurred during cloud setup
type SetupError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Cause   error  `json:"-"`
}

func (e *SetupError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *SetupError) Unwrap() error {
	return e.Cause
}

const (
	//nolint:gosec // G101: constant name contains "CREDENTIALS", not a hardcoded credential value
	ErrorCodeInvalidCredentials = "INVALID_CREDENTIALS"
	ErrorCodeSetupFailed        = "SETUP_FAILED"
)

func NewSetupError(code, message string, cause error) *SetupError {
	return &SetupError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

func WrapSetupError(result *CloudSetupResult, resourceType string, err error) (*CloudSetupResult, error) {
	result.Resources = append(result.Resources, CloudSetupResource{
		Type:   resourceType,
		Status: ResourceStatusFailed,
		Error:  err.Error(),
	})
	message := "failed to create: " + resourceType

	return result, NewSetupError(ErrorCodeSetupFailed, message, err)
}

func RunWithRetries(ctx context.Context, maxAttempts int, sleepDuration time.Duration, operation func() error) error {
	var lastErr error
	for i := range maxAttempts {
		err := operation()
		if err == nil {
			return nil
		}
		lastErr = err

		if i < maxAttempts-1 {
			select {
			case <-ctx.Done():
				return errors.New("context cancelled while retrying operation")
			case <-time.After(sleepDuration):
			}
		}
	}

	return fmt.Errorf("operation failed after %d attempts, last error: %w", maxAttempts, lastErr)
}
