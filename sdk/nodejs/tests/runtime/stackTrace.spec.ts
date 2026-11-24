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

import * as assert from "assert";
import * as path from "path";
import * as url from "url";
import { getStackTrace } from "../../runtime/stackTrace";

describe("getStackTrace", () => {
    it("omits the capture helpers and skips additional caller frames", () => {
        function capture(skip: number) {
            return getStackTrace(skip);
        }
        function middle(skip: number) {
            return capture(skip);
        }
        function outer(skip: number) {
            return middle(skip);
        }

        const noSkip = outer(0);
        const skipOne = outer(1);
        const skipTwo = outer(2);

        assert.ok(noSkip.length >= 3);
        assert.ok(noSkip[0]?.uri.endsWith("/stackTrace.spec.ts"));
        assert.ok(noSkip[1]?.uri.endsWith("/stackTrace.spec.ts"));
        assert.ok(noSkip[2]?.uri.endsWith("/stackTrace.spec.ts"));
        assert.deepStrictEqual(skipOne.slice(0, 2), noSkip.slice(1, 3));
        assert.deepStrictEqual(skipTwo[0], noSkip[2]);
    });

    it("returns an empty trace when skipping beyond the stack", () => {
        assert.deepStrictEqual(getStackTrace(1000), []);
    });

    it("captures anonymous paths and file URLs", () => {
        const filename = path.join(__dirname, "program.ts");
        const uri = url.pathToFileURL(filename).href;
        const prepare = Error.prepareStackTrace;
        try {
            Error.prepareStackTrace = () =>
                [
                    "Error",
                    `    at callsites (${filename}:1:1)`,
                    `    at getStackTrace (${filename}:2:1)`,
                    `    at ${filename}:10:2`,
                    `    at run (${uri}:20:3)`,
                ].join("\n");
            assert.deepStrictEqual(getStackTrace(0), [
                { uri, line: 10, column: 2 },
                { uri, line: 20, column: 3 },
            ]);
        } finally {
            Error.prepareStackTrace = prepare;
        }
    });
});
