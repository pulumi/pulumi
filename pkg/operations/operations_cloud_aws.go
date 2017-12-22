// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package operations

import (
	"encoding/json"
	"regexp"
	"time"

	"github.com/golang/glog"

	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// TODO[pulumi/pulumi#54] This should be factored out behind an OperationsProvider RPC interface and versioned with the
// `pulumi-cloud` repo instead of statically linked into the engine.

// CloudOperationsProvider creates an OperationsProvider capable of answering operational queries based on the
// underlying resources of the `@pulumi/cloud-aws` implementation.
func CloudOperationsProvider(config map[tokens.ModuleMember]string, component *Resource) (Provider, error) {
	prov := &cloudOpsProvider{
		config:    config,
		component: component,
	}
	return prov, nil
}

type cloudOpsProvider struct {
	config    map[tokens.ModuleMember]string
	component *Resource
}

var _ Provider = (*cloudOpsProvider)(nil)

const (
	// Pulumi Framework component types
	cloudFunctionType     = tokens.Type("cloud:function:Function")
	cloudLogCollectorType = tokens.Type("cloud:logCollector:LogCollector")
	cloudServiceType      = tokens.Type("cloud:service:Service")
	cloudTaskType         = tokens.Type("cloud:task:Task")

	// AWS resource types
	awsServerlessFunctionTypeName = "aws:serverless:Function"
	awsLogGroupTypeName           = "aws:cloudwatch/logGroup:LogGroup"
)

func (ops *cloudOpsProvider) GetLogs(query LogQuery) (*[]LogEntry, error) {
	state := ops.component.State
	glog.V(6).Infof("GetLogs[%v]", state.URN)
	switch state.Type {
	case cloudFunctionType:
		// We get the aws:serverless:Function child and request it's logs, parsing out the user-visible content from
		// those logs to project into our own log output, but leaving out explicit Lambda metadata.
		name := string(state.URN.Name())
		serverlessFunction := ops.component.GetChild(awsServerlessFunctionTypeName, name)
		rawLogs, err := serverlessFunction.OperationsProvider(ops.config).GetLogs(query)
		if err != nil {
			return nil, err
		}
		contract.Assertf(rawLogs != nil, "expect aws:serverless:Function to provide logs")
		var logs []LogEntry
		for _, rawLog := range *rawLogs {
			extractedLog := extractLambdaLogMessage(rawLog.Message, name)
			if extractedLog != nil {
				logs = append(logs, *extractedLog)
			}
		}
		glog.V(5).Infof("GetLogs[%v] return %d logs", state.URN, len(logs))
		return &logs, nil
	case cloudLogCollectorType:
		// A LogCollector has an aws:serverless:Function which is wired up to receive logs from all other compute in the
		// program.  These logs are batched and then console.log'd into the log collector lambdas own logs, so we must
		// get those logs and then decode through two layers of Lambda logging to extract the original messages.  These
		// logs are delayed somewhat more than raw lambda logs, but can survive even after the source lambda is deleted.
		// In addition, we set the Lambda logs to automatically delete after 24 hours, which is safe because we have
		// centrally archived into the log collector. As a result, we will combine reading these logs with reading the
		// live Lambda logs from individual functions, de-duplicating the results, to piece together the full set of
		// logs.
		name := string(state.URN.Name())
		serverlessFunction := ops.component.GetChild(awsServerlessFunctionTypeName, name)
		rawLogs, err := serverlessFunction.OperationsProvider(ops.config).GetLogs(query)
		if err != nil {
			return nil, err
		}
		contract.Assertf(rawLogs != nil, "expect aws:serverless:Function to provide logs")
		// Extract out the encoded and batched logs
		var logs []LogEntry
		for _, rawLog := range *rawLogs {
			extractedLog := extractLambdaLogMessage(rawLog.Message, name)
			if extractedLog != nil {
				// Decode the JSON blog of data from within the log entries, which will itself be a nested log entry.
				var logMessage encodedLogMessage
				err := json.Unmarshal([]byte(extractedLog.Message), &logMessage)
				if err != nil {
					return nil, err
				}
				// Reverse engineer the name of the function that was the source of this message from the LogGroup name.
				match := functionNameFromLogGroupNameRegExp.FindStringSubmatch(logMessage.LogGroup)
				if len(match) != 2 {
					// Try older format as well
					match = oldFunctionNameFromLogGroupNameRegExp.FindStringSubmatch(logMessage.LogGroup)
				}
				if len(match) != 2 {
					glog.V(5).Infof("Skipping invalid log name found in log collector %s. " +
						"Possibly mismatched versions of pulumi and pulumi-cloud.")
					continue
				}
				logName := match[1]
				// Extract out each individual log event and add them to our array of logs.
				for _, logEvent := range logMessage.LogEvents {
					if extracted := extractLambdaLogMessage(logEvent.Message, logName); extracted != nil {
						logs = append(logs, *extracted)
					}
				}
			}
		}
		glog.V(5).Infof("GetLogs[%v] return %d logs", state.URN, len(logs))
		return &logs, nil
	case cloudServiceType, cloudTaskType:
		// Both Services and Tasks track a log group, which we can directly query for logs.  These logs are only
		// populated by user code within containers, so we can safely project these logs back unmodified.
		urn := state.URN
		name := string(urn.Name())
		logGroup := ops.component.GetChild(awsLogGroupTypeName, name)
		rawLogs, err := logGroup.OperationsProvider(ops.config).GetLogs(query)
		if err != nil {
			return nil, err
		}
		contract.Assertf(rawLogs != nil, "expect aws:cloudwatch/logGroup:LogGroup to provide logs")
		var logs []LogEntry
		for _, rawLog := range *rawLogs {
			logs = append(logs, LogEntry{
				ID:        name,
				Message:   rawLog.Message,
				Timestamp: rawLog.Timestamp,
			})
		}
		glog.V(5).Infof("GetLogs[%v] return %d logs", state.URN, len(logs))
		return &logs, nil
	default:
		// Else this resource kind does not produce any logs.
		glog.V(6).Infof("GetLogs[%v] does not produce logs", state.URN)
		return nil, nil
	}
}

type encodedLogEvent struct {
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp"`
	Message   string `json:"message"`
}

type encodedLogMessage struct {
	MessageType         string            `json:"messageType"`
	Owner               string            `json:"owner"`
	LogGroup            string            `json:"logGroup"`
	LogStream           string            `json:"logStream"`
	SubscriptionFilters []string          `json:"subscriptionFilters"`
	LogEvents           []encodedLogEvent `json:"logEvents"`
}

var (
	// Extract function name from LogGroup name
	functionNameFromLogGroupNameRegExp = regexp.MustCompile(`^/aws/lambda/(.*)-[0-9A-Fa-f]{7}$`)
	// Used prior to pulumi-terraform@1307256eeeefdd87ffd76581cd3ab73c3d7cfd4a
	oldFunctionNameFromLogGroupNameRegExp = regexp.MustCompile(`^/aws/lambda/(.*)[0-9A-Fa-f]{8}$`)
	// Extract Lambda log parts from Lambda log format
	logRegexp = regexp.MustCompile("^(.{23}Z)\t[a-g0-9\\-]{36}\t(.*)")
)

// extractLambdaLogMessage extracts out only the log messages associated with user logs, skipping Lambda-specific
// metadata.  In particular, only the second line below is extracter, and it is extracted with the recorded timestamp.
//
// ```
//  START RequestId: 25e0d1e0-cbd6-11e7-9808-c7085dfe5723 Version: $LATEST
//  2017-11-17T20:30:27.736Z	25e0d1e0-cbd6-11e7-9808-c7085dfe5723	GET /todo
//  END RequestId: 25e0d1e0-cbd6-11e7-9808-c7085dfe5723
//  REPORT RequestId: 25e0d1e0-cbd6-11e7-9808-c7085dfe5723	Duration: 222.92 ms	Billed Duration: 300 ms 	<snip>
// ```
func extractLambdaLogMessage(message string, id string) *LogEntry {
	innerMatches := logRegexp.FindAllStringSubmatch(message, -1)
	if len(innerMatches) > 0 {
		contract.Assertf(len(innerMatches[0]) >= 3, "expected log regexp to always produce at least two capture groups")
		timestamp, err := time.Parse(time.RFC3339Nano, innerMatches[0][1])
		glog.V(9).Infof("Matched Lambda log message as [%v]:'%s' from: %s", timestamp, innerMatches[0][2], message)
		contract.Assertf(err == nil, "expected to be able to parse timestamp")
		return &LogEntry{
			ID:        id,
			Message:   innerMatches[0][2],
			Timestamp: timestamp.UnixNano() / 1000000, // milliseconds
		}
	}
	glog.V(9).Infof("Could not match Lambda log message: %s", message)
	return nil
}
