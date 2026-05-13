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

// InsightsResourceSearchParams collects the query parameters accepted by the
// resource search v2 endpoint (`GetOrgResourceSearchV2Query`). Zero values for
// optional fields are omitted from the request so the server can apply its
// own defaults.
type InsightsResourceSearchParams struct {
	// Query is the Pulumi-query-syntax filter string. Empty means "match all".
	Query string `url:"query,omitempty"`
	// Sort is the list of fields to sort by, in priority order. Empty means
	// sort by relevance (or modified time when Query is empty).
	Sort []string `url:"sort,omitempty"`
	// Ascending flips the sort direction. Defaults to descending on the
	// service side, so we only send the param when explicitly set.
	Ascending bool `url:"asc,omitempty"`
	// Page is the 1-based page number to return. The API supports paging up
	// to 10,000 results total; use Cursor beyond that.
	Page int `url:"page,omitempty"`
	// Size is the number of results per page.
	Size int `url:"size,omitempty"`
	// Cursor is an opaque bookmark for pagination beyond 10,000 results
	// (Enterprise plans only).
	Cursor string `url:"cursor,omitempty"`
	// Properties asks the server to include resource input/output values in
	// each result. Requires a supported subscription — the service returns
	// 402 Payment Required otherwise.
	Properties bool `url:"properties,omitempty"`
	// Collapse consolidates resources discovered through multiple sources
	// (e.g. an IaC stack and an Insights scan) into a single result.
	Collapse bool `url:"collapse,omitempty"`
}

// InsightsResourceSearchResponse is the envelope returned by the resource
// search v2 endpoint. Mirrors the OpenAPI `ResourceSearchResult` schema.
type InsightsResourceSearchResponse struct {
	// Total is the total number of matching resources across all pages.
	Total int64 `json:"total,omitempty"`
	// Resources holds the page of results. May be nil/empty when no matches.
	Resources []InsightsResourceSearchResult `json:"resources,omitempty"`
	// Aggregations is the per-facet bucket counts requested via facet/groupBy
	// (not exposed by the CLI today but passed through for forward compat).
	Aggregations map[string]InsightsResourceSearchAggregation `json:"aggregations,omitempty"`
	// Pagination carries cursors/links for advancing through pages.
	Pagination *InsightsResourceSearchPagination `json:"pagination,omitempty"`
}

// InsightsResourceSearchResult is one row in a resource search response. The
// v2 endpoint uses snake_case for the URN fields (`parent_urn`, `provider_urn`),
// distinguishing it from the v1 schema which uses dotted names.
type InsightsResourceSearchResult struct {
	Account      string          `json:"account,omitempty"`
	Category     string          `json:"category,omitempty"`
	Created      string          `json:"created,omitempty"`
	Custom       *bool           `json:"custom,omitempty"`
	Delete       *bool           `json:"delete,omitempty"`
	Dependencies []string        `json:"dependencies,omitempty"`
	Dependents   []string        `json:"dependents,omitempty"`
	External     *bool           `json:"external,omitempty"`
	Fingerprint  string          `json:"fingerprint,omitempty"`
	ID           string          `json:"id,omitempty"`
	Managed      string          `json:"managed,omitempty"`
	Matches      json.RawMessage `json:"matches,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
	Modified     string          `json:"modified,omitempty"`
	Module       string          `json:"module,omitempty"`
	Name         string          `json:"name,omitempty"`
	Package      string          `json:"package,omitempty"`
	ParentURN    string          `json:"parent_urn,omitempty"`
	Pending      string          `json:"pending,omitempty"`
	Project      string          `json:"project,omitempty"`
	Properties   json.RawMessage `json:"properties,omitempty"`
	Protected    *bool           `json:"protected,omitempty"`
	ProviderURN  string          `json:"provider_urn,omitempty"`
	SourceCount  int64           `json:"sourceCount,omitempty"`
	Stack        string          `json:"stack,omitempty"`
	Teams        []string        `json:"teams,omitempty"`
	Type         string          `json:"type,omitempty"`
	URN          string          `json:"urn,omitempty"`
}

// InsightsResourceSearchPagination carries pagination metadata. `Next` is the
// link to the next page (empty on the last page); the cursor is embedded in
// its query string. `Cursor` is a bookmark of the *current* page — do not use
// it to advance.
type InsightsResourceSearchPagination struct {
	Previous string `json:"previous,omitempty"`
	Next     string `json:"next,omitempty"`
	Cursor   string `json:"cursor,omitempty"`
}

// InsightsResourceSearchAggregation is a single facet's aggregated bucket
// list.
type InsightsResourceSearchAggregation struct {
	Others  int64                                     `json:"others,omitempty"`
	Results []InsightsResourceSearchAggregationBucket `json:"results,omitempty"`
}

// InsightsResourceSearchAggregationBucket is a single aggregation bucket: an
// example value and the number of resources that share it.
type InsightsResourceSearchAggregationBucket struct {
	Name  string `json:"name,omitempty"`
	Count int64  `json:"count,omitempty"`
}
