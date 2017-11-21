package pulumiframework

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/component"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// This file contains the implementation of the component.Components interface for the
// AWS implementation of the Pulumi Framework defined in this repo.

// GetComponents extracts the Pulumi Framework components from a checkpoint
// file, based on the raw resources created by the implementation of the Pulumi Framework
// in this repo.
func GetComponents(source []*resource.State) component.Components {
	sourceMap := makeIDLookup(source)
	components := make(component.Components)
	for _, res := range source {
		name := string(res.URN.Name())
		if res.Type == stageType {
			stage := res
			deployment := lookup(sourceMap, deploymentType, stage.Inputs["deployment"].StringValue())
			restAPI := lookup(sourceMap, restAPIType, stage.Inputs["restApi"].StringValue())
			baseURL := deployment.Outputs["invokeUrl"].StringValue() + stage.Inputs["stageName"].StringValue() + "/"
			restAPIName := restAPI.URN.Name()
			urn := newPulumiFrameworkURN(res.URN, pulumiEndpointType, restAPIName)
			components[urn] = &component.Component{
				Type: pulumiEndpointType,
				Properties: resource.NewPropertyMapFromMap(map[string]interface{}{
					"url": baseURL,
				}),
				Resources: map[string]*resource.State{
					"restapi":    restAPI,
					"deployment": deployment,
					"stage":      stage,
				},
			}
		} else if res.Type == eventRuleType {
			urn := newPulumiFrameworkURN(res.URN, pulumiTimerType, tokens.QName(name))
			components[urn] = &component.Component{
				Type: pulumiTimerType,
				Properties: resource.NewPropertyMapFromMap(map[string]interface{}{
					"schedule": res.Inputs["scheduleExpression"].StringValue(),
				}),
				Resources: map[string]*resource.State{
					"rule":       res,
					"target":     nil,
					"permission": nil,
				},
			}
		} else if res.Type == tableType {
			urn := newPulumiFrameworkURN(res.URN, pulumiTableType, tokens.QName(name))
			components[urn] = &component.Component{
				Type: pulumiTableType,
				Properties: resource.NewPropertyMapFromMap(map[string]interface{}{
					"primaryKey": res.Inputs["hashKey"].StringValue(),
				}),
				Resources: map[string]*resource.State{
					"table": res,
				},
			}
		} else if res.Type == topicType {
			if !strings.HasSuffix(name, "unhandled-error-topic") {
				urn := newPulumiFrameworkURN(res.URN, pulumiTopicType, tokens.QName(name))
				components[urn] = &component.Component{
					Type:       pulumiTopicType,
					Properties: resource.NewPropertyMapFromMap(map[string]interface{}{}),
					Resources: map[string]*resource.State{
						"topic": res,
					},
				}
			}
		} else if res.Type == functionType {
			if !strings.HasSuffix(name, "-log-collector") {
				urn := newPulumiFrameworkURN(res.URN, pulumiFunctionType, tokens.QName(name))
				components[urn] = &component.Component{
					Type:       pulumiFunctionType,
					Properties: resource.NewPropertyMapFromMap(map[string]interface{}{}),
					Resources: map[string]*resource.State{
						"function":              res,
						"role":                  nil,
						"roleAttachment":        nil,
						"logGroup":              nil,
						"logSubscriptionFilter": nil,
						"permission":            nil,
					},
				}
			}
		}
	}
	return components
}

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
	component *component.Component) (component.OperationsProvider, error) {

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
	component     *component.Component
}

var _ component.OperationsProvider = (*componentOpsProvider)(nil)

const (
	// AWS config keys
	regionKey = "aws:config:region"

	// AWS Resource Types
	stageType      = "aws:apigateway/stage:Stage"
	deploymentType = "aws:apigateway/deployment:Deployment"
	restAPIType    = "aws:apigateway/restApi:RestApi"
	eventRuleType  = "aws:cloudwatch/eventRule:EventRule"
	tableType      = "aws:dynamodb/table:Table"
	topicType      = "aws:sns/topic:Topic"
	functionType   = "aws:lambda/function:Function"

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
	switch ops.component.Type {
	case pulumiFunctionType:
		functionName := ops.component.Resources["function"].Outputs["name"].StringValue()
		logResult := ops.awsConnection.getLogsForFunction(functionName)
		sort.SliceStable(logResult, func(i, j int) bool { return logResult[i].Timestamp < logResult[j].Timestamp })
		return logResult, nil
	default:
		return nil, fmt.Errorf("Logs not supported for component type: %s", ops.component.Type)
	}
}

