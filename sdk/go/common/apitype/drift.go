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

package apitype

// ListDriftRunsResponse is the paginated response for listing drift runs.
type ListDriftRunsResponse struct {
	DriftRuns    []DriftRun `json:"driftRuns"`
	ItemsPerPage int        `json:"itemsPerPage"`
	Total        int        `json:"total"`
}

// DriftRun describes a single drift detection run.
type DriftRun struct {
	ID                string          `json:"id"`
	DriftDetected     bool            `json:"driftDetected"`
	Created           string          `json:"created"`
	Status            string          `json:"status"`
	DeploymentID      string          `json:"deploymentId,omitempty"`
	DeploymentVersion int             `json:"deploymentVersion,omitzero"`
	DetectUpdate      *DriftRunUpdate `json:"detectUpdate,omitempty"`
	RemediateUpdate   *DriftRunUpdate `json:"remediateUpdate,omitempty"`
}

// DriftRunUpdate describes a detect or remediate phase of a drift run.
type DriftRunUpdate struct {
	UpdateID        string         `json:"updateId"`
	ResourceChanges map[string]int `json:"resourceChanges,omitempty"`
	Modified        string         `json:"modified"`
	Status          string         `json:"status"`
}
