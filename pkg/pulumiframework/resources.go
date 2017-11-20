package pulumiframework

import (
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/component"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// This file contains the implementation of the component.Components interface for the
// AWS implementation of the Pulumi Framework defined in this repo.

// This function grovels through the given configuration bag, extracts the bits necessary to create an AWS session
// (currently just the AWS region to target), and creates and returns the session. If the bag does not contain the
// necessary properties or if session creation fails, this function returns `nil, error`.
func createSessionFromConfig(config map[tokens.ModuleMember]string) (*session.Session, error) {
	awsRegion, ok := config[regionKey]
	if !ok {
		return nil, errors.New("no AWS region found")
	}

	awsConfig := aws.NewConfig()
	awsConfig.Region = aws.String(awsRegion)
	return session.NewSession(awsConfig)
}

// OperationsProviderForComponent creates an OperationsProvider capable of answering
// operational queries based on the underlying resources of the AWS  Pulumi Framework implementation.
func OperationsProviderForComponent(
	config map[tokens.ModuleMember]string,
	component *Resource) (component.OperationsProvider, error) {

	sess, err := createSessionFromConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create AWS session")
	}

	prov := &componentOpsProvider{
		awsConnection: newAWSConnection(sess),
		component:     component,
	}
	return prov, nil
}

type componentOpsProvider struct {
	awsConnection *awsConnection
	component     *Resource
}

var _ component.OperationsProvider = (*componentOpsProvider)(nil)

const (
	// AWS config keys
	regionKey = "aws:config:region"

	// Pulumi Framework "virtual" types
	pulumiEndpointType = tokens.Type("pulumi:framework:Endpoint")
	pulumiTopicType    = tokens.Type("pulumi:framework:Topic")
	pulumiTimerType    = tokens.Type("pulumi:framework:Timer")
	pulumiTableType    = tokens.Type("pulumi:framework:Table")
	pulumiFunctionType = tokens.Type("pulumi:framework:Function")

	// Operational metric names for Pulumi Framework components
	functionInvocations        component.MetricName = "Invocation"
	functionDuration           component.MetricName = "Duration"
	functionErrors             component.MetricName = "Errors"
	functionThrottles          component.MetricName = "Throttles"
	endpoint4xxError           component.MetricName = "4xxerror"
	endpoint5xxError           component.MetricName = "5xxerror"
	endpointCount              component.MetricName = "count"
	endpointLatency            component.MetricName = "latency"
	topicPulished              component.MetricName = "published"
	topicPublishSize           component.MetricName = "publishsize"
	topicDelivered             component.MetricName = "delivered"
	topicFailed                component.MetricName = "failed"
	timerInvocations           component.MetricName = "invocations"
	timerFailedInvocations     component.MetricName = "failedinvocations"
	tableConsumedReadCapacity  component.MetricName = "consumedreadcapacity"
	tableConsumedWriteCapacity component.MetricName = "consumerwritecapacity"
	tableThrottles             component.MetricName = "throttles"
)

func (ops *componentOpsProvider) GetLogs(query component.LogQuery) ([]component.LogEntry, error) {
	if query.StartTime != nil || query.EndTime != nil || query.Query != nil {
		contract.Failf("not yet implemented - StartTime, Endtime, Query")
	}
	fmt.Printf("[GetLogs] type = %v", ops.component.state.Type)
	switch ops.component.state.Type {
	case pulumiFunctionType:
		urn := ops.component.state.URN
		serverlessFunction := ops.component.GetChild("aws:serverless:Function", string(urn.Name()))
		awsFunction := serverlessFunction.GetChild("aws:lambda/function:Function", string(urn.Name()))
		functionName := awsFunction.state.Outputs["name"].StringValue()
		logResult := ops.awsConnection.getLogsForLogGroupsConcurrently([]string{functionName}, []string{"/aws/lambda/" + functionName})
		sort.SliceStable(logResult, func(i, j int) bool { return logResult[i].Timestamp < logResult[j].Timestamp })
		return logResult, nil
	default:
		return nil, fmt.Errorf("Logs not supported for component type: %s", ops.component.state.Type)
	}
}

func (ops *componentOpsProvider) ListMetrics() []component.MetricName {
	switch ops.component.state.Type {
	case pulumiFunctionType:
		// Don't include these which are internal implementation metrics: DLQ delivery errors
		return []component.MetricName{functionInvocations, functionDuration, functionErrors, functionThrottles}
	case pulumiEndpointType:
		return []component.MetricName{endpoint4xxError, endpoint5xxError, endpointCount, endpointLatency}
	case pulumiTopicType:
		return []component.MetricName{topicPulished, topicPublishSize, topicDelivered, topicFailed}
	case pulumiTimerType:
		return []component.MetricName{timerInvocations, timerFailedInvocations}
	case pulumiTableType:
		// Internal only: "provisionedreadcapacity", "provisionedwritecapacity", "usererrors", "timetolivedeleted",
		// "systemerrors", "succesfulrequestlatency", "returnedrecordscount", "returenditemcount", "returnedbytes",
		// "onlineindex*", "conditionalcheckfailed"
		return []component.MetricName{tableConsumedReadCapacity, tableConsumedWriteCapacity, tableThrottles}
	default:
		contract.Failf("invalid component type")
		return nil
	}
}

