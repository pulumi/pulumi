// Copyright 2023, Pulumi Corporation.
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

package client

import (
	"fmt"
	"strings"
	"time"

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

type RotateEnvironmentResponse struct {
	Code                int                     `json:"code,omitempty"`
	Message             string                  `json:"message,omitempty"`
	Diagnostics         []EnvironmentDiagnostic `json:"diagnostics,omitempty"`
	SecretRotationEvent SecretRotationEvent     `json:"secretRotationEvent,omitempty"`
}

type SecretRotationEvent struct {
	ID                   string              `json:"id"`
	EnvironmentID        string              `json:"environmentId"`
	CreatedAt            time.Time           `json:"created"`
	PreRotationRevision  int                 `json:"preRotationRevision"`
	PostRotationRevision *int                `json:"postRotationRevision,omitempty"`
	UserID               string              `json:"userID"`
	CompletedAt          *time.Time          `json:"completed,omitempty"`
	Status               RotationEventStatus `json:"status"`
	ScheduledActionID    *string             `json:"scheduledActionID,omitempty"`
	ErrorMessage         *string             `json:"errorMessage,omitempty"`
	Rotations            []SecretRotation    `json:"rotations"`
}

type SecretRotation struct {
	ID              string         `json:"id"`
	EnvironmentPath string         `json:"environmentPath"`
	Status          RotationStatus `json:"status"`
	ErrorMessage    *string        `json:"errorMessage,omitempty"`
}

type RotationEventStatus string

const (
	RotationEventSucceeded  RotationEventStatus = "succeeded"
	RotationEventFailed     RotationEventStatus = "failed"
	RotationEventInProgress RotationEventStatus = "in_progress"
)

type RotationStatus string

const (
	RotationSucceeded RotationStatus = "succeeded"
	RotationFailed    RotationStatus = "failed"
)

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

type CloneEnvironmentRequest struct {
	Project                 string `json:"project,omitempty"`
	Name                    string `json:"name"`
	Version                 int    `json:"version,omitempty"`
	PreserveHistory         bool   `json:"preserveHistory,omitempty"`
	PreserveAccess          bool   `json:"preserveAccess,omitempty"`
	PreserveEnvironmentTags bool   `json:"preserveEnvironmentTags,omitempty"`
	PreserveRevisionTags    bool   `json:"preserveRevisionTags,omitempty"`
}

type EnvironmentRevisionRetracted struct {
	Replacement int       `json:"replacement"`
	At          time.Time `json:"at"`
	ByLogin     string    `json:"byLogin,omitempty"`
	ByName      string    `json:"byName,omitempty"`
	Reason      string    `json:"reason,omitempty"`
}

type EnvironmentRevision struct {
	Number       int       `json:"number"`
	Created      time.Time `json:"created"`
	CreatorLogin string    `json:"creatorLogin"`
	CreatorName  string    `json:"creatorName"`
	Tags         []string  `json:"tags"`

	Retracted *EnvironmentRevisionRetracted `json:"retracted,omitempty"`
}

type CreateEnvironmentRevisionTagRequest struct {
	Name     string `json:"name"`
	Revision *int   `json:"revision,omitempty"`
}

type UpdateEnvironmentRevisionTagRequest struct {
	Revision *int `json:"revision,omitempty"`
}

type CreateEnvironmentDraftResponse struct {
	ChangeRequestID      string `json:"changeRequestId"`
	LatestRevisionNumber int    `json:"latestRevisionNumber"`
}

type SubmitChangeRequestRequest struct {
	Description *string `json:"description,omitempty"`
}

type EnvironmentTag struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Value       string    `json:"value"`
	Created     time.Time `json:"created"`
	Modified    time.Time `json:"modified"`
	EditorLogin string    `json:"editorLogin"`
	EditorName  string    `json:"editorName"`
}

type ListEnvironmentTagsResponse struct {
	Tags      map[string]*EnvironmentTag `json:"tags"`
	NextToken string                     `json:"nextToken"`
}

type TagRequest struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type CreateEnvironmentTagRequest = TagRequest

type UpdateEnvironmentTagRequest struct {
	CurrentTag TagRequest `json:"currentTag"`
	NewTag     TagRequest `json:"newTag"`
}

type EnvironmentRevisionTag struct {
	Name        string    `json:"name"`
	Revision    int       `json:"revision"`
	Created     time.Time `json:"created"`
	Modified    time.Time `json:"modified"`
	EditorLogin string    `json:"editorLogin"`
	EditorName  string    `json:"editorName"`
}

type ListEnvironmentRevisionTagsResponse struct {
	Tags      []EnvironmentRevisionTag `json:"tags"`
	NextToken string                   `json:"nextToken"`
}
type OrgEnvironment struct {
	Organization string `json:"organization,omitempty"`
	Project      string `json:"project,omitempty"`
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

type RetractEnvironmentRevisionRequest struct {
	Replacement *int   `json:"replacement,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

// GetDefaultOrganizationResponse returns the backend's opinion of which organization
// to default to for a given user, if a default organization has not been configured.
type GetDefaultOrganizationResponse struct {
	// Returns the organization name.
	// Can be an empty string, if the user is a member of no organizations
	Organization string `json:"gitHubLogin"`
}
