// Copyright 2016-2018, Pulumi Corporation.
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

package operations

import (
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	logs "go.opentelemetry.io/proto/otlp/logs/v1"
	metrics "go.opentelemetry.io/proto/otlp/metrics/v1"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Resource is a tree representation of a resource/component hierarchy
type Resource struct {
	Stack    tokens.QName
	Project  tokens.PackageName
	State    *resource.State
	Parent   *Resource
	Children map[resource.URN]*Resource
}

// NewResourceMap constructs a map of resources with parent/child relations, indexed by URN.
func NewResourceMap(source []*resource.State) map[resource.URN]*Resource {
	_, resources := makeResourceTreeMap(source)
	return resources
}

// NewResourceTree constructs a tree representation of a resource/component hierarchy
func NewResourceTree(source []*resource.State) *Resource {
	root, _ := makeResourceTreeMap(source)
	return root
}

// makeResourceTreeMap is a helper used by the two above functions to construct a resource hierarchy.
func makeResourceTreeMap(source []*resource.State) (*Resource, map[resource.URN]*Resource) {
	resources := make(map[resource.URN]*Resource)

	var stack tokens.QName
	var proj tokens.PackageName

	// First create a list of resource nodes, without parent/child relations hooked up.
	for _, state := range source {
		stack = state.URN.Stack()
		proj = state.URN.Project()
		if !state.Delete {
			// Only include resources which are not marked as pending-deletion.
			contract.Assertf(resources[state.URN] == nil, "Unexpected duplicate resource %s", state.URN)
			resources[state.URN] = &Resource{
				Stack:    stack,
				Project:  proj,
				State:    state,
				Children: make(map[resource.URN]*Resource),
			}
		}
	}

	// Next, walk the list of resources, and wire up parents and children.  We do this in a second pass so
	// that the creation of the tree isn't order dependent.
	for _, child := range resources {
		if parurn := child.State.Parent; parurn != "" {
			parent, ok := resources[parurn]
			contract.Assertf(ok, "Expected to find parent node '%v' in checkpoint tree nodes", parurn)
			child.Parent = parent
			parent.Children[child.State.URN] = child
		}
	}

	// Create a single root node which is the parent of all unparented nodes
	root := &Resource{
		Stack:    stack,
		Project:  proj,
		State:    nil,
		Parent:   nil,
		Children: make(map[resource.URN]*Resource),
	}
	for _, node := range resources {
		if node.Parent == nil {
			root.Children[node.State.URN] = node
			node.Parent = root
		}
	}

	// Return the root node and map of children.
	return root, resources
}

// GetChild find a child with the given type and name or returns `nil`.
func (r *Resource) GetChild(typ string, name string) (*Resource, bool) {
	for childURN, childResource := range r.Children {
		if childURN.Stack() == r.Stack &&
			childURN.Project() == r.Project &&
			childURN.Type() == tokens.Type(typ) &&
			childURN.Name() == tokens.QName(name) {
			return childResource, true
		}
	}

	return nil, false
}

// OperationsProvider gets an OperationsProvider for this resource.
func (r *Resource) OperationsProvider(providers *engine.Providers) Provider {
	return &resourceOperations{
		resource:  r,
		providers: providers,
	}
}

// ResourceOperations is an OperationsProvider for Resources
type resourceOperations struct {
	resource  *Resource
	providers *engine.Providers
}

var _ Provider = (*resourceOperations)(nil)

func (ops *resourceOperations) GetLogs(query LogQuery) ([]*logs.ResourceLogs, interface{}, error) {
	if ops.resource == nil {
		return nil, "", nil
	}

	if query.EndTime == nil {
		now := time.Now()
		query.EndTime = &now
	}
	if query.StartTime == nil {
		t := query.EndTime.Add(-1 * time.Hour)
		query.StartTime = &t
	}

	// Only get logs for this resource if it matches the resource filter query
	if ops.matchesResourceFilter(query.ResourceFilter) {
		// Set query to be a new query with `ResourceFilter` nil so that we don't filter out logs from any children of
		// this resource since this resource did match the resource filter.
		query.ResourceFilter = ""

		// Try to get an operations provider for this resource, it may be `nil`
		opsProvider, err := ops.getOperationsProvider()
		if err != nil {
			return nil, "", err
		}
		if opsProvider != nil {
			// If this resource has an operations provider - use it and don't recur into children. It is the
			// responsibility of its GetLogs implementation to aggregate all logs from children, either by passing them
			// through or by filtering specific content out.
			logsResult, continuationToken, err := opsProvider.GetLogs(query)
			if err != nil {
				return nil, nil, err
			}
			return logsResult, continuationToken, nil
		}
	}

	type getLogsResult struct {
		urn   resource.URN
		logs  []*logs.ResourceLogs
		token interface{}
		err   error
	}

	tokens, _ := query.ContinuationToken.(map[resource.URN]interface{})

	// If this resource did not choose to provide its own logs, recur into children and collect + aggregate their logs.
	var logs []*logs.ResourceLogs
	var nextTokens map[resource.URN]interface{}
	// Kick off GetLogs on all children in parallel, writing results to shared channels
	ch := make(chan getLogsResult)
	results := 0
	for _, child := range ops.resource.Children {
		childToken, ok := tokens[child.State.URN]
		if tokens != nil && !ok {
			continue
		}
		results++

		childOps := &resourceOperations{
			resource:  child,
			providers: ops.providers,
		}
		childQuery := query
		childQuery.ContinuationToken = childToken
		go func() {
			childLogs, token, err := childOps.GetLogs(childQuery)
			ch <- getLogsResult{childOps.resource.State.URN, childLogs, token, err}
		}()
	}
	// Handle results from GetLogs calls as they complete
	var err error
	for i := 0; i < results; i++ {
		result := <-ch
		if result.err != nil {
			err = multierror.Append(err, result.err)
			continue
		}

		logs = append(logs, result.logs...)
		if result.token != nil {
			if nextTokens == nil {
				nextTokens = map[resource.URN]interface{}{}
			}
			nextTokens[result.urn] = result.token
		}
	}
	if err != nil {
		return nil, nil, err
	}

	var nextToken interface{}
	if len(nextTokens) != 0 {
		nextToken = nextTokens
	}
	return logs, nextToken, nil
}

func (ops *resourceOperations) GetMetrics(query MetricsQuery) ([]*metrics.ResourceMetrics, interface{}, error) {
	if ops.resource == nil {
		return nil, "", nil
	}

	if query.EndTime == nil {
		now := time.Now()
		query.EndTime = &now
	}
	if query.StartTime == nil {
		t := query.EndTime.Add(-1 * time.Hour)
		query.StartTime = &t
	}

	// Only get logs for this resource if it matches the resource filter query
	if ops.matchesResourceFilter(query.ResourceFilter) {
		// Set query to be a new query with `ResourceFilter` nil so that we don't filter out logs from any children of
		// this resource since this resource did match the resource filter.
		query.ResourceFilter = ""

		// Try to get an operations provider for this resource, it may be `nil`
		opsProvider, err := ops.getOperationsProvider()
		if err != nil {
			return nil, "", err
		}
		if opsProvider != nil {
			// If this resource has an operations provider - use it and don't recur into children. It is the
			// responsibility of its GetMetrics implementation to aggregate all logs from children, either by passing them
			// through or by filtering specific content out.
			logsResult, continuationToken, err := opsProvider.GetMetrics(query)
			if err != nil {
				return nil, nil, err
			}
			return logsResult, continuationToken, nil
		}
	}

	type getMetricsResult struct {
		urn   resource.URN
		metrics  []*metrics.ResourceMetrics
		token interface{}
		err   error
	}

	tokens, _ := query.ContinuationToken.(map[resource.URN]interface{})

	// If this resource did not choose to provide its own metrics, recur into children and collect + aggregate their metrics.
	var metrics []*metrics.ResourceMetrics
	var nextTokens map[resource.URN]interface{}
	// Kick off GetMetrics on all children in parallel, writing results to shared channels
	ch := make(chan getMetricsResult)
	results := 0
	for _, child := range ops.resource.Children {
		childToken, ok := tokens[child.State.URN]
		if tokens != nil && !ok {
			continue
		}
		results++

		childOps := &resourceOperations{
			resource:  child,
			providers: ops.providers,
		}
		childQuery := query
		childQuery.ContinuationToken = childToken
		go func() {
			childMetrics, token, err := childOps.GetMetrics(childQuery)
			ch <- getMetricsResult{childOps.resource.State.URN, childMetrics, token, err}
		}()
	}
	// Handle results from GetMetrics calls as they complete
	var err error
	for i := 0; i < results; i++ {
		result := <-ch
		if result.err != nil {
			err = multierror.Append(err, result.err)
			continue
		}

		metrics = append(metrics, result.metrics...)
		if result.token != nil {
			if nextTokens == nil {
				nextTokens = map[resource.URN]interface{}{}
			}
			nextTokens[result.urn] = result.token
		}
	}
	if err != nil {
		return nil, nil, err
	}

	var nextToken interface{}
	if len(nextTokens) != 0 {
		nextToken = nextTokens
	}
	return metrics, nextToken, nil
}

// matchesResourceFilter determines whether this resource matches the provided resource filter.
func (ops *resourceOperations) matchesResourceFilter(filter ResourceFilter) bool {
	if filter == "" {
		// No filter, all resources match it.
		return true
	}
	if ops.resource == nil || ops.resource.State == nil {
		return false
	}
	urn := ops.resource.State.URN
	if resource.URN(filter) == urn {
		// The filter matched the full URN
		return true
	}
	if filter == string(urn.Type())+"::"+string(urn.Name()) {
		// The filter matched the '<type>::<name>' part of the URN
		return true
	}
	if tokens.QName(filter) == urn.Name() {
		// The filter matched the '<name>' part of the URN
		return true
	}
	return false
}

func (ops *resourceOperations) getOperationsProvider() (Provider, error) {
	if ops.resource == nil || ops.resource.State == nil || ops.resource.State.Provider == "" {
		return nil, nil
	}

	ref, err := providers.ParseReference(ops.resource.State.Provider)
	if err != nil {
		return nil, err
	}

	provider, ok := ops.providers.GetProvider(ref)
	if !ok {
		return nil, fmt.Errorf("missing provider %q", ops.resource.State.Provider)
	}

	return &pluginOpsProvider{provider: provider, resource: ops.resource.State}, nil
}
