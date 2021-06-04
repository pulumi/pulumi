// Copyright 2016-2021, Pulumi Corporation.
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
package auto

import (
	"encoding/json"
	"github.com/nxadm/tail"

	"github.com/pulumi/pulumi/sdk/v3/go/auto/events"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

func watchFile(path string, receivers []chan<- events.EngineEvent) (*tail.Tail, error) {
	t, err := tail.TailFile(path, tail.Config{
		Follow: true,
		Logger: tail.DiscardingLogger,
	})
	if err != nil {
		return nil, err
	}
	go func(tailedLog *tail.Tail) {
		for line := range tailedLog.Lines {
			if line.Err != nil {
				for _, r := range receivers {
					r <- events.EngineEvent{Error: line.Err}
				}
				continue
			}
			var e apitype.EngineEvent
			err = json.Unmarshal([]byte(line.Text), &e)
			if err != nil {
				for _, r := range receivers {
					r <- events.EngineEvent{Error: err}
				}
				continue
			}
			for _, r := range receivers {
				r <- events.EngineEvent{EngineEvent: e}
			}
		}
	}(t)
	return t, nil
}
