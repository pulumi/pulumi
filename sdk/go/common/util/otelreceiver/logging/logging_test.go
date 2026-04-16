// Copyright 2026, Pulumi Corporation.
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

package logging

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
)

type mockLogExporter struct {
	mu   sync.Mutex
	logs []plog.Logs
}

func (m *mockLogExporter) ExportLogs(_ context.Context, logs plog.Logs) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, logs)
	return nil
}

func (m *mockLogExporter) Shutdown(context.Context) error { return nil }

func (m *mockLogExporter) firstRecord() plog.LogRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.logs[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
}

func TestExportForwardsToExporter(t *testing.T) {
	t.Parallel()

	exporter := &mockLogExporter{}
	svc := &service{exporter: exporter}
	now := uint64(time.Now().UnixNano()) //nolint:gosec // test timestamps

	req := &collogspb.ExportLogsServiceRequest{
		ResourceLogs: []*logspb.ResourceLogs{{
			Resource: &resourcepb.Resource{
				Attributes: []*commonpb.KeyValue{{
					Key: "service.name",
					Value: &commonpb.AnyValue{
						Value: &commonpb.AnyValue_StringValue{StringValue: "test-svc"},
					},
				}},
			},
			ScopeLogs: []*logspb.ScopeLogs{{
				LogRecords: []*logspb.LogRecord{{
					TimeUnixNano:   now,
					SeverityNumber: logspb.SeverityNumber_SEVERITY_NUMBER_INFO,
					Body: &commonpb.AnyValue{
						Value: &commonpb.AnyValue_StringValue{StringValue: "hello"},
					},
					Attributes: []*commonpb.KeyValue{{
						Key:   "key",
						Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "value"}},
					}},
				}},
			}},
		}},
	}

	resp, err := svc.Export(t.Context(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	require.Len(t, exporter.logs, 1)
	lr := exporter.firstRecord()
	assert.Equal(t, "hello", lr.Body().AsString())
	v, ok := lr.Attributes().Get("key")
	require.True(t, ok)
	assert.Equal(t, "value", v.Str())
}

func TestExportDecodesPropertyValues(t *testing.T) {
	t.Parallel()

	pv := resource.NewProperty(resource.PropertyMap{
		"name": resource.NewProperty("my-bucket"),
		"password": resource.NewProperty(&resource.Secret{
			Element: resource.NewProperty("hunter2"),
		}),
	})

	encoded, err := plugin.EncodePropertyValueForLog(pv)
	require.NoError(t, err)

	exporter := &mockLogExporter{}
	svc := &service{exporter: exporter}
	now := uint64(time.Now().UnixNano()) //nolint:gosec // test timestamps

	req := &collogspb.ExportLogsServiceRequest{
		ResourceLogs: []*logspb.ResourceLogs{{
			ScopeLogs: []*logspb.ScopeLogs{{
				LogRecords: []*logspb.LogRecord{{
					TimeUnixNano:   now,
					SeverityNumber: logspb.SeverityNumber_SEVERITY_NUMBER_DEBUG,
					Body: &commonpb.AnyValue{
						Value: &commonpb.AnyValue_StringValue{StringValue: "resource inputs"},
					},
					Attributes: []*commonpb.KeyValue{{
						Key:   "inputs",
						Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_BytesValue{BytesValue: encoded}},
					}},
				}},
			}},
		}},
	}

	resp, err := svc.Export(t.Context(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	lr := exporter.firstRecord()
	v, ok := lr.Attributes().Get("inputs")
	require.True(t, ok)
	// Property value bytes are decoded to a JSON string of the
	// resource.PropertyValue's Mappable() representation.
	jsonStr := v.Str()
	assert.Contains(t, jsonStr, "my-bucket")
	assert.Contains(t, jsonStr, "hunter2")
}

func TestExportNilExporterDoesNotPanic(t *testing.T) {
	t.Parallel()

	svc := &service{exporter: nil}
	now := uint64(time.Now().UnixNano()) //nolint:gosec // test timestamps

	req := &collogspb.ExportLogsServiceRequest{
		ResourceLogs: []*logspb.ResourceLogs{{
			ScopeLogs: []*logspb.ScopeLogs{{
				LogRecords: []*logspb.LogRecord{{
					TimeUnixNano:   now,
					SeverityNumber: logspb.SeverityNumber_SEVERITY_NUMBER_INFO,
					Body: &commonpb.AnyValue{
						Value: &commonpb.AnyValue_StringValue{StringValue: "test"},
					},
				}},
			}},
		}},
	}

	resp, err := svc.Export(t.Context(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
}
