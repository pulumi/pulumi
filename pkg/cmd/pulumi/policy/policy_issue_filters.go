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
	"fmt"
	"slices"
	"strings"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/spf13/pflag"
)

type policyIssueFilters struct {
	severities, priorities, statuses []string
	packs, policies, groups          []string
	stacks, accounts                 []string
	resourceTypes, urns              []string
	assignedTo                       []string
	sort                             []string // "<field>[:asc|desc]"
}

const (
	gridFilterTypeText = "text"
	// The service only rewrites assignedTo logins inside nested groups marked as joins.
	gridFilterTypeJoin = "join"
	gridOpEquals       = "equals"
	gridCombinatorAnd  = "AND"
	gridCombinatorOr   = "OR"
	gridSortAsc        = "asc"
	gridSortDesc       = "desc"

	colEntityType    = "entityType"
	colEntityProject = "entityProject"
	colEntityID      = "entityId"
)

var policyIssueSortFields = []string{
	"severity", "priority", "status", "observedAt", "lastModified",
	"policyPack", "policyName", "policyGroupName", "resourceType",
}

var policyIssueSortDirections = []string{gridSortAsc, gridSortDesc}

type policyIssueFilterColumn struct {
	flag    string
	colID   string
	allowed []string
	usage   string
	values  func(*policyIssueFilters) *[]string
}

func enumStrings[T ~string](all []T) []string {
	out := make([]string, 0, len(all))
	for _, v := range all {
		if v != "" {
			out = append(out, string(v))
		}
	}
	return out
}

// policyIssueFilterColumns lists the simple one-flag-one-column filters in the order they are ANDed.
// The stack and account filters span several columns and are handled separately, between group and
// resourceType.
func policyIssueFilterColumns() []policyIssueFilterColumn {
	return []policyIssueFilterColumn{
		{
			flag: "severity", colID: "severity",
			allowed: enumStrings(cloudapitype.AppPolicySeverity("").AllValues()),
			usage:   "Filter by severity (repeatable)",
			values:  func(f *policyIssueFilters) *[]string { return &f.severities },
		},
		{
			flag: "priority", colID: "priority",
			allowed: enumStrings(cloudapitype.PolicyIssuePriority("").AllValues()),
			usage:   "Filter by priority (repeatable)",
			values:  func(f *policyIssueFilters) *[]string { return &f.priorities },
		},
		{
			flag: "status", colID: "status",
			allowed: enumStrings(cloudapitype.PolicyIssueStatus("").AllValues()),
			usage:   "Filter by status (repeatable)",
			values:  func(f *policyIssueFilters) *[]string { return &f.statuses },
		},
		{
			flag: "pack", colID: "policyPack",
			usage:  "Filter by policy pack (repeatable)",
			values: func(f *policyIssueFilters) *[]string { return &f.packs },
		},
		{
			flag: "policy", colID: "policyName",
			usage:  "Filter by policy name (repeatable)",
			values: func(f *policyIssueFilters) *[]string { return &f.policies },
		},
		{
			flag: "group", colID: "policyGroupName",
			usage:  "Filter by policy group (repeatable)",
			values: func(f *policyIssueFilters) *[]string { return &f.groups },
		},
		{
			flag: "resource-type", colID: "resourceType",
			usage:  "Filter by resource type (repeatable)",
			values: func(f *policyIssueFilters) *[]string { return &f.resourceTypes },
		},
		{
			flag: "urn", colID: "resourceURN",
			usage:  "Filter by resource URN (repeatable)",
			values: func(f *policyIssueFilters) *[]string { return &f.urns },
		},
		{
			flag: "assigned-to", colID: "assignedTo",
			usage:  "Filter by assignee login (repeatable)",
			values: func(f *policyIssueFilters) *[]string { return &f.assignedTo },
		},
	}
}

func (c policyIssueFilterColumn) fullUsage() string {
	if len(c.allowed) == 0 {
		return c.usage
	}
	return fmt.Sprintf("%s. One of: %s", c.usage, strings.Join(c.allowed, ", "))
}

