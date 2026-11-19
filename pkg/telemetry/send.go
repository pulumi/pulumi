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

package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const requestTimeout = 10 * time.Second

// sendFunc delivers a batch of events. An error means the batch wasn't delivered and should be retried later.
type sendFunc func(ctx context.Context, events []json.RawMessage) error

// httpSender sends batches to the telemetry endpoint of the Pulumi Cloud API at url. A batch the service rejects
// counts as delivered, since sending it again would only be rejected again.
func httpSender(url, userAgent string) sendFunc {
	client := &http.Client{Timeout: requestTimeout}
	endpoint := strings.TrimSuffix(url, "/") + "/api/telemetry/v1"
	return func(ctx context.Context, events []json.RawMessage) error {
		body, err := json.Marshal(struct {
			Events []json.RawMessage `json:"events"`
		}{events})
		if err != nil {
			return err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", userAgent)

		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)

		switch {
		case resp.StatusCode < 300:
			return nil
		case resp.StatusCode < 500 && resp.StatusCode != http.StatusRequestTimeout &&
			resp.StatusCode != http.StatusTooManyRequests:
			slog.DebugContext(ctx, "telemetry rejected", "status", resp.StatusCode)
			return nil
		default:
			return fmt.Errorf("telemetry endpoint returned %s", resp.Status)
		}
	}
}
