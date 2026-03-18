// Copyright 2016-2022, Pulumi Corporation.
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

import assert from "assert";
import * as semver from "semver";

import { CommandResult, LocalWorkspace, Stack } from "../../automation";
import { withTestBackend } from "./util";

describe("Stack.cancel (generated CLI integration)", () => {
    it("invokes pulumi cancel with --stack", async () => {
        let recordedArgs: string[] | undefined;
        const mockCommand = {
            command: "pulumi",
            version: semver.parse("3.200.0"),
            run: async (args: string[]): Promise<CommandResult> => {
                recordedArgs = args;
                return new CommandResult("some output", "", 0);
            },
        };

        const ws = await LocalWorkspace.create(withTestBackend({ pulumiCommand: mockCommand as any }));
        const stack = await Stack.create("cancel-test", ws);

        await stack.cancel();

        assert.deepStrictEqual(recordedArgs, ["cancel", "--yes", "--stack", stack.name]);
    });
});
