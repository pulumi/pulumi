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
	"fmt"
	"io"

	"github.com/nxadm/tail"

	"github.com/pulumi/pulumi/sdk/v2/go/common/apitype"
)

func watchFile(path string, streams []io.Writer, events chan<- apitype.EngineEvent) (*tail.Tail, error) {
	t, err := tail.TailFile(path, tail.Config{
		Follow: true,
		Logger: tail.DiscardingLogger,
	})
	if err != nil {
		return nil, err
	}
	go func(tailedLog *tail.Tail) {
		for line := range tailedLog.Lines {
			for _, s := range streams {
				_, err = io.WriteString(s, fmt.Sprintf("%s\n", line.Text))
			}
			var e apitype.EngineEvent
			err = json.Unmarshal([]byte(line.Text), &e)
			events <- e
		}
	}(t)
	return t, nil
}
