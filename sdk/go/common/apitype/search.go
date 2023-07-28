// Copyright 2016-2023, Pulumi Corporation.
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
	"fmt"
	"strings"
)

type ResourceSearchResponse struct {
	Total        *int64                    `json:"total,omitempty"`
	Resources    []ResourceResult          `json:"resources,omitempty"`
	Aggregations map[string]Aggregation    `json:"aggregations,omitempty"`
	Pagination   *ResourceSearchPagination `json:"pagination,omitempty"`
}

func (r *ResourceSearchResponse) Error() error {
	return fmt.Errorf("just a normal error")
}

// ResourceResult is the user-facing type for our indexed resources.
//
// If you add a property here, don't forget to update fieldMappings to make it
// queryable!
type ResourceResult struct {
	Created      *string                      `json:"created,omitempty" csv:"created"`
	Custom       *bool                        `json:"custom,omitempty" csv:"custom"`
	Delete       *bool                        `json:"delete,omitempty" csv:"delete"`
	Dependencies []string                     `json:"dependencies,omitempty" csv:"-"`
	ID           *string                      `json:"id,omitempty" csv:"id"`
	Matches      map[string][]json.RawMessage `json:"matches,omitempty" csv:"-"`
	Modified     *string                      `json:"modified,omitempty" csv:"modified"`
	Module       *string                      `json:"module" csv:"module"`
	Name         *string                      `json:"name,omitempty" csv:"name"`
	Package      *string                      `json:"package" csv:"package"`
	Parent       *string                      `json:"parent.urn,omitempty" csv:"parent_urn"`
	Pending      *string                      `json:"pending,omitempty" csv:"pending"`
	Program      *string                      `json:"project,omitempty" csv:"project"`
	Protected    *bool                        `json:"protected,omitempty" csv:"protected"`
	ProviderURN  *string                      `json:"provider.urn,omitempty" csv:"provider_urn"`
	Stack        *string                      `json:"stack,omitempty" csv:"stack"`
	Type         *string                      `json:"type,omitempty" csv:"type"`
	URN          *string                      `json:"urn,omitempty" csv:"urn"`
	Teams        teamNameSlice                `json:"teams,omitempty" csv:"teams"`
	Properties   *RawProperty                 `json:"properties,omitempty" csv:"properties"`
}

// Aggregation collects the top 5 aggregated values for the requested dimension.
type Aggregation struct {
	Others  *int64              `json:"others,omitempty"`
	Results []AggregationBucket `json:"results,omitempty"`
}

// AggregationBucket is an example value for the requested aggregation, with a
// count of how many resources share that value.
type AggregationBucket struct {
	Name  *string `json:"name,omitempty"`
	Count *int64  `json:"count,omitempty"`
}

// ResourceSearchPagination provides links for paging through results.
type ResourceSearchPagination struct {
	Previous *string `json:"previous,omitempty"`
	Next     *string `json:"next,omitempty"`
	Cursor   *string `json:"cursor,omitempty"`
}

type teamNameSlice []string

func (teams *teamNameSlice) MarshalCSV() (string, error) {
	if teams == nil || len(*teams) == 0 {
		return "", nil
	}

	json, err := json.Marshal(*teams)
	if err != nil {
		return "", err
	}

	return string(json), nil
}

// RawProperty wraps a json.RawProperty for JSON or CSV export.
type RawProperty struct {
	json.RawMessage
}

type PulumiQueryResponse struct {
	Query string `json:"query"`
}

type PulumiQueryRequest struct {
	Query string `url:"query"`
}

func ParseQueryParams(rawParams *[]string) *PulumiQueryRequest {
	queryString := ""
	for _, param := range *rawParams {
		paramElements := strings.Split(param, "=")
		if len(paramElements) != 2 {
			queryString = fmt.Sprintf("%s%s ", queryString, param)
		} else {
			queryString = fmt.Sprintf("%s%s ", queryString, fmt.Sprintf("%s:%s", paramElements[0], paramElements[1]))
		}
	}
	return &PulumiQueryRequest{Query: queryString}
}
