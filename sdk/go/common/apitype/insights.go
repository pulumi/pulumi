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

import (
	"encoding/json"
	"time"
)

// InsightsResourceWithVersion is a single discovered resource as returned by the
// Pulumi Insights ReadResource endpoint. The shape mirrors the OpenAPI schema of
// the same name in the Pulumi Cloud REST API.
type InsightsResourceWithVersion struct {
	// Account is the name of the Insights account the resource was discovered in.
	Account string `json:"account"`
	// Type is the Pulumi resource type token (e.g. `aws:s3/bucket:Bucket`).
	Type string `json:"type"`
	// ID is the cloud-provider-assigned identifier for the resource.
	ID string `json:"id"`
	// Version is the monotonically-increasing version number for this resource.
	Version int64 `json:"version"`
	// Modified is the time at which the resource was last modified on the cloud
	// provider side, as recorded by Insights.
	Modified time.Time `json:"modified"`
	// State is the raw resource state as captured by the scan. The shape is
	// resource-type-specific and is passed through as JSON.
	State json.RawMessage `json:"state,omitempty"`
	// PolicyState is the evaluation state for any policies that ran against the
	// resource. Empty when no policies were evaluated.
	PolicyState string `json:"policyState,omitempty"`
}

// InsightsScanRequest configures a scan started via the Pulumi Insights
// ScanAccount endpoint. Every field is optional; zero values are omitted from
// the JSON payload so the server can fall back to its own defaults.
type InsightsScanRequest struct {
	// AgentPoolID is the ID of the agent pool to use for scanning. Empty
	// selects the default agent pool.
	AgentPoolID string `json:"agentPoolID,omitempty"`
	// ListConcurrency caps the parallelism of list operations during the scan.
	ListConcurrency int64 `json:"listConcurrency,omitempty"`
	// ReadConcurrency caps the parallelism of read operations during the scan.
	ReadConcurrency int64 `json:"readConcurrency,omitempty"`
	// BatchSize is the number of resources processed in a single batch.
	BatchSize int64 `json:"batchSize,omitempty"`
	// ReadTimeout is the per-read timeout as a Go duration string (e.g. "30s",
	// "5m"). Empty leaves the server-side default in place.
	ReadTimeout string `json:"readTimeout,omitempty"`
}

// InsightsScanResponse is the workflow run returned by the Pulumi Insights
// ScanAccount endpoint. The shape mirrors the OpenAPI `WorkflowRun` schema.
type InsightsScanResponse struct {
	// ID is the unique identifier of the workflow run. It is also the scan ID
	// used by follow-up endpoints (e.g. ReadScanStatus, GetScanLogs).
	ID string `json:"id"`
	// OrgID is the organization that owns the scan.
	OrgID string `json:"orgId"`
	// UserID is the user that initiated the scan.
	UserID string `json:"userId"`
	// Status is the workflow status: "running", "failed", or "succeeded". A
	// freshly initiated scan reports "running".
	Status string `json:"status"`
	// StartedAt is when the workflow run began.
	StartedAt time.Time `json:"startedAt"`
	// FinishedAt is when the workflow run completed. Zero when still running.
	FinishedAt time.Time `json:"finishedAt"`
	// LastUpdatedAt is the most recent state-change timestamp.
	LastUpdatedAt time.Time `json:"lastUpdatedAt"`
	// JobTimeout is the deadline for jobs in the workflow run. Modeled as a
	// timestamp on the wire (not a duration), matching the OpenAPI schema.
	JobTimeout time.Time `json:"jobTimeout"`
	// Jobs is the list of job runs within the workflow. Empty for a scan that
	// has not been scheduled yet.
	Jobs []InsightsScanJobRun `json:"jobs,omitempty"`
}

// InsightsScanJobRun is one job within an Insights scan workflow.
type InsightsScanJobRun struct {
	// Status is the job status: one of "not-started", "accepted", "running",
	// "failed", "succeeded", "skipped".
	Status string `json:"status"`
	// Started is when the job began running. Zero before then.
	Started time.Time `json:"started,omitempty"`
	// LastUpdated is the most recent state-change timestamp for the job.
	LastUpdated time.Time `json:"lastUpdated,omitempty"`
	// Timeout is the per-job timeout in nanoseconds (Go time.Duration).
	Timeout int64 `json:"timeout"`
	// Steps is the ordered list of steps within the job.
	Steps []InsightsScanStepRun `json:"steps"`
	// Worker is opaque metadata about the worker executing the job. The shape
	// is server-defined and forwarded verbatim.
	Worker json.RawMessage `json:"worker,omitempty"`
}

// InsightsScanStepRun is one step within an Insights scan job.
type InsightsScanStepRun struct {
	// Name is the step name.
	Name string `json:"name"`
	// Status is the step status: one of "not-started", "running", "failed",
	// "succeeded".
	Status string `json:"status"`
	// Started is when the step began running. Zero before then.
	Started time.Time `json:"started,omitempty"`
	// LastUpdated is the most recent state-change timestamp for the step.
	LastUpdated time.Time `json:"lastUpdated,omitempty"`
}
