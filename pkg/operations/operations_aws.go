// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package operations

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/golang/glog"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/tokens"
)

// TODO[pulumi/pulumi#54] This should be factored out behind an OperationsProvider RPC interface and versioned with the
// `pulumi-aws` repo instead of statically linked into the engine.

// AWSOperationsProvider creates an OperationsProvider capable of answering operational queries based on the
// underlying resources of the `@pulumi/aws` implementation.
func AWSOperationsProvider(
	config map[tokens.ModuleMember]string,
	component *Resource) (Provider, error) {

	awsRegion, ok := config[regionKey]
	if !ok {
		return nil, errors.New("no AWS region found")
	}

	// If provided, also pass along the access and secret keys so that we have permission to access operational data on
	// resources in the target account.
	//
	// [pulumi/pulumi#608]: We are only approximating the actual logic that the AWS provider (via
	// terraform-provdider-aws) uses to turn config into a valid AWS connection.  We should find some way to unify these
	// as part of moving this code into a separate process on the other side of an RPC boundary.
	awsAccessKey := config[accessKey]
	awsSecretKey := config[secretKey]
	awsToken := config[token]

	awsConnection, err := getAWSConnection(awsRegion, awsAccessKey, awsSecretKey, awsToken)
	if err != nil {
		return nil, err
	}

	prov := &awsOpsProvider{
		awsConnection: awsConnection,
		component:     component,
		region:        awsRegion,
	}
	return prov, nil
}

type awsOpsProvider struct {
	awsConnection *awsConnection
	component     *Resource
	region        string
}

var _ Provider = (*awsOpsProvider)(nil)

const (
	// AWS config keys
	regionKey = "aws:config:region"
	accessKey = "aws:config:accessKey"
	secretKey = "aws:config:secretKey" // nolint: gas
	token     = "aws:config:token"

	// AWS resource types
	awsFunctionType = tokens.Type("aws:lambda/function:Function")
	awsLogGroupType = tokens.Type("aws:cloudwatch/logGroup:LogGroup")
)

func (ops *awsOpsProvider) GetLogs(query LogQuery) (*[]LogEntry, error) {
	state := ops.component.State
	glog.V(6).Infof("GetLogs[%v]", state.URN)
	switch state.Type {
	case awsFunctionType:
		functionName := state.Outputs["name"].StringValue()
		logResult := ops.awsConnection.getLogsForLogGroupsConcurrently(
			[]string{functionName},
			[]string{"/aws/lambda/" + functionName},
			query.StartTime,
			query.EndTime,
		)
		sort.SliceStable(logResult, func(i, j int) bool { return logResult[i].Timestamp < logResult[j].Timestamp })
		glog.V(5).Infof("GetLogs[%v] return %d logs", state.URN, len(logResult))
		return &logResult, nil
	case awsLogGroupType:
		name := state.Outputs["name"].StringValue()
		logResult := ops.awsConnection.getLogsForLogGroupsConcurrently(
			[]string{name},
			[]string{name},
			query.StartTime,
			query.EndTime,
		)
		sort.SliceStable(logResult, func(i, j int) bool { return logResult[i].Timestamp < logResult[j].Timestamp })
		glog.V(5).Infof("GetLogs[%v] return %d logs", state.URN, len(logResult))
		return &logResult, nil
	default:
		// Else this resource kind does not produce any logs.
		glog.V(6).Infof("GetLogs[%v] does not produce logs", state.URN)
		return nil, nil
	}
}

func (ops *awsOpsProvider) GetResourceData() (map[string]string, error) {
	result := make(map[string]string)
	consoleLink := ops.getConsoleLink(ops.component.State)
	if len(consoleLink) > 0 {
		result["consoleLink"] = consoleLink
	}

	return result, nil
}

type awsConnection struct {
	sess   *session.Session
	logSvc *cloudwatchlogs.CloudWatchLogs
}

var awsConnectionCache = map[string]*awsConnection{}
var awsConnectionCacheMutex = sync.RWMutex{}

