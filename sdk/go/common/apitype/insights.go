// Copyright 2016-2025, Pulumi Corporation.
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

import "time"

// Insights Account Types

// InsightsAccount represents a discovery account configuration.
type InsightsAccount struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	OrgName        string                 `json:"orgName,omitempty"`
	Provider       string                 `json:"provider"` // "aws", "azure-native", "gcp", etc.
	Environment    string                 `json:"providerEnvRef"`
	ScanSchedule   string                 `json:"scanSchedule,omitempty"` // "daily", "none"
	ProviderConfig map[string]interface{} `json:"providerConfig,omitempty"`
	ScanStatus     *ScanStatus            `json:"scanStatus,omitempty"`
	CreatedAt      time.Time              `json:"createdAt,omitempty"`
	UpdatedAt      time.Time              `json:"updatedAt,omitempty"`
	ChildAccounts  []string               `json:"childAccounts,omitempty"`
}

// CreateInsightsAccountRequest is the request body for creating an Insights account.
type CreateInsightsAccountRequest struct {
	Provider       string                 `json:"provider"`
	Environment    string                 `json:"environment"`
	ScanSchedule   string                 `json:"scanSchedule,omitempty"`
	ProviderConfig map[string]interface{} `json:"providerConfig,omitempty"`
}

// UpdateInsightsAccountRequest is the request body for updating an Insights account.
type UpdateInsightsAccountRequest struct {
	Environment    *string                `json:"environment,omitempty"`
	ScanSchedule   *string                `json:"scanSchedule,omitempty"`
	ProviderConfig map[string]interface{} `json:"providerConfig,omitempty"`
}

// ListInsightsAccountsResponse is the response for listing Insights accounts.
type ListInsightsAccountsResponse struct {
	Accounts          []InsightsAccount `json:"accounts"`
	ContinuationToken string            `json:"continuationToken,omitempty"`
}

// Scan Types

// ScanStatusType represents the status of a scan.
type ScanStatusType string

const (
	// ScanStatusPending indicates the scan is pending execution.
	ScanStatusPending ScanStatusType = "Pending"
	// ScanStatusRunning indicates the scan is currently running.
	ScanStatusRunning ScanStatusType = "Running"
	// ScanStatusSucceeded indicates the scan completed successfully.
	ScanStatusSucceeded ScanStatusType = "Succeeded"
	// ScanStatusFailed indicates the scan failed.
	ScanStatusFailed ScanStatusType = "Failed"
	// ScanStatusCancelled indicates the scan was cancelled.
	ScanStatusCancelled ScanStatusType = "Cancelled"
)

// ScanStatus represents the status of a discovery scan.
type ScanStatus struct {
	ID               string         `json:"id,omitempty"`
	Status           ScanStatusType `json:"status,omitempty"`
	StartedAt        *time.Time     `json:"startedAt,omitempty"`
	FinishedAt       *time.Time     `json:"finishedAt,omitempty"`
	LastUpdatedAt    *time.Time     `json:"lastUpdatedAt,omitempty"`
	ResourceCount    int            `json:"resourceCount,omitempty"`
	NextScheduledScan *time.Time    `json:"nextScheduledScan,omitempty"`
}

// Scan represents a discovery scan execution.
type Scan struct {
	ID            string                 `json:"id"`
	AccountID     string                 `json:"accountId,omitempty"`
	AccountName   string                 `json:"accountName,omitempty"`
	Status        ScanStatusType         `json:"status"`
	StartedAt     *time.Time             `json:"startedAt,omitempty"`
	CompletedAt   *time.Time             `json:"completedAt,omitempty"`
	ResourceCount int                    `json:"resourceCount,omitempty"`
	ErrorCount    int                    `json:"errorCount,omitempty"`
	Options       map[string]interface{} `json:"options,omitempty"`
}

// CreateScanRequest is the request body for creating a scan.
type CreateScanRequest struct {
	ListConcurrency *int    `json:"listConcurrency,omitempty"`
	ReadConcurrency *int    `json:"readConcurrency,omitempty"`
	BatchSize       *int    `json:"batchSize,omitempty"`
	ReadTimeout     *string `json:"readTimeout,omitempty"`
	AgentPoolID     *string `json:"agentPoolId,omitempty"`
}

// ListScansResponse is the response for listing scans.
type ListScansResponse struct {
	Scans             []Scan `json:"scans"`
	ContinuationToken string `json:"continuationToken,omitempty"`
}
