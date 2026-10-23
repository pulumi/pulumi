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
	"encoding/json"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseIssueFilters(t *testing.T, args ...string) policyIssueFilters {
	t.Helper()
	var f policyIssueFilters
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	addPolicyIssueFilterFlags(flags, &f)
	require.NoError(t, flags.Parse(args))
	return f
}

func TestBuildPolicyIssueFilter_Empty(t *testing.T) {
	t.Parallel()
	f := parseIssueFilters(t)
	assert.True(t, f.empty())
	filter, sort, err := buildPolicyIssueGrid(f)
	require.NoError(t, err)
	assert.Nil(t, filter)
	assert.Nil(t, sort)
}

func TestBuildPolicyIssueFilter_SingleValue(t *testing.T) {
	t.Parallel()
	f := parseIssueFilters(t, "--severity", "high")
	assert.False(t, f.empty())
	filter, _, err := buildPolicyIssueGrid(f)
	require.NoError(t, err)
	got, err := json.Marshal(filter)
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"AND","conditions":[
		{"colId":"severity","filterType":"text","type":"equals","filter":"high"}]}`, string(got))
}

func TestBuildPolicyIssueFilter_RepeatIsOr_FlagsAreAnd(t *testing.T) {
	t.Parallel()
	f := parseIssueFilters(t, "--pack", "aws", "--severity", "high", "--severity", "critical")
	filter, _, err := buildPolicyIssueGrid(f)
	require.NoError(t, err)
	got, err := json.Marshal(filter)
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"AND","conditions":[
		{"colId":"","filterType":"join","filter":"","type":"OR","conditions":[
			{"colId":"severity","filterType":"text","type":"equals","filter":"high"},
			{"colId":"severity","filterType":"text","type":"equals","filter":"critical"}]},
		{"colId":"policyPack","filterType":"text","type":"equals","filter":"aws"}]}`, string(got))
}

func TestBuildPolicyIssueFilter_Stack(t *testing.T) {
	t.Parallel()
	f := parseIssueFilters(t, "--stack", "web/prod")
	filter, _, err := buildPolicyIssueGrid(f)
	require.NoError(t, err)
	got, err := json.Marshal(filter)
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"AND","conditions":[{"colId":"","filterType":"join","filter":"","type":"AND","conditions":[
		{"colId":"entityType","filterType":"text","type":"equals","filter":"stack"},
		{"colId":"entityProject","filterType":"text","type":"equals","filter":"web"},
		{"colId":"entityId","filterType":"text","type":"equals","filter":"prod"}]}]}`, string(got))

	_, _, err = buildPolicyIssueGrid(parseIssueFilters(t, "--stack", "prod"))
	assert.ErrorContains(t, err, "expected <project>/<stack>")
}

func TestBuildPolicyIssueFilter_StackRepeatIsOr(t *testing.T) {
	t.Parallel()
	f := parseIssueFilters(t, "--stack", "web/prod", "--stack", "web/dev")
	filter, _, err := buildPolicyIssueGrid(f)
	require.NoError(t, err)
	require.Len(t, filter.Conditions, 1)
	assert.Equal(t, "OR", filter.Conditions[0].Type)
	require.Len(t, filter.Conditions[0].Conditions, 2)
	assert.Equal(t, "AND", filter.Conditions[0].Conditions[0].Type)
}

func TestBuildPolicyIssueFilter_Account(t *testing.T) {
	t.Parallel()
	f := parseIssueFilters(t, "--account", "my-acct")
	filter, _, err := buildPolicyIssueGrid(f)
	require.NoError(t, err)
	got, err := json.Marshal(filter)
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"AND","conditions":[{"colId":"","filterType":"join","filter":"","type":"AND","conditions":[
		{"colId":"entityType","filterType":"text","type":"equals","filter":"insights-account"},
		{"colId":"entityProject","filterType":"text","type":"equals","filter":"my-acct"},
		{"colId":"entityId","filterType":"text","type":"equals","filter":"my-acct"}]}]}`, string(got))

	filter, _, err = buildPolicyIssueGrid(parseIssueFilters(t, "--account", "team/aws-prod"))
	require.NoError(t, err)
	got, err = json.Marshal(filter)
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"AND","conditions":[{"colId":"","filterType":"join","filter":"","type":"AND","conditions":[
		{"colId":"entityType","filterType":"text","type":"equals","filter":"insights-account"},
		{"colId":"entityProject","filterType":"text","type":"equals","filter":"team"},
		{"colId":"entityId","filterType":"text","type":"equals","filter":"aws-prod"}]}]}`, string(got))

	for _, bad := range []string{"/aws-prod", "team/", "a/b/c"} {
		_, _, err = buildPolicyIssueGrid(parseIssueFilters(t, "--account", bad))
		assert.ErrorContains(t, err, "expected [<project>/]<account>", bad)
	}
}

func TestBuildPolicyIssueFilter_AssignedTo(t *testing.T) {
	t.Parallel()
	filter, _, err := buildPolicyIssueGrid(parseIssueFilters(t, "--assigned-to", "bob"))
	require.NoError(t, err)
	assert.Equal(t, "bob", filter.Conditions[0].Filter)

	filter, _, err = buildPolicyIssueGrid(parseIssueFilters(t, "--assigned-to", "bob", "--assigned-to", "alice"))
	require.NoError(t, err)
	got, err := json.Marshal(filter)
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"AND","conditions":[{"colId":"","filterType":"join","filter":"","type":"OR","conditions":[
		{"colId":"assignedTo","filterType":"text","type":"equals","filter":"bob"},
		{"colId":"assignedTo","filterType":"text","type":"equals","filter":"alice"}]}]}`, string(got))
}