func (ops *componentOpsProvider) ListMetrics() []component.MetricName {
	switch ops.component.Type {
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

func (ops *componentOpsProvider) GetMetricStatistics(metric component.MetricRequest) (
	[]component.MetricDataPoint, error) {

	var dimensions []*cloudwatch.Dimension
	var namespace string

	switch ops.component.Type {
	case pulumiFunctionType:
		dimensions = append(dimensions, &cloudwatch.Dimension{
			Name:  aws.String("FunctionName"),
			Value: aws.String(string(ops.component.Resources["function"].ID)),
		})
		namespace = "AWS/Lambda"
	case pulumiEndpointType:
		contract.Failf("not yet implemented")
	case pulumiTopicType:
		contract.Failf("not yet implemented")
	case pulumiTimerType:
		contract.Failf("not yet implemented")
	case pulumiTableType:
		contract.Failf("not yet implemented")
	default:
		contract.Failf("invalid component type")
	}

	resp, err := ops.awsConnection.metricSvc.GetMetricStatistics(&cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String(namespace),
		MetricName: aws.String(metric.Name),
		Dimensions: dimensions,
		Statistics: []*string{
			aws.String("Sum"), aws.String("SampleCount"), aws.String("Average"),
			aws.String("Maximum"), aws.String("Minimum"),
		},
	})
	if err != nil {
		return nil, err
	}

	var metrics []component.MetricDataPoint
	for _, datapoint := range resp.Datapoints {
		metrics = append(metrics, component.MetricDataPoint{
			Timestamp:   aws.TimeValue(datapoint.Timestamp),
			Unit:        aws.StringValue(datapoint.Unit),
			Sum:         aws.Float64Value(datapoint.Sum),
			SampleCount: aws.Float64Value(datapoint.SampleCount),
			Average:     aws.Float64Value(datapoint.Average),
			Maximum:     aws.Float64Value(datapoint.Maximum),
			Minimum:     aws.Float64Value(datapoint.Minimum),
		})
	}
	return metrics, nil
}

// OperationsProviderForComponents creates an OperationsProvider capable of answering
// operational queries about a collection of Pulumi Framework Components based on the
// underlying resources of the AWS  Pulumi Framework implementation.
func OperationsProviderForComponents(
	config map[tokens.ModuleMember]string,
	components component.Components) (component.OperationsProvider, error) {

	sess, err := createSessionFromConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create AWS session")
	}

	prov := &componentsOpsProvider{
		awsConnection: newAWSConnection(sess),
		components:    components,
	}
	return prov, nil
}

type componentsOpsProvider struct {
	awsConnection *awsConnection
	components    component.Components
}

var _ component.OperationsProvider = (*componentsOpsProvider)(nil)

// GetLogs for a collection of Components returns combined logs from all Pulumi Function
// components in the collection.
func (ops *componentsOpsProvider) GetLogs(query component.LogQuery) ([]component.LogEntry, error) {
	if query.StartTime != nil || query.EndTime != nil || query.Query != nil {
		contract.Failf("not yet implemented - StartTime, Endtime, Query")
	}
	var functionNames []string
	functionComponents := ops.components.FilterByType(pulumiFunctionType)
	for _, v := range functionComponents {
		functionName := v.Resources["function"].Outputs["name"].StringValue()
		functionNames = append(functionNames, functionName)
	}
	logResults := ops.awsConnection.getLogsForFunctionsConcurrently(functionNames)
	sort.SliceStable(logResults, func(i, j int) bool { return logResults[i].Timestamp < logResults[j].Timestamp })
	return logResults, nil
}

func (ops *componentsOpsProvider) ListMetrics() []component.MetricName {
	return []component.MetricName{}
}

func (ops *componentsOpsProvider) GetMetricStatistics(metric component.MetricRequest) (
	[]component.MetricDataPoint, error) {
	return nil, fmt.Errorf("not yet implemented")
}

type typeid struct {
	Type tokens.Type
	ID   resource.ID
}

func makeIDLookup(source []*resource.State) map[typeid]*resource.State {
	ret := make(map[typeid]*resource.State)
	for _, state := range source {
		tid := typeid{Type: state.Type, ID: state.ID}
		ret[tid] = state
	}
	return ret
}

func lookup(m map[typeid]*resource.State, t string, id string) *resource.State {
	return m[typeid{Type: tokens.Type(t), ID: resource.ID(id)}]
}

func newPulumiFrameworkURN(resourceURN resource.URN, t tokens.Type, name tokens.QName) resource.URN {
	namespace := resourceURN.Namespace()
	return resource.NewURN(namespace, resourceURN.Alloc(), t, name)
}
