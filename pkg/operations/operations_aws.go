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
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// TODO[pulumi/pulumi#54] This should be factored out behind an OperationsProvider RPC interface and versioned with the
// `pulumi-aws` repo instead of statically linked into the engine.

// AWSOperationsProvider creates an OperationsProvider capable of answering operational queries based on the
// underlying resources of the `@pulumi/aws` implementation.
func AWSOperationsProvider(
	config map[config.Key]string,
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

	sess, err := getAWSSession(awsRegion, awsAccessKey, awsSecretKey, awsToken)
	if err != nil {
		return nil, err
	}

	connection := &awsConnection{
		logSvc: cloudwatchlogs.New(sess),
	}

	prov := &awsOpsProvider{
		awsConnection: connection,
		component:     component,
	}
	return prov, nil
}

type awsOpsProvider struct {
	awsConnection *awsConnection
	component     *Resource
}

var _ Provider = (*awsOpsProvider)(nil)

var (
	// AWS config keys
	regionKey = config.MustMakeKey("aws", "region")
	accessKey = config.MustMakeKey("aws", "accessKey")
	secretKey = config.MustMakeKey("aws", "secretKey")
	token     = config.MustMakeKey("aws", "token")
)

const (
	// AWS resource types
	awsFunctionType = tokens.Type("aws:lambda/function:Function")
	awsLogGroupType = tokens.Type("aws:cloudwatch/logGroup:LogGroup")
)

func (ops *awsOpsProvider) GetLogs(query LogQuery) (*[]LogEntry, error) {
	state := ops.component.State
	logging.V(6).Infof("GetLogs[%v]", state.URN)
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
		logging.V(5).Infof("GetLogs[%v] return %d logs", state.URN, len(logResult))
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
		logging.V(5).Infof("GetLogs[%v] return %d logs", state.URN, len(logResult))
		return &logResult, nil
	default:
		// Else this resource kind does not produce any logs.
		logging.V(6).Infof("GetLogs[%v] does not produce logs", state.URN)
		return nil, nil
	}
}

type awsConnection struct {
	logSvc *cloudwatchlogs.CloudWatchLogs
}

var awsDefaultSession *session.Session
var awsDefaultSessionMutex sync.Mutex

func getAWSSession(awsRegion, awsAccessKey, awsSecretKey, token string) (*session.Session, error) {
	// AWS SDK for Go documentation: "Sessions should be cached when possible"
	// We keep a default session around and then make cheap copies of it.
	awsDefaultSessionMutex.Lock()
	defer awsDefaultSessionMutex.Unlock()

	if awsDefaultSession == nil {
		sess, err := session.NewSession()
		if err != nil {
			return nil, errors.Wrap(err, "failed to create AWS session")
		}

		awsDefaultSession = sess
	}

	extraConfig := aws.NewConfig()
	extraConfig.Region = aws.String(awsRegion)

	if awsAccessKey != "" || awsSecretKey != "" || token != "" {
		extraConfig.Credentials = credentials.NewStaticCredentials(awsAccessKey, awsSecretKey, token)
	}

	return awsDefaultSession.Copy(extraConfig), nil
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
					logging.V(5).Infof("[getLogs] Getting more logs for %v...\n", logGroup)
				}
				return true
			})
			if err != nil {
				logging.V(5).Infof("[getLogs] Error getting logs: %v %v\n", logGroup, err)
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
