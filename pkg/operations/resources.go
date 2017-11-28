package operations

import (
	"fmt"
	"sort"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// Resource is a tree representation of a resource/component hierarchy
type Resource struct {
	NS       tokens.QName
	Alloc    tokens.PackageName
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

	var ns tokens.QName
	var alloc tokens.PackageName

	// Walk the ordered resource list and build tree nodes based on child relationships
	for _, state := range source {
		ns = state.URN.Namespace()
		alloc = state.URN.Alloc()
		newTree := &Resource{
			NS:       ns,
			Alloc:    alloc,
			State:    state,
			Parent:   nil,
			Children: map[resource.URN]*Resource{},
		}
		for _, childURN := range state.Children {
			childTree, ok := resources[childURN]
			contract.Assertf(ok, "Expected children to be before parents in resource checkpoint")
			childTree.Parent = newTree
			newTree.Children[childTree.State.URN] = childTree
		}
		resources[state.URN] = newTree
	}

	// Create a single root node which is the parent of all unparented nodes
	root := &Resource{
		NS:       ns,
		Alloc:    alloc,
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
func (r *Resource) GetChild(typ string, name string) *Resource {
	childURN := resource.NewURN(r.NS, r.Alloc, tokens.Type(typ), tokens.QName(name))
	return r.Children[childURN]
}

// OperationsProvider gets an OperationsProvider for this resource.
func (r *Resource) OperationsProvider(config map[tokens.ModuleMember]string) Provider {
	return &resourceOperations{
		resource: r,
		config:   config,
	}
}

// ResourceOperations is an OperationsProvider for Resources
type resourceOperations struct {
	resource *Resource
	config   map[tokens.ModuleMember]string
}

var _ Provider = (*resourceOperations)(nil)

// GetLogs gets logs for a Resource
func (ops *resourceOperations) GetLogs(query LogQuery) (*[]LogEntry, error) {
	// Only get logs for this resource if it matches the resource filter query
	if ops.matchesResourceFilter(query.ResourceFilter) {
		// Set query to be a new query with `ResourceFilter` nil so that we don't filter out logs from any children of
		// this resource since this resource did match the resource filter.
		query = LogQuery{
			StartTime:      query.StartTime,
			EndTime:        query.EndTime,
			Query:          query.Query,
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
	for range ops.resource.Children {
		childLogs := <-ch
		err := <-errch
		if err != nil {
			return &logs, err
		}
		if childLogs != nil {
			logs = append(logs, *childLogs...)
		}
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

// ListMetrics lists metrics for a Resource
func (ops *resourceOperations) ListMetrics() []MetricName {
	return []MetricName{}
}

// GetMetricStatistics gets metric statistics for a Resource
func (ops *resourceOperations) GetMetricStatistics(metric MetricRequest) ([]MetricDataPoint, error) {
	return nil, fmt.Errorf("not yet implemented")
}

func (ops *resourceOperations) getOperationsProvider() (Provider, error) {
	if ops.resource == nil || ops.resource.State == nil {
		return nil, nil
	}
	switch ops.resource.State.Type.Package() {
	case "cloud":
		return CloudOperationsProvider(ops.config, ops.resource)
	case "aws":
		return AWSOperationsProvider(ops.config, ops.resource)
	default:
		return nil, nil
	}
}
