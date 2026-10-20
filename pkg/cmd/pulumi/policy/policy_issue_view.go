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

package policy

import (
	"time"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
)

type policyIssueJSON struct {
	ID               string `json:"id"`
	EntityType       string `json:"entityType,omitempty"`
	EntityProject    string `json:"entityProject,omitempty"`
	EntityID         string `json:"entityId,omitempty"`
	PolicyPack       string `json:"policyPack"`
	PolicyPackTag    string `json:"policyPackTag,omitempty"`
	PolicyName       string `json:"policyName"`
	Level            string `json:"level"`
	Severity         string `json:"severity,omitempty"`
	ResourceURN      string `json:"resourceURN,omitempty"`
	ResourceProvider string `json:"resourceProvider,omitempty"`
	ResourceType     string `json:"resourceType,omitempty"`
	ResourceName     string `json:"resourceName,omitempty"`
	Message          string `json:"message,omitempty"`
	ObservedAt       string `json:"observedAt,omitempty"`
	LastModified     string `json:"lastModified,omitempty"`
	Status           string `json:"status,omitempty"`
	Kind             string `json:"kind,omitempty"`
	Priority         string `json:"priority,omitempty"`
}

func toPolicyIssueJSON(issue cloudapitype.PolicyIssue) policyIssueJSON {
	return policyIssueJSON{
		ID:               issue.ID,
		EntityType:       string(issue.EntityType),
		EntityProject:    issue.EntityProject,
		EntityID:         issue.EntityID,
		PolicyPack:       issue.PolicyPack,
		PolicyPackTag:    issue.PolicyPackTag,
		PolicyName:       issue.PolicyName,
		Level:            issue.Level,
		Severity:         string(issue.Severity),
		ResourceURN:      issue.ResourceURN,
		ResourceProvider: issue.ResourceProvider,
		ResourceType:     issue.ResourceType,
		ResourceName:     issue.ResourceName,
		Message:          issue.Message,
		ObservedAt:       formatPolicyIssueTime(issue.ObservedAt),
		LastModified:     formatPolicyIssueTime(issue.LastModified),
		Status:           string(issue.Status),
		Kind:             string(issue.Kind),
		Priority:         string(issue.Priority),
	}
}

// formatPolicyIssueTime reproduces the service's timestamp text: the service encodes these times with Go's
// time.Time JSON marshaller, which writes RFC3339Nano.
func formatPolicyIssueTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339Nano)
}
