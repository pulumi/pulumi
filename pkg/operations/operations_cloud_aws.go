package operations

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

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
	pulumiFunctionType = tokens.Type("cloud:function:Function")
	logCollectorType   = tokens.Type("cloud:logCollector:LogCollector")

	// AWS resource types
	serverlessFunctionType = "aws:serverless:Function"
)

func (ops *cloudOpsProvider) GetLogs(query LogQuery) (*[]LogEntry, error) {
	if query.StartTime != nil || query.EndTime != nil || query.Query != nil {
		contract.Failf("not yet implemented - StartTime, Endtime, Query")
	}
	switch ops.component.state.Type {
	case pulumiFunctionType:
		urn := ops.component.state.URN
		serverlessFunction := ops.component.GetChild(serverlessFunctionType, string(urn.Name()))
		rawLogs, err := serverlessFunction.OperationsProvider(ops.config).GetLogs(query)
		if err != nil {
			return nil, err
		}
		contract.Assertf(rawLogs != nil, "expect aws:serverless:Function to provide logs")
		var logs []LogEntry
		for _, rawLog := range *rawLogs {
			extractedLog := extractLambdaLogMessage(rawLog.Message)
			if extractedLog != nil {
				logs = append(logs, *extractedLog)
			}
		}
		return &logs, nil
	case logCollectorType:
		urn := ops.component.state.URN
		serverlessFunction := ops.component.GetChild(serverlessFunctionType, string(urn.Name()))
		rawLogs, err := serverlessFunction.OperationsProvider(ops.config).GetLogs(query)
		if err != nil {
			return nil, err
		}
		contract.Assertf(rawLogs != nil, "expect aws:serverless:Function to provide logs")
		// Extract out the encoded and batched logs
		var logs []LogEntry
		for _, rawLog := range *rawLogs {
			var logMessage encodedLogMessage
			extractedLog := extractLambdaLogMessage(rawLog.Message)
			if extractedLog != nil {
				err := json.Unmarshal([]byte(extractedLog.Message), &logMessage)
				if err != nil {
					return nil, err
				}
				for _, logEvent := range logMessage.LogEvents {
					if extracted := extractLambdaLogMessage(logEvent.Message); extracted != nil {
						logs = append(logs, *extracted)
					}
				}
			}
		}
		return &logs, nil
	default:
		// Else this resource kind does not produce any logs.
		return nil, nil
	}
}

func (ops *cloudOpsProvider) ListMetrics() []MetricName {
	return nil
}

func (ops *cloudOpsProvider) GetMetricStatistics(metric MetricRequest) ([]MetricDataPoint, error) {
	return nil, fmt.Errorf("Not yet implmeneted: GetMetricStatistics")
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

var logRegexp = regexp.MustCompile("(.*Z)\t[a-g0-9\\-]*\t(.*)")

// extractLambdaLogMessage extracts out only the log messages associated with user logs, skipping Lambda-specific metadata.
// In particular, only the second line below is extracter, and it is extracted with the recorded timestamp.
//
// ```
//  START RequestId: 25e0d1e0-cbd6-11e7-9808-c7085dfe5723 Version: $LATEST
//  2017-11-17T20:30:27.736Z	25e0d1e0-cbd6-11e7-9808-c7085dfe5723	GET /todo
//  END RequestId: 25e0d1e0-cbd6-11e7-9808-c7085dfe5723
//  REPORT RequestId: 25e0d1e0-cbd6-11e7-9808-c7085dfe5723	Duration: 222.92 ms	Billed Duration: 300 ms 	Memory Size: 128 MB	Max Memory Used: 33 MB
// ```
func extractLambdaLogMessage(message string) *LogEntry {
	innerMatches := logRegexp.FindAllStringSubmatch(message, -1)
	if len(innerMatches) > 0 {
		contract.Assertf(len(innerMatches[0]) >= 3, "expected log regexp to always produce at least two capture groups")
		timestamp, err := time.Parse(time.RFC3339Nano, innerMatches[0][1])
		contract.Assertf(err == nil, "expected to be able to parse timestamp")
		return &LogEntry{
			ID:        "hmm",
			Message:   innerMatches[0][2],
			Timestamp: timestamp.UnixNano() / 1000000, // milliseconds
		}
	}
	return nil
}
