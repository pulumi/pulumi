// Copyright 2023, Pulumi Corporation.  All rights reserved.

package client

import (
	"fmt"
	"strings"

	"github.com/pulumi/esc"
)

type EnvironmentDiagnostic struct {
	Range   *esc.Range `json:"range,omitempty"`
	Summary string     `json:"summary,omitempty"`
	Detail  string     `json:"detail,omitempty"`
}

type EnvironmentErrorResponse struct {
	Code        int                     `json:"code,omitempty"`
	Message     string                  `json:"message,omitempty"`
	Diagnostics []EnvironmentDiagnostic `json:"diagnostics,omitempty"`
}

func (err EnvironmentErrorResponse) Error() string {
	errString := fmt.Sprintf("[%d] %s", err.Code, err.Message)
	if len(err.Diagnostics) > 0 {
		errString += fmt.Sprintf("\nDiags: %s", diagsErrorString(err.Diagnostics))
	}
	return errString
}

type EnvironmentDiagnosticError struct {
	Diagnostics []EnvironmentDiagnostic `json:"diagnostics,omitempty"`
}

// Error implements the Error interface.
func (err EnvironmentDiagnosticError) Error() string {
	return diagsErrorString(err.Diagnostics)
}

func diagsErrorString(envDiags []EnvironmentDiagnostic) string {
	var diags strings.Builder
	for _, d := range envDiags {
		fmt.Fprintf(&diags, "%v\n", d.Summary)
	}
	return diags.String()
}

type OrgEnvironment struct {
	Organization string `json:"organization,omitempty"`
	Name         string `json:"name,omitempty"`
}

type ListEnvironmentsResponse struct {
	Environments []OrgEnvironment `json:"environments,omitempty"`
	NextToken    string           `json:"nextToken,omitempty"`
}

type UpdateEnvironmentResponse struct {
	EnvironmentDiagnosticError
}

type CheckEnvironmentResponse struct {
	Diagnostics []EnvironmentDiagnostic `json:"diagnostics,omitempty"`
}

type OpenEnvironmentResponse struct {
	ID          string                  `json:"id"`
	Diagnostics []EnvironmentDiagnostic `json:"diagnostics,omitempty"`
}
