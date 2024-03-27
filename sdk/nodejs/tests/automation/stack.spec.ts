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

import assert from "assert";
import * as fs from "fs";
import * as os from "os";
import { join } from "path";

import { EngineEvent, fullyQualifiedStackName, LocalWorkspace, ProjectSettings, Stack } from "../../automation";

import { getTestOrg, getTestSuffix } from "./util";

async function writeLines(fileName: string) {
    const file = fs.createWriteStream(fileName);
    const parts = [
        `{"stdoutEvent": `,
        `{"message": "hello", "color": "blue"}`,
        `}\n`,
        `{"stdoutEvent": `,
        `{"message": "world"`,
        `, "color": "red"}}\n`,
        `{"cancelEvent": {}}\n`,
    ];
    for (let i = 0; i < parts.length; i++) {
        file.write(parts[i]);
        await new Promise((r) => setTimeout(r, 200));
    }
}

describe("Stack", () => {
    it(`correctly deals with half written lines`, async () => {
        const tmpDir = fs.mkdtempSync(join(os.tmpdir(), "temp-"));
        const tmpFilePath = join(tmpDir, "temp-file.txt");
        const projectName = "test";
        const projectSettings: ProjectSettings = {
            name: projectName,
            runtime: "nodejs",
        };
        const file = fs.openSync(tmpFilePath, "w");
        fs.closeSync(file);

        const ws = await LocalWorkspace.create({ projectSettings });

        const stackName = fullyQualifiedStackName(getTestOrg(), "test", `int_test${getTestSuffix()}`);
        const stack = await Stack.create(stackName, ws);
        let eventNo = 0;
        const readlines = stack.readLines(tmpFilePath, (event: EngineEvent) => {
            if (eventNo === 2) {
                assert(event.cancelEvent !== undefined);
                return;
            }
            const stdoutEvent = event.stdoutEvent;
            assert(stdoutEvent !== undefined);
            if (eventNo === 0) {
                assert.strictEqual(stdoutEvent.message, "hello");
                assert.strictEqual(stdoutEvent.color, "blue");
            } else if (eventNo === 1) {
                assert.strictEqual(stdoutEvent.message, "world");
                assert.strictEqual(stdoutEvent.color, "red");
            } else {
                // Should never reach here
                assert(false);
            }
            eventNo++;
        });
        await writeLines(tmpFilePath);
        await readlines;
        assert.equal(eventNo, 2);
    });
});
