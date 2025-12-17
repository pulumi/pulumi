// Copyright 2025, Pulumi Corporation.
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

// Package pluginstorage will be the definitive source for how plugins are stored and
// managed on disk.
//
// Right now, this is pending a refactor to move methods like [(workspace.PluginSpec).Dir]
// and all functions that deal with <name>.lock & <name>.partial files to this package.
package pluginstorage
