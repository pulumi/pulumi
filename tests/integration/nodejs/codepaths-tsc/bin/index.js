// Copyright 2024-2024, Pulumi Corporation.
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
// Dummy file to keep ProgramTest happy. We need ProjectInfo.main to exist already.
// This will be overwritten by the compiled main file in the test after running
// `yarn run build`.

throw new Error(
  `This file should not be required in tests. It should be overwritten by the tsc output during testing.`
);
