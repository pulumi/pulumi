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

// OrgUsageSummaryParams collects the query parameters accepted by the
// GetUsageSummaryResourceHours endpoint. Zero values for optional fields are
// omitted from the request so the server can apply its own defaults.
type OrgUsageSummaryParams struct {
	// Granularity is the time aggregation: "hourly", "daily", or "monthly".
	Granularity string `url:"granularity,omitempty"`
	// LookbackDays is the number of days to look back from LookbackStart (or
	// the current time when LookbackStart is omitted).
	LookbackDays int64 `url:"lookbackDays,omitempty"`
	// LookbackStart is a Unix timestamp marking the end of the lookback
	// window. Defaults to the current time when omitted.
	LookbackStart int64 `url:"lookbackStart,omitempty"`
}

// OrgUsageSummaryResponse is the envelope returned by the
// GetUsageSummaryResourceHours endpoint.
type OrgUsageSummaryResponse struct {
	// Summary is the list of per-bucket resource count summaries.
	Summary []OrgResourceCountSummary `json:"summary"`
}

// OrgResourceCountSummary is a single point of summary for resources under
// management for an organization, mirroring the cloud-side
// ResourceCountSummary schema.
//
// Which of Month/Day/WeekNumber/Hour are populated depends on the requested
// granularity; they are pointers so JSON can distinguish "field present and
// zero" from "field omitted".
type OrgResourceCountSummary struct {
	// Year is the 4-digit year.
	Year int `json:"year"`
	// Month is the month of the year (1-12).
	Month *int `json:"month,omitempty"`
	// Day is the day of the month (1-31).
	Day *int `json:"day,omitempty"`
	// WeekNumber is the week number in the year (0-53), with Sunday marking
	// the start of the week.
	WeekNumber *int `json:"weekNumber,omitempty"`
	// Hour is the hour of the day (0-23).
	Hour *int `json:"hour,omitempty"`
	// Resources is the Resources Under Management count: the average of all
	// resources for the given time frame.
	Resources int64 `json:"resources"`
	// ResourceHours is the Resource Hours Under Management count: the sum of
	// all resources for the given time frame. 1 resource hour = 1 Pulumi
	// credit.
	ResourceHours int64 `json:"resourceHours"`
}
