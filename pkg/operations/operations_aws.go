package operations

import (
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/golang/glog"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// AWSOperationsProvider creates an OperationsProvider capable of answering operational queries based on the
// underlying resources of the `@pulumi/cloud-aws` implementation.
func AWSOperationsProvider(
	config map[tokens.ModuleMember]string,
	component *Resource) (Provider, error) {

	awsRegion, ok := config[regionKey]
	if !ok {
		return nil, errors.New("no AWS region found")
	}

	awsConnection, err := getAWSConnection(awsRegion)
	if err != nil {
		return nil, err
	}

	prov := &awsOpsProvider{
		awsConnection: awsConnection,
		component:     component,
	}
	return prov, nil
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
	awsLogGroupType = tokens.Type("aws:cloudwatch/logGroup:LogGroup")
)

func (ops *awsOpsProvider) GetLogs(query LogQuery) (*[]LogEntry, error) {
	if query.Query != nil {
		contract.Failf("not yet implemented - Query")
	}
	switch ops.component.state.Type {
	case awsFunctionType:
		functionName := ops.component.state.Outputs["name"].StringValue()
		logResult := ops.awsConnection.getLogsForLogGroupsConcurrently(
			[]string{functionName},
			[]string{"/aws/lambda/" + functionName},
			query.StartTime,
			query.EndTime,
		)
		sort.SliceStable(logResult, func(i, j int) bool { return logResult[i].Timestamp < logResult[j].Timestamp })
		return &logResult, nil
	case awsLogGroupType:
		name := ops.component.state.Outputs["name"].StringValue()
		logResult := ops.awsConnection.getLogsForLogGroupsConcurrently(
			[]string{name},
			[]string{name},
			query.StartTime,
			query.EndTime,
		)
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

type awsConnection struct {
	sess      *session.Session
	logSvc    *cloudwatchlogs.CloudWatchLogs
	metricSvc *cloudwatch.CloudWatch
}

var awsConnectionCache = map[string]*awsConnection{}

func getAWSConnection(awsRegion string) (*awsConnection, error) {
	connection, ok := awsConnectionCache[awsRegion]
	if !ok {
		awsConfig := aws.NewConfig()
		awsConfig.Region = aws.String(awsRegion)
		sess, err := session.NewSession(awsConfig)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create AWS session")
		}
		connection = &awsConnection{
			sess:      sess,
			logSvc:    cloudwatchlogs.New(sess),
			metricSvc: cloudwatch.New(sess),
		}
		awsConnectionCache[awsRegion] = connection
	}
	return connection, nil
}

func (p *awsConnection) getLogsForLogGroupsConcurrently(
	names []string,
	logGroups []string,
	startTime *time.Time,
	endTime *time.Time) []LogEntry {

	// Create a channel for collecting log event outputs
	ch := make(chan []*cloudwatchlogs.FilteredLogEvent)

	var startMilli *int64
	if startTime != nil {
		startMilli = aws.Int64(aws.TimeUnixMilli(*startTime))
	}
	var endMilli *int64
	if endTime != nil {
		endMilli = aws.Int64(aws.TimeUnixMilli(*endTime))
	}

	// Run FilterLogEvents for each log group in parallel
	for _, logGroup := range logGroups {
		go func(logGroup string) {
			var ret []*cloudwatchlogs.FilteredLogEvent
			err := p.logSvc.FilterLogEventsPages(&cloudwatchlogs.FilterLogEventsInput{
				LogGroupName: aws.String(logGroup),
				StartTime:    startMilli,
				EndTime:      endMilli,
			}, func(resp *cloudwatchlogs.FilterLogEventsOutput, lastPage bool) bool {
				ret = append(ret, resp.Events...)
				if !lastPage {
					fmt.Printf("Getting more logs for %v...\n", logGroup)
				}
				return true
			})
			if err != nil {
				glog.V(5).Infof("[getLogs] Error getting logs: %v %v\n", logGroup, err)
			}
			ch <- ret
		}(logGroup)
	}

	// Collect responses on the channel and append logs into combined log array
	var logs []LogEntry
	for i := 0; i < len(logGroups); i++ {
		logEvents := <-ch
		for _, event := range logEvents {
			logs = append(logs, LogEntry{
				ID:        names[i],
				Message:   aws.StringValue(event.Message),
				Timestamp: aws.Int64Value(event.Timestamp),
			})
		}
	}

	return logs
}
