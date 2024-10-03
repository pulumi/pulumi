// Copyright 2020-2024, Pulumi Corporation.
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

package constant

// ExecKindAutoLocal is a flag used to identify a command as originating
// from automation API using a traditional Pulumi project.
const ExecKindAutoLocal = "auto.local"

// ExecKindAutoInline is a flag used to identify a command as originating
// from automation API using an inline Pulumi project.
const ExecKindAutoInline = "auto.inline"

// ExecKindCLI is a flag used to identify a command as originating
// from the CLI using a traditional Pulumi project.
const ExecKindCLI = "cli"

// ExitStatusLoggedError is the exit status to indicate that a pulumi program
// has failed, but successfully logged an error message to the engine
const ExitStatusLoggedError = 32
