package pulumiframework

import (
	"regexp"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/golang/glog"

	"github.com/pulumi/pulumi/pkg/component"
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

var logRegexp = regexp.MustCompile(".*Z\t[a-g0-9\\-]*\t(.*)")

func (p *awsConnection) getLogsForLogGroupsConcurrently(names []string, logGroups []string) []component.LogEntry {

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
	var logs []component.LogEntry
	for i := 0; i < len(logGroups); i++ {
		logEvents := <-ch
		if logEvents != nil {
			for _, event := range logEvents {
				innerMatches := logRegexp.FindAllStringSubmatch(aws.StringValue(event.Message), -1)
				glog.V(5).Infof("[getLogs] Inner matches: %v\n", innerMatches)
				if len(innerMatches) > 0 {
					logs = append(logs, component.LogEntry{
						ID:        names[i],
						Message:   innerMatches[0][1],
						Timestamp: aws.Int64Value(event.Timestamp),
					})
				}
			}
		}
	}

	return logs
}
