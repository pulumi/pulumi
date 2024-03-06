// Copyright 2016-2024, Pulumi Corporation.
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
//
// If this program runs successfully, then the test passes.
// Executing this file demonstrates that we're able to successfully install
// dependencies and run in a npm workspaces setup. 

import * as myRandom from "my-random";

const random = new myRandom.MyRandom("plop", {})

export const id = random.randomID
