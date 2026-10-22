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

package insights

import (
	"encoding/json"
	"reflect"
	"time"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
)

type insightsAccountJSON struct {
	ID                   string                  `json:"id"`
	Name                 string                  `json:"name"`
	Provider             string                  `json:"provider"`
	ProviderVersion      string                  `json:"providerVersion,omitempty"`
	ProviderEnvRef       string                  `json:"providerEnvRef,omitempty"`
	ScheduledScanEnabled bool                    `json:"scheduledScanEnabled"`
	AgentPoolID          string                  `json:"agentPoolID,omitempty"`
	ProviderConfig       json.RawMessage         `json:"providerConfig,omitempty"`
	OwnedBy              insightsAccountOwner    `json:"ownedBy"`
	ScanStatus           *insightsAccountScanRun `json:"scanStatus,omitempty"`
}

type insightsAccountOwner struct {
	Name        string `json:"name"`
	GitHubLogin string `json:"githubLogin"`
	AvatarURL   string `json:"avatarUrl"`
}

type insightsAccountScanRun struct {
	AccountName   string     `json:"accountName,omitempty"`
	ID            string     `json:"id"`
	OrgID         string     `json:"orgId"`
	ResourceCount int        `json:"resourceCount,omitempty"`
	UserID        string     `json:"userId"`
	Status        string     `json:"status"`
	StartedAt     time.Time  `json:"startedAt"`
	FinishedAt    *time.Time `json:"finishedAt"`
	LastUpdatedAt time.Time  `json:"lastUpdatedAt"`
	JobTimeout    time.Time  `json:"jobTimeout"`
}

func toInsightsAccountJSON(account cloudapitype.InsightsAccount) (insightsAccountJSON, error) {
	view := insightsAccountJSON{
		ID:                   account.ID,
		Name:                 account.Name,
		Provider:             account.Provider,
		ProviderVersion:      account.ProviderVersion, //nolint:staticcheck // kept so existing output is unchanged
		ProviderEnvRef:       account.Environment,
		ScheduledScanEnabled: account.ScheduledScanEnabled,
		AgentPoolID:          account.AgentPoolID,
		OwnedBy: insightsAccountOwner{
			Name:        account.OwnedBy.Name,
			GitHubLogin: account.OwnedBy.GitHubLogin,
			AvatarURL:   account.OwnedBy.AvatarURL,
		},
	}
	switch config := account.ProviderConfig.(type) {
	case nil:
	case json.RawMessage:
		view.ProviderConfig = config
	default:
		raw, err := json.Marshal(config)
		if err != nil {
			return insightsAccountJSON{}, err
		}
		view.ProviderConfig = raw
	}
	if !reflect.ValueOf(account.ScanStatus).IsZero() {
		scan := account.ScanStatus
		view.ScanStatus = &insightsAccountScanRun{
			AccountName:   scan.AccountName,
			ID:            scan.ID,
			OrgID:         scan.OrgID,
			ResourceCount: scan.ResourceCount,
			UserID:        scan.UserID,
			Status:        string(scan.Status),
			StartedAt:     scan.StartedAt,
			FinishedAt:    scan.FinishedAt,
			LastUpdatedAt: scan.LastUpdatedAt,
			JobTimeout:    scan.JobTimeout,
		}
	}
	return view, nil
}
