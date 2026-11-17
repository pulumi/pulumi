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
	"log/slog"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// countingHandler is an slog.Handler that is enabled at every level and counts how many records it receives.
type countingHandler struct{ count *atomic.Int64 }

func (h countingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h countingHandler) Handle(context.Context, slog.Record) error {
	h.count.Add(1)
	return nil
}

func (h countingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h countingHandler) WithGroup(string) slog.Handler      { return h }

// Regression test for https://github.com/pulumi/pulumi/issues/23772
//
//nolint:paralleltest // sets global logging sink
func TestPropertySinkHandlerNoRecursion(t *testing.T) {
	var count atomic.Int64
	logging.SetSinkHandler(NewPropertySinkHandler(countingHandler{count: &count}))
	t.Cleanup(func() { logging.SetSinkHandler(nil) })

	m := resource.PropertyMap{
		"scalar": resource.NewProperty("s"),
		"nested": resource.NewProperty(resource.PropertyMap{
			"a": resource.NewProperty(42.0),
			"deep": resource.NewProperty(resource.PropertyMap{
				"b": resource.NewProperty(true),
			}),
		}),
	}

	_, err := plugin.MarshalProperties(m, plugin.MarshalOptions{})
	require.NoError(t, err)

	require.Equal(t, int64(5), count.Load(), "sink handler should log exactly one record per property")
}
