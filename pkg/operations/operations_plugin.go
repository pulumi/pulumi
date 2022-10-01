package operations

import (
	"time"

	logs "go.opentelemetry.io/proto/otlp/logs/v1"
	metrics "go.opentelemetry.io/proto/otlp/metrics/v1"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

type pluginOpsProvider struct {
	provider plugin.Provider
	resource *resource.State
}

func (pp *pluginOpsProvider) GetLogs(query LogQuery) ([]*logs.ResourceLogs, interface{}, error) {
	continuationToken, _ := query.ContinuationToken.(string)
	options := plugin.GetResourceLogsOptions{
		Count:             query.Count,
		ContinuationToken: continuationToken,
	}
	options.EndTime = time.Now()
	if query.EndTime != nil {
		options.EndTime = *query.EndTime
	}
	options.StartTime = options.EndTime.Add(-1 * time.Hour)
	if query.StartTime != nil {
		options.StartTime = *query.StartTime
	}

	logs, continuationToken, err := pp.provider.GetResourceLogs(pp.resource.URN, pp.resource.ID, pp.resource.Outputs, options)
	if err != nil {
		return nil, nil, err
	}

	var nextToken interface{}
	if continuationToken != "" {
		nextToken = continuationToken
	}
	return logs, nextToken, nil
}

func (pp *pluginOpsProvider) GetMetrics(query MetricsQuery) ([]*metrics.ResourceMetrics, interface{}, error) {
	continuationToken, _ := query.ContinuationToken.(string)
	options := plugin.GetResourceMetricsOptions{
		Count:             query.Count,
		ContinuationToken: continuationToken,
	}
	options.EndTime = time.Now()
	if query.EndTime != nil {
		options.EndTime = *query.EndTime
	}
	options.StartTime = options.EndTime.Add(-1 * time.Hour)
	if query.StartTime != nil {
		options.StartTime = *query.StartTime
	}

	metrics, continuationToken, err := pp.provider.GetResourceMetrics(pp.resource.URN, pp.resource.ID, pp.resource.Outputs, options)
	if err != nil {
		return nil, nil, err
	}

	var nextToken interface{}
	if continuationToken != "" {
		nextToken = continuationToken
	}
	return metrics, nextToken, nil
}
