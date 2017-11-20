package operations

import (
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// AWSOperationsProvider creates an OperationsProvider capable of answering operational queries based on the
// underlying resources of the `@pulumi/cloud-aws` implementation.
func AWSOperationsProvider(
	config map[tokens.ModuleMember]string,
	component *Resource) (Provider, error) {

	sess, err := createSessionFromConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create AWS session")
	}

	prov := &awsOpsProvider{
		awsConnection: newAWSConnection(sess),
		component:     component,
	}
	return prov, nil
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

type awsOpsProvider struct {
	awsConnection *awsConnection
	component     *Resource
}

var _ Provider = (*awsOpsProvider)(nil)

const (
	// AWS config keys
	regionKey = "aws:config:region"

	// AWS resource types
	awsFunctionType = tokens.Type("aws:lambda/function:Function")
)

func (ops *awsOpsProvider) GetLogs(query LogQuery) (*[]LogEntry, error) {
	if query.StartTime != nil || query.EndTime != nil || query.Query != nil {
		contract.Failf("not yet implemented - StartTime, Endtime, Query")
	}
	switch ops.component.state.Type {
	case awsFunctionType:
		functionName := ops.component.state.Outputs["name"].StringValue()
		logResult := ops.awsConnection.getLogsForLogGroupsConcurrently([]string{functionName}, []string{"/aws/lambda/" + functionName})
		sort.SliceStable(logResult, func(i, j int) bool { return logResult[i].Timestamp < logResult[j].Timestamp })
		return &logResult, nil
	default:
		// Else this resource kind does not produce any logs.
		return nil, nil
	}
}

func (ops *awsOpsProvider) ListMetrics() []MetricName {
	return nil
}

func (ops *awsOpsProvider) GetMetricStatistics(metric MetricRequest) ([]MetricDataPoint, error) {
	return nil, fmt.Errorf("Not yet implmeneted: GetMetricStatistics")
}
