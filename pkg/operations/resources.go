package operations

import (
	"fmt"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// Resource is a tree representation of a resource/component hierarchy
type Resource struct {
	ns       tokens.QName
	alloc    tokens.PackageName
	state    *resource.State
	parent   *Resource
	children map[resource.URN]*Resource
}

// NewResource constructs a tree representation of a resource/component hierarchy
func NewResource(source []*resource.State) *Resource {
	treeNodes := map[resource.URN]*Resource{}
	var ns tokens.QName
	var alloc tokens.PackageName

	// Walk the ordered resource list and build tree nodes based on child relationships
	for _, state := range source {
		ns = state.URN.Namespace()
		alloc = state.URN.Alloc()
		newTree := &Resource{
			ns:       ns,
			alloc:    alloc,
			state:    state,
			parent:   nil,
			children: map[resource.URN]*Resource{},
		}
		for _, childURN := range state.Children {
			childTree, ok := treeNodes[childURN]
			contract.Assertf(ok, "Expected children to be before parents in resource checkpoint")
			childTree.parent = newTree
			newTree.children[childTree.state.URN] = childTree
		}
		treeNodes[state.URN] = newTree
	}

	// Create a single root node which is the parent of all unparented nodes
	root := &Resource{
		ns:       ns,
		alloc:    alloc,
		state:    nil,
		parent:   nil,
		children: map[resource.URN]*Resource{},
	}
	for _, node := range treeNodes {
		if node.parent == nil {
			root.children[node.state.URN] = node
			node.parent = root
		}
	}

	// Return the root node
	return root
}

// GetChild find a child with the given type and name or returns `nil`.
func (r *Resource) GetChild(typ string, name string) *Resource {
	childURN := resource.NewURN(r.ns, r.alloc, tokens.Type(typ), tokens.QName(name))
	return r.children[childURN]
}

// OperationsProvider gets an OperationsProvider for this resource.
func (r *Resource) OperationsProvider(config map[tokens.ModuleMember]string) Provider {
	return &resourceOperations{
		resource: r,
		config:   config,
	}
}

func getOperationsProvider(resource *Resource, config map[tokens.ModuleMember]string) (Provider, error) {
	if resource == nil || resource.state == nil {
		return nil, nil
	}
	switch resource.state.Type.Package() {
	case "cloud":
		return CloudOperationsProvider(config, resource)
	default:
		return nil, nil
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
	opsProvider, err := getOperationsProvider(ops.resource, ops.config)
	if err != nil {
		return nil, err
	}
	if opsProvider != nil {
		// If this resource has an operations provider - use it and don't recur into children.  It is the responsibility
		// of it's GetLogs implementation to aggregate all logs from children, either by passing them through or by
		// filtering specific content out.
		logsResult, err := opsProvider.GetLogs(query)
		if err != nil {
			return logsResult, err
		}
		if logsResult != nil {
			return logsResult, nil
		}
	}
	var logs []LogEntry
	for _, child := range ops.resource.children {
		childOps := &resourceOperations{
			resource: child,
			config:   ops.config,
		}
		childLogs, err := childOps.GetLogs(query)
		if err != nil {
			return &logs, err
		}
		if childLogs != nil {
			logs = append(logs, *childLogs...)
		}
	}
	return &logs, nil
}

// ListMetrics lists metrics for a Resource
func (ops *resourceOperations) ListMetrics() []MetricName {
	return []MetricName{}
}

// GetMetricStatistics gets metric statistics for a Resource
func (ops *resourceOperations) GetMetricStatistics(metric MetricRequest) ([]MetricDataPoint, error) {
	return nil, fmt.Errorf("not yet implemented")
}