func getAWSConnection(awsRegion, awsAccessKey, awsSecretKey, token string) (*awsConnection, error) {
	awsConnectionCacheMutex.RLock()
	connection, ok := awsConnectionCache[awsRegion]
	awsConnectionCacheMutex.RUnlock()
	if !ok {
		awsConfig := aws.NewConfig()
		awsConfig.Region = aws.String(awsRegion)
		if awsAccessKey != "" || awsSecretKey != "" {
			awsConfig.Credentials = credentials.NewStaticCredentials(awsAccessKey, awsSecretKey, token)
			glog.V(5).Infof("Using credentials from stack config for AWS operations provider.")
		} else {
			glog.V(5).Infof("Using ambient credentials for AWS operations provider.")
		}
		sess, err := session.NewSession(awsConfig)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create AWS session")
		}
		connection = &awsConnection{
			sess:   sess,
			logSvc: cloudwatchlogs.New(sess),
		}
		awsConnectionCacheMutex.Lock()
		awsConnectionCache[awsRegion] = connection
		awsConnectionCacheMutex.Unlock()
	}
	return connection, nil
}

func (p *awsConnection) getLogsForLogGroupsConcurrently(
	names []string,
	logGroups []string,
	startTime *time.Time,
	endTime *time.Time) []LogEntry {

	// Create a channel for collecting log event outputs
	ch := make(chan []*cloudwatchlogs.FilteredLogEvent, len(logGroups))

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

func (ops *awsOpsProvider) getConsoleLink(state *resource.State) string {
	switch state.Type {
	case "aws:apigateway/deployment:Deployment":
		return fmt.Sprintf("https://%s.console.aws.amazon.com/apigateway/home?region=%s#/apis/%s",
			ops.region, ops.region, state.Inputs["restApi"].StringValue())
	case "aws:apigateway/restApi:RestApi":
		return fmt.Sprintf("https://%s.console.aws.amazon.com/apigateway/home?region=%s#/apis/%s",
			ops.region, ops.region, state.ID)
	case "aws:apigateway/stage:Stage":
		return ""
	case "aws:cloudwatch/eventRule:EventRule":
		fallthrough
	case "aws:cloudwatch/eventTarget:EventTarget":
		return fmt.Sprintf("https://%s.console.aws.amazon.com/cloudwatch/home?region=%s#rules:name=%s",
			ops.region, ops.region, state.ID)
	case "aws:cloudwatch/logGroup:LogGroup":
		return fmt.Sprintf("https://%s.console.aws.amazon.com/cloudwatch/home?region=%s#logStream:group=%s",
			ops.region, ops.region, state.ID)
	case "aws:cloudwatch/logSubscriptionFilter:LogSubscriptionFilter":
		return fmt.Sprintf("https://%s.console.aws.amazon.com/cloudwatch/home?region=%s#logStream:group=%s",
			ops.region, ops.region, state.ID)
	case "aws:dynamodb/table:Table":
		return fmt.Sprintf("https://%s.console.aws.amazon.com/dynamodb/home?region=%s#tables:selected=%s",
			ops.region, ops.region, state.ID)
	case "aws:iam/rolePolicyAttachment:RolePolicyAttachment":
		return fmt.Sprintf("https://%s.console.aws.amazon.com/iam/home?region=%s#/policies/%s",
			ops.region, ops.region, state.Inputs["policyArn"].StringValue())
	case "aws:iam/role:Role":
		return fmt.Sprintf("https://%s.console.aws.amazon.com/iam/home?region=%s#/roles/%s",
			ops.region, ops.region, state.ID)
	case "aws:lambda/function:Function":
		return fmt.Sprintf("https://%s.console.aws.amazon.com/lambda/home?region=%s#/functions/%s",
			ops.region, ops.region, state.ID)
	case "aws:lambda/permission:Permission":
		return ""
	case "aws:s3/bucket:Bucket":
		return fmt.Sprintf("https://s3.console.aws.amazon.com/s3/buckets/%s/?region=%s",
			state.ID, ops.region)
	case "aws:s3/bucketObject:BucketObject":
		return fmt.Sprintf("https://s3.console.aws.amazon.com/s3/buckets/%s/%s/?region=%s",
			state.Inputs["bucket"].StringValue(), state.ID, ops.region)
	case "aws:serverless:Function":
		return ""
	case "aws:sns/topicSubscription:TopicSubscription":
		// TODO: No direct link
		return fmt.Sprintf("https://%s.console.aws.amazon.com/sns/v2/home?region=%s#/subscriptions",
			ops.region, ops.region)
	case "aws:sns/topic:Topic":
		return fmt.Sprintf("https://%s.console.aws.amazon.com/sns/v2/home?region=%s#/topics/%s",
			ops.region, ops.region, state.ID)
	default:
		return ""
	}
}
