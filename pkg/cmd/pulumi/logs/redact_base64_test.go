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

package logs

import (
	"bytes"
	"log/slog"
	"testing"
	"time"

	pkglogging "github.com/pulumi/pulumi/pkg/v3/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"
)

func TestLogsShareRedactsBase64EncodedSecretArgs(t *testing.T) {
	t.Parallel()

	const secret = "SECRET-VALUE-do-not-leak"
	pm := resource.PropertyMap{
		"password": resource.MakeSecret(resource.NewProperty(secret)),
		"name":     resource.NewProperty("plain"),
	}

	var disk bytes.Buffer
	handler := pkglogging.NewPropertySinkHandler(slog.NewJSONHandler(&disk, &slog.HandlerOptions{Level: slog.LevelDebug}))
	rec := slog.NewRecord(time.Now(), slog.LevelInfo, "inputs=%v", 0)
	rec.AddAttrs(slog.Any("pulumi.log.arg0", pm))
	require.NoError(t, handler.Handle(t.Context(), rec))
	require.Contains(t, disk.String(), "pulumi.log.arg0")
	require.NotContains(t, disk.String(), secret,
		"the on-disk form encodes the property, so the raw secret is not present")

	var out bytes.Buffer
	require.NoError(t, formatLogRecords(&disk, &out, true))
	require.Contains(t, out.String(), "[secret]")
	require.NotContains(t, out.String(), secret)
}
