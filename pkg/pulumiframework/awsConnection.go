package pulumiframework

import (
	"regexp"

	"github.com/aws/aws-sdk-go/service/cloudwatch"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
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

func (p *awsConnection) getLogsForFunctionsConcurrently(functions []string) []component.LogEntry {
	var logs []component.LogEntry
	ch := make(chan []component.LogEntry)
	for _, functionName := range functions {
		go func(functionName string) {
			ch <- p.getLogsForFunction(functionName)
		}(functionName)
	}
	for i := 0; i < len(functions); i++ {
		logs = append(logs, <-ch...)
	}
	return logs
}

func (p *awsConnection) getLogsForFunction(functionName string) []component.LogEntry {
	logGroupName := "/aws/lambda/" + functionName
	resp, err := p.logSvc.DescribeLogStreams(&cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName: aws.String(logGroupName),
	})
	if err != nil {
		glog.V(5).Infof("[getLogs] Error getting logs: %v %v\n", logGroupName, err)
		return nil
	}
	glog.V(5).Infof("[getLogs] Log streams: %v\n", resp)
	logResult := p.getLogsForFunctionNameStreamsConcurrently(functionName, resp.LogStreams)
	return logResult
}

func (p *awsConnection) getLogsForFunctionNameStreamsConcurrently(functionName string,
	logStreams []*cloudwatchlogs.LogStream) []component.LogEntry {
	var logs []component.LogEntry
	ch := make(chan []component.LogEntry)
	for _, logStream := range logStreams {
		go func(logStreamName *string) {
			ch <- p.getLogsForFunctionNameStream(functionName, logStreamName)
		}(logStream.LogStreamName)
	}
	for i := 0; i < len(logStreams); i++ {
		logs = append(logs, <-ch...)
	}
	return logs
}

func (p *awsConnection) getLogsForFunctionNameStream(functionName string, logStreamName *string) []component.LogEntry {
	var logResult []component.LogEntry
	logGroupName := "/aws/lambda/" + functionName
	logsResp, err := p.logSvc.GetLogEvents(&cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  aws.String(logGroupName),
		LogStreamName: logStreamName,
		StartFromHead: aws.Bool(true),
	})
	if err != nil {
		glog.V(5).Infof("[getLogs] Error getting logs: %v %v\n", logStreamName, err)
	}
	glog.V(5).Infof("[getLogs] Log events: %v\n", logsResp)
	for _, event := range logsResp.Events {
		innerMatches := logRegexp.FindAllStringSubmatch(aws.StringValue(event.Message), -1)
		glog.V(5).Infof("[getLogs] Inner matches: %v\n", innerMatches)
		if len(innerMatches) > 0 {
			logResult = append(logResult, component.LogEntry{
				ID:        functionName,
				Message:   innerMatches[0][1],
				Timestamp: aws.Int64Value(event.Timestamp),
			})
		}
	}
	return logResult
}
