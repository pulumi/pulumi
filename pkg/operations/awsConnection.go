package operations

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/golang/glog"
)

type awsConnection struct {
	sess      *session.Session
	logSvc    *cloudwatchlogs.CloudWatchLogs
	metricSvc *cloudwatch.CloudWatch
}

func newAWSConnection(sess *session.Session) *awsConnection {
	return &awsConnection{
		sess:      sess,
		logSvc:    cloudwatchlogs.New(sess),
		metricSvc: cloudwatch.New(sess),
	}
}

func (p *awsConnection) getLogsForLogGroupsConcurrently(names []string, logGroups []string) []LogEntry {

	// Create a channel for collecting log event outputs
	ch := make(chan []*cloudwatchlogs.FilteredLogEvent)

	// Run FilterLogEvents for each log group in parallel
	for _, logGroup := range logGroups {
		go func(logGroup string) {
			resp, err := p.logSvc.FilterLogEvents(&cloudwatchlogs.FilterLogEventsInput{
				LogGroupName: aws.String(logGroup),
			})
			if err != nil {
				glog.V(5).Infof("[getLogs] Error getting logs: %v %v\n", logGroup, err)
			}
			ch <- resp.Events
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