// addPolicyIssueFilterFlags registers the filter and sort flags and binds them to f.
func addPolicyIssueFilterFlags(flags *pflag.FlagSet, f *policyIssueFilters) {
	for _, c := range policyIssueFilterColumns() {
		flags.StringArrayVar(c.values(f), c.flag, nil, c.fullUsage())
	}
	flags.StringArrayVar(&f.stacks, "stack", nil,
		"Filter by stack, as <project>/<stack> (repeatable)")
	flags.StringArrayVar(&f.accounts, "account", nil,
		"Filter by Insights account, as [<project>/]<account> (repeatable)")
	flags.StringArrayVar(&f.sort, "sort", nil,
		"Sort by <field>[:asc|desc], default asc (repeatable). Fields: "+
			strings.Join(policyIssueSortFields, ", "))
}

// empty reports whether no filter or sort flag was set.
func (f policyIssueFilters) empty() bool {
	return len(f.severities)+len(f.priorities)+len(f.statuses)+
		len(f.packs)+len(f.policies)+len(f.groups)+
		len(f.stacks)+len(f.accounts)+
		len(f.resourceTypes)+len(f.urns)+
		len(f.assignedTo)+len(f.sort) == 0
}

func gridLeaf(colID, value string) cloudapitype.AngularGridFilterModel {
	return cloudapitype.AngularGridFilterModel{
		ColID:      colID,
		FilterType: gridFilterTypeText,
		Type:       gridOpEquals,
		Filter:     value,
	}
}

func gridGroup(combinator string, conds []cloudapitype.AngularGridFilterModel) cloudapitype.AngularGridFilterModel {
	if len(conds) == 1 {
		return conds[0]
	}
	return cloudapitype.AngularGridFilterModel{FilterType: gridFilterTypeJoin, Type: combinator, Conditions: conds}
}

func validateFlagValues(flag string, values, allowed []string) error {
	for _, v := range values {
		if v == "" {
			return fmt.Errorf("invalid value %q for --%s: must not be empty", v, flag)
		}
		if allowed != nil && !slices.Contains(allowed, v) {
			return fmt.Errorf("invalid value %q for --%s: must be one of %s", v, flag, strings.Join(allowed, ", "))
		}
	}
	return nil
}

func validateAccount(a string) error {
	project, account, ok := strings.Cut(a, "/")
	if ok && (project == "" || account == "" || strings.Contains(account, "/")) {
		return fmt.Errorf("invalid value %q for --account: expected [<project>/]<account>", a)
	}
	return nil
}

func validateStack(s string) error {
	project, stack, ok := strings.Cut(s, "/")
	if !ok || project == "" || stack == "" || strings.Contains(stack, "/") {
		return fmt.Errorf("invalid value %q for --stack: expected <project>/<stack>", s)
	}
	return nil
}

// validatePolicyIssueFilters checks every filter and sort flag value without needing the current user, so bad
// input is rejected before the backend is resolved.
func validatePolicyIssueFilters(f policyIssueFilters) error {
	for _, c := range policyIssueFilterColumns() {
		if err := validateFlagValues(c.flag, *c.values(&f), c.allowed); err != nil {
			return err
		}
	}
	if err := validateFlagValues("stack", f.stacks, nil); err != nil {
		return err
	}
	for _, s := range f.stacks {
		if err := validateStack(s); err != nil {
			return err
		}
	}
	if err := validateFlagValues("account", f.accounts, nil); err != nil {
		return err
	}
	for _, a := range f.accounts {
		if err := validateAccount(a); err != nil {
			return err
		}
	}
	if err := validateFlagValues("sort", f.sort, nil); err != nil {
		return err
	}
	_, err := buildSortModel(f.sort)
	return err
}

func entityFilter(parts ...[2]string) cloudapitype.AngularGridFilterModel {
	conds := make([]cloudapitype.AngularGridFilterModel, 0, len(parts))
	for _, p := range parts {
		conds = append(conds, gridLeaf(p[0], p[1]))
	}
	return cloudapitype.AngularGridFilterModel{FilterType: gridFilterTypeJoin, Type: gridCombinatorAnd, Conditions: conds}
}

