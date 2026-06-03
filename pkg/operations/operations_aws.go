// Copyright 2016, Pulumi Corporation.
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
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwltypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
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
	component *Resource,
) (Provider, error) {
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
	awsProfile := config[profile]

	// If there is an explicit provider - instead use the configuration on that provider
	if component.Provider != nil {
		outputs := component.Provider.State.Outputs
		awsRegion = getPropertyMapStringValue(outputs, "region")
		awsAccessKey = getPropertyMapStringValue(outputs, "accessKey")
		awsSecretKey = getPropertyMapStringValue(outputs, "secretKey")
		awsToken = getPropertyMapStringValue(outputs, "token")
		awsProfile = getPropertyMapStringValue(outputs, "profile")
	}

	ctx := context.TODO()
	cfg, err := getAWSConfig(ctx, awsRegion, awsAccessKey, awsSecretKey, awsToken, awsProfile, true)
	if err != nil {
		return nil, err
	}

	connection := &awsConnection{
		logSvc: cloudwatchlogs.NewFromConfig(cfg),
	}

	prov := &awsOpsProvider{
		awsConnection: connection,
		component:     component,
	}
	return prov, nil
}

func getPropertyMapStringValue(m resource.PropertyMap, k resource.PropertyKey) string {
	v, ok := m[k]
	if !ok {
		return ""
	}
	if !v.IsString() {
		return ""
	}
	return v.StringValue()
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
	profile   = config.MustMakeKey("aws", "profile")
)

const (
	// AWS resource types
	awsFunctionType = tokens.Type("aws:lambda/function:Function")
	awsLogGroupType = tokens.Type("aws:cloudwatch/logGroup:LogGroup")
)

func (ops *awsOpsProvider) GetLogs(query LogQuery) (*[]LogEntry, error) {
	state := ops.component.State
	logging.V(6).Infof("GetLogs[%v]", state.URN)
	//exhaustive:ignore
	switch state.Type {
	case awsFunctionType:
		functionName := state.Outputs["name"].StringValue()
		logResult := ops.awsConnection.getLogsForLogGroupsConcurrently(
			[]string{functionName},
			[]string{"/aws/lambda/" + functionName},
			query.StartTime,
			query.EndTime,
		)
		sort.SliceStable(logResult, func(i, j int) bool {
			return logResult[i].Timestamp < logResult[j].Timestamp
		})
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
		sort.SliceStable(logResult, func(i, j int) bool {
			return logResult[i].Timestamp < logResult[j].Timestamp
		})
		logging.V(5).Infof("GetLogs[%v] return %d logs", state.URN, len(logResult))
		return &logResult, nil
	default:
		// Else this resource kind does not produce any logs.
		logging.V(6).Infof("GetLogs[%v] does not produce logs", state.URN)
		return nil, nil
	}
}

type awsConnection struct {
	logSvc *cloudwatchlogs.Client
}

var (
	awsDefaultConfigs     = map[string]aws.Config{}
	awsDefaultConfigMutex sync.Mutex
)

// getAWSConfig gets or creates an aws.Config instance to use for making AWS SDK calls using the provided credentials
// and configuration.  If `validate` is true, it also uses the credentials to make an AWS call to get the caller
// identity to ensure they are valid, and return an error if not.
func getAWSConfig(
	ctx context.Context,
	awsRegion, awsAccessKey, awsSecretKey, awsToken, awsProfile string,
	validate bool,
) (aws.Config, error) {
	// AWS SDK for Go documentation: configs should be cached when possible.
	// We keep a default config around and then make cheap copies of it.
	awsDefaultConfigMutex.Lock()
	defer awsDefaultConfigMutex.Unlock()

	key := awsRegion + awsAccessKey + awsSecretKey + awsToken + awsProfile
	cfg, ok := awsDefaultConfigs[key]
	if !ok {
		opts := []func(*awsconfig.LoadOptions) error{
			awsconfig.WithRegion(awsRegion),
		}
		if awsProfile != "" {
			opts = append(opts, awsconfig.WithSharedConfigProfile(awsProfile))
		}
		if awsAccessKey != "" || awsSecretKey != "" || awsToken != "" {
			opts = append(opts, awsconfig.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(awsAccessKey, awsSecretKey, awsToken),
			))
		}

		loaded, err := awsconfig.LoadDefaultConfig(ctx, opts...)
		if err != nil {
			return aws.Config{}, fmt.Errorf("failed to load AWS config: %w", err)
		}

		if validate {
			// Make a call to STS to ensure the credentials are valid and fail early if not.
			stsSvc := sts.NewFromConfig(loaded)
			if _, err := stsSvc.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{}); err != nil {
				return aws.Config{}, err
			}
		}

		cfg = loaded
		awsDefaultConfigs[key] = cfg
	}
	return cfg, nil
}

func (p *awsConnection) getLogsForLogGroupsConcurrently(
	names []string,
	logGroups []string,
	startTime *time.Time,
	endTime *time.Time,
) []LogEntry {
	// Create a channel for collecting log event outputs
	ch := make(chan []cwltypes.FilteredLogEvent, len(logGroups))

	var startMilli *int64
	if startTime != nil {
		startMilli = aws.Int64(startTime.UnixMilli())
	}
	var endMilli *int64
	if endTime != nil {
		endMilli = aws.Int64(endTime.UnixMilli())
	}

	// Run FilterLogEvents for each log group in parallel
	for _, logGroup := range logGroups {
		go func(logGroup string) {
			ctx := context.TODO()
			var ret []cwltypes.FilteredLogEvent
			paginator := cloudwatchlogs.NewFilterLogEventsPaginator(p.logSvc, &cloudwatchlogs.FilterLogEventsInput{
				LogGroupName: aws.String(logGroup),
				StartTime:    startMilli,
				EndTime:      endMilli,
			})
			for paginator.HasMorePages() {
				resp, err := paginator.NextPage(ctx)
				if err != nil {
					logging.V(5).Infof("[getLogs] Error getting logs: %v %v\n", logGroup, err)
					break
				}
				ret = append(ret, resp.Events...)
				if paginator.HasMorePages() {
					logging.V(5).Infof("[getLogs] Getting more logs for %v...\n", logGroup)
				}
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
				Message:   aws.ToString(event.Message),
				Timestamp: aws.ToInt64(event.Timestamp),
			})
		}
	}

	return logs
}
