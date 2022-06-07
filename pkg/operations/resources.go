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
	"sort"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
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
func (r *Resource) OperationsProvider(config map[config.Key]string) Provider {
	return &resourceOperations{
		resource: r,
		config:   config,
	}
}

// ResourceOperations is an OperationsProvider for Resources
type resourceOperations struct {
	resource *Resource
	config   map[config.Key]string
}

var _ Provider = (*resourceOperations)(nil)

// GetLogs gets logs for a Resource
func (ops *resourceOperations) GetLogs(query LogQuery) (*[]LogEntry, error) {
	if ops.resource == nil {
		return nil, nil
	}

	// Only get logs for this resource if it matches the resource filter query
	if ops.matchesResourceFilter(query.ResourceFilter) {
		// Set query to be a new query with `ResourceFilter` nil so that we don't filter out logs from any children of
		// this resource since this resource did match the resource filter.
		query = LogQuery{
			StartTime:      query.StartTime,
			EndTime:        query.EndTime,
			ResourceFilter: nil,
		}
		// Try to get an operations provider for this resource, it may be `nil`
		opsProvider, err := ops.getOperationsProvider()
		if err != nil {
			return nil, err
		}
		if opsProvider != nil {
			// If this resource has an operations provider - use it and don't recur into children.  It is the
			// responsibility of it's GetLogs implementation to aggregate all logs from children, either by passing them
			// through or by filtering specific content out.
			logsResult, err := opsProvider.GetLogs(query)
			if err != nil {
				return logsResult, err
			}
			if logsResult != nil {
				return logsResult, nil
			}
		}
	}
	// If this resource did not choose to provide it's own logs, recur into children and collect + aggregate their logs.
	var logs []LogEntry
	// Kick off GetLogs on all children in parallel, writing results to shared channels
	ch := make(chan *[]LogEntry)
	errch := make(chan error)
	for _, child := range ops.resource.Children {
		childOps := &resourceOperations{
			resource: child,
			config:   ops.config,
		}
		go func() {
			childLogs, err := childOps.GetLogs(query)
			ch <- childLogs
			errch <- err
		}()
	}
	// Handle results from GetLogs calls as they complete
	var err error
	for range ops.resource.Children {
		childLogs := <-ch
		childErr := <-errch
		if childErr != nil {
			err = multierror.Append(err, childErr)
		}
		if childLogs != nil {
			logs = append(logs, *childLogs...)
		}
	}
	if err != nil {
		return &logs, err
	}
	// Sort
	sort.SliceStable(logs, func(i, j int) bool { return logs[i].Timestamp < logs[j].Timestamp })
	// Remove duplicates
	var retLogs []LogEntry
	var lastLogTimestamp int64
	var lastLogs []LogEntry
	for _, log := range logs {
		shouldContinue := false
		if log.Timestamp == lastLogTimestamp {
			for _, lastLog := range lastLogs {
				if log.Message == lastLog.Message {
					shouldContinue = true
					break
				}
			}
		} else {
			lastLogs = nil
		}
		if shouldContinue {
			continue
		}
		lastLogs = append(lastLogs, log)
		lastLogTimestamp = log.Timestamp
		retLogs = append(retLogs, log)
	}
	return &retLogs, nil
}

// matchesResourceFilter determines whether this resource matches the provided resource filter.
func (ops *resourceOperations) matchesResourceFilter(filter *ResourceFilter) bool {
	if filter == nil {
		// No filter, all resources match it.
		return true
	}
	if ops.resource == nil || ops.resource.State == nil {
		return false
	}
	urn := ops.resource.State.URN
	if resource.URN(*filter) == urn {
		// The filter matched the full URN
		return true
	}
	if string(*filter) == string(urn.Type())+"::"+string(urn.Name()) {
		// The filter matched the '<type>::<name>' part of the URN
		return true
	}
	if tokens.QName(*filter) == urn.Name() {
		// The filter matched the '<name>' part of the URN
		return true
	}
	return false
}

func (ops *resourceOperations) getOperationsProvider() (Provider, error) {
	if ops.resource == nil || ops.resource.State == nil {
		return nil, nil
	}

	tokenSeparators := strings.Count(ops.resource.State.Type.String(), ":")
	if tokenSeparators != 2 {
		return nil, nil
	}

	switch ops.resource.State.Type.Package() {
	case "cloud":
		return CloudOperationsProvider(ops.config, ops.resource)
	case "aws":
		return AWSOperationsProvider(ops.config, ops.resource)
	case "gcp":
		return GCPOperationsProvider(ops.config, ops.resource)
	default:
		return nil, nil
	}
}