func TestBuildPolicyIssueFilter_InvalidEnum(t *testing.T) {
	t.Parallel()
	_, _, err := buildPolicyIssueGrid(parseIssueFilters(t, "--severity", "urgent"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--severity")
	assert.Contains(t, err.Error(), "low, medium, high, critical")

	_, _, err = buildPolicyIssueGrid(parseIssueFilters(t, "--status", "nope"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open, in_progress, by_design, fixed, ignored")

	_, _, err = buildPolicyIssueGrid(parseIssueFilters(t, "--priority", "p9"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "p0, p1, p2, p3, p4")
}

func TestBuildPolicyIssueSort(t *testing.T) {
	t.Parallel()
	f := parseIssueFilters(t, "--sort", "severity:desc", "--sort", "observedAt")
	assert.False(t, f.empty())
	filter, sort, err := buildPolicyIssueGrid(f)
	require.NoError(t, err)
	assert.Nil(t, filter)
	got, err := json.Marshal(sort)
	require.NoError(t, err)
	assert.JSONEq(t, `[{"colId":"severity","sort":"desc"},{"colId":"observedAt","sort":"asc"}]`, string(got))

	_, _, err = buildPolicyIssueGrid(parseIssueFilters(t, "--sort", "message"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "severity, priority, status, observedAt")

	_, _, err = buildPolicyIssueGrid(parseIssueFilters(t, "--sort", "severity:down"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "asc, desc")
}

func TestValidatePolicyIssueFilters_RejectsEmptyValues(t *testing.T) {
	t.Parallel()
	for _, flag := range []string{
		"severity", "priority", "status", "pack", "policy", "group", "stack", "account",
		"resource-type", "urn", "assigned-to", "sort",
	} {
		t.Run(flag, func(t *testing.T) {
			t.Parallel()
			f := parseIssueFilters(t, "--"+flag, "")
			err := validatePolicyIssueFilters(f)
			assert.ErrorContains(t, err, `invalid value "" for --`+flag)
			_, _, err = buildPolicyIssueGrid(f)
			assert.ErrorContains(t, err, `invalid value "" for --`+flag)
		})
	}
}

func TestValidatePolicyIssueFilters(t *testing.T) {
	t.Parallel()
	require.NoError(t, validatePolicyIssueFilters(parseIssueFilters(t, "--assigned-to", "bob")))
	assert.ErrorContains(t, validatePolicyIssueFilters(parseIssueFilters(t, "--severity", "urgent")), "critical")
	assert.ErrorContains(t, validatePolicyIssueFilters(parseIssueFilters(t, "--stack", "prod")),
		"expected <project>/<stack>")
	assert.ErrorContains(t, validatePolicyIssueFilters(parseIssueFilters(t, "--sort", "message")), "--sort")
}