func buildStackCondition(stacks []string) cloudapitype.AngularGridFilterModel {
	conds := make([]cloudapitype.AngularGridFilterModel, 0, len(stacks))
	for _, s := range stacks {
		project, stack, _ := strings.Cut(s, "/")
		conds = append(conds, entityFilter(
			[2]string{colEntityType, string(cloudapitype.IssueEntityTypeStack)},
			[2]string{colEntityProject, project},
			[2]string{colEntityID, stack},
		))
	}
	return gridGroup(gridCombinatorOr, conds)
}

func buildAccountCondition(accounts []string) cloudapitype.AngularGridFilterModel {
	conds := make([]cloudapitype.AngularGridFilterModel, 0, len(accounts))
	for _, a := range accounts {
		project, account, ok := strings.Cut(a, "/")
		if !ok {
			// The service uses the account name as the project for accounts without one.
			account = project
		}
		conds = append(conds, entityFilter(
			[2]string{colEntityType, string(cloudapitype.IssueEntityTypeInsightsAccount)},
			[2]string{colEntityProject, project},
			[2]string{colEntityID, account},
		))
	}
	return gridGroup(gridCombinatorOr, conds)
}

func buildSortModel(specs []string) ([]cloudapitype.AngularGridSortModelItem, error) {
	if len(specs) == 0 {
		return nil, nil
	}
	items := make([]cloudapitype.AngularGridSortModelItem, 0, len(specs))
	for _, spec := range specs {
		field, dir, hasDir := strings.Cut(spec, ":")
		if !slices.Contains(policyIssueSortFields, field) {
			return nil, fmt.Errorf("invalid field %q for --sort: must be one of %s",
				field, strings.Join(policyIssueSortFields, ", "))
		}
		if !hasDir {
			dir = gridSortAsc
		}
		if !slices.Contains(policyIssueSortDirections, dir) {
			return nil, fmt.Errorf("invalid direction %q for --sort: must be one of %s",
				dir, strings.Join(policyIssueSortDirections, ", "))
		}
		items = append(items, cloudapitype.AngularGridSortModelItem{ColID: field, Sort: dir})
	}
	return items, nil
}

// buildPolicyIssueGrid converts the filter flags into the grid filter and sort models sent to the service.
// It returns nil for a model whose flags were not set.
func buildPolicyIssueGrid(
	f policyIssueFilters,
) (*cloudapitype.AngularGridAdvancedFilterModel, []cloudapitype.AngularGridSortModelItem, error) {
	if err := validatePolicyIssueFilters(f); err != nil {
		return nil, nil, err
	}
	sortModel, err := buildSortModel(f.sort)
	if err != nil {
		return nil, nil, err
	}

	var perFlag []cloudapitype.AngularGridFilterModel
	addColumn := func(c policyIssueFilterColumn) error {
		values := *c.values(&f)
		if len(values) == 0 {
			return nil
		}
		leaves := make([]cloudapitype.AngularGridFilterModel, 0, len(values))
		for _, v := range values {
			leaves = append(leaves, gridLeaf(c.colID, v))
		}
		perFlag = append(perFlag, gridGroup(gridCombinatorOr, leaves))
		return nil
	}

	for _, c := range policyIssueFilterColumns() {
		if c.flag == "resource-type" {
			if len(f.stacks) > 0 {
				perFlag = append(perFlag, buildStackCondition(f.stacks))
			}
			if len(f.accounts) > 0 {
				perFlag = append(perFlag, buildAccountCondition(f.accounts))
			}
		}
		if err := addColumn(c); err != nil {
			return nil, nil, err
		}
	}

	if len(perFlag) == 0 {
		return nil, sortModel, nil
	}
	return &cloudapitype.AngularGridAdvancedFilterModel{Type: gridCombinatorAnd, Conditions: perFlag}, sortModel, nil
}