func (ops *componentOpsProvider) GetMetricStatistics(metric component.MetricRequest) ([]component.MetricDataPoint, error) {

	return nil, fmt.Errorf("Not yet implmeneted: GetMetricStatistics")

	// var dimensions []*cloudwatch.Dimension
	// var namespace string

	// switch ops.component.state.Type {
	// case pulumiFunctionType:
	// 	dimensions = append(dimensions, &cloudwatch.Dimension{
	// 		Name:  aws.String("FunctionName"),
	// 		Value: aws.String(string(ops.component.Resources["function"].ID)),
	// 	})
	// 	namespace = "AWS/Lambda"
	// case pulumiEndpointType:
	// 	contract.Failf("not yet implemented")
	// case pulumiTopicType:
	// 	contract.Failf("not yet implemented")
	// case pulumiTimerType:
	// 	contract.Failf("not yet implemented")
	// case pulumiTableType:
	// 	contract.Failf("not yet implemented")
	// default:
	// 	contract.Failf("invalid component type")
	// }

	// resp, err := ops.awsConnection.metricSvc.GetMetricStatistics(&cloudwatch.GetMetricStatisticsInput{
	// 	Namespace:  aws.String(namespace),
	// 	MetricName: aws.String(metric.Name),
	// 	Dimensions: dimensions,
	// 	Statistics: []*string{
	// 		aws.String("Sum"), aws.String("SampleCount"), aws.String("Average"),
	// 		aws.String("Maximum"), aws.String("Minimum"),
	// 	},
	// })
	// if err != nil {
	// 	return nil, err
	// }

	// var metrics []component.MetricDataPoint
	// for _, datapoint := range resp.Datapoints {
	// 	metrics = append(metrics, component.MetricDataPoint{
	// 		Timestamp:   aws.TimeValue(datapoint.Timestamp),
	// 		Unit:        aws.StringValue(datapoint.Unit),
	// 		Sum:         aws.Float64Value(datapoint.Sum),
	// 		SampleCount: aws.Float64Value(datapoint.SampleCount),
	// 		Average:     aws.Float64Value(datapoint.Average),
	// 		Maximum:     aws.Float64Value(datapoint.Maximum),
	// 		Minimum:     aws.Float64Value(datapoint.Minimum),
	// 	})
	// }
	// return metrics, nil
}

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

func getOperationsProvider(resource *Resource, config map[tokens.ModuleMember]string) (component.OperationsProvider, error) {
	if resource == nil || resource.state == nil {
		return nil, nil
	}
	switch resource.state.Type {
	case "cloud:function:Function":
		return cloudFunctionOperationsProvider(resource, config)
	default:
		return nil, nil
	}
}

func cloudFunctionOperationsProvider(resource *Resource, config map[tokens.ModuleMember]string) (component.OperationsProvider, error) {
	return OperationsProviderForComponent(config, resource)
}

// ResourceOperations is an OperationsProvider for Resources
type ResourceOperations struct {
	resource *Resource
	config   map[tokens.ModuleMember]string
}

var _ component.OperationsProvider = (*ResourceOperations)(nil)

// NewResourceOperations constructs an OperationsProvider for a resource and configuration options
func NewResourceOperations(config map[tokens.ModuleMember]string, resource *Resource) *ResourceOperations {
	return &ResourceOperations{
		resource: resource,
		config:   config,
	}
}

// GetLogs gets logs for a Resource
func (ops *ResourceOperations) GetLogs(query component.LogQuery) ([]component.LogEntry, error) {
	fmt.Printf("[ResourceOperations.GetLogs]: %v\n", query)
	opsProvider, err := getOperationsProvider(ops.resource, ops.config)
	if err != nil {
		return nil, err
	}
	if opsProvider != nil {
		// If this resource has an operations provider - use it and don't recur into children.  It is the responsibility
		// of it's GetLogs implementation to aggregate all logs from children, either by passing them through or by
		// filtering specific content out.
		//
		// Note: We should also allow it to have a resource provider but to elect not to
		// handle GetLogs.
		return opsProvider.GetLogs(query)
	}
	var logs []component.LogEntry
	for _, child := range ops.resource.children {
		childOps := &ResourceOperations{
			resource: child,
			config:   ops.config,
		}
		childLogs, err := childOps.GetLogs(query)
		if err != nil {
			return logs, err
		}
		logs = append(logs, childLogs...)
	}
	return logs, nil
}

// ListMetrics lists metrics for a Resource
func (ops *ResourceOperations) ListMetrics() []component.MetricName {
	return []component.MetricName{}
}

// GetMetricStatistics gets metric statistics for a Resource
func (ops *ResourceOperations) GetMetricStatistics(metric component.MetricRequest) ([]component.MetricDataPoint, error) {
	return nil, fmt.Errorf("not yet implemented")
}
