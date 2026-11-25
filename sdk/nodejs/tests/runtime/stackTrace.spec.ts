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
import * as fs from "fs";
import * as os from "os";
import * as path from "path";
import * as url from "url";
import { getStackTrace, marshalSourcePosition, marshalStackTrace } from "../../runtime/stackTrace";

function syntheticStack(frames: string[]) {
    const prepare = Error.prepareStackTrace;
    try {
        Error.prepareStackTrace = () =>
            ["Error", "    at callsites (/capture.js:1:1)", "    at getStackTrace (/capture.js:2:1)", ...frames].join(
                "\n",
            );
        return marshalStackTrace(getStackTrace(0))!.getFramesList();
    } finally {
        Error.prepareStackTrace = prepare;
    }
}

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

    it("does not use Node's built-in frames as registration positions", () => {
        assert.strictEqual(
            marshalSourcePosition({ uri: "node:internal/modules/cjs/loader", line: 1, column: 1 }),
            undefined,
        );
    });

    it("retains package identity and classifies SDK, loader, and runtime frames", () => {
        const root = fs.mkdtempSync(path.join(os.tmpdir(), "pulumi-stack-trace-"));
        try {
            const sdk = path.join(root, "node_modules", "@pulumi", "pulumi");
            const aws = path.join(root, "node_modules", "@pulumi", "aws");
            const tsNode = path.join(root, "node_modules", "ts-node");
            for (const [directory, name] of [
                [root, "example-app"],
                [sdk, "@pulumi/pulumi"],
                [aws, "@pulumi/aws"],
                [tsNode, "ts-node"],
            ]) {
                fs.mkdirSync(directory, { recursive: true });
                fs.writeFileSync(path.join(directory, "package.json"), JSON.stringify({ name }));
            }
            const caller = path.join(root, "index.ts");
            const frames = syntheticStack([
                `    at new Resource (${path.join(sdk, "resource.ts")}:10:2)`,
                `    at new Bucket (${path.join(aws, "s3", "bucket.ts")}:20:3)`,
                `    at ${url.pathToFileURL(caller)}:30:4`,
                `    at compile (${path.join(tsNode, "src", "index.ts")}:1618:23)`,
                "    at processTicksAndRejections (node:internal/process/task_queues:105:5)",
                "    at nativeCall (native)",
            ]);
            assert.strictEqual(frames.length, 6);
            assert.strictEqual(frames[0].getModule(), "@pulumi/pulumi");
            assert.strictEqual(frames[0].getOrigin(), "pulumi");
            assert.strictEqual(frames[1].getModule(), "@pulumi/aws");
            assert.strictEqual(frames[1].getOrigin(), "");
            assert.strictEqual(frames[2].getModule(), "example-app");
            assert.strictEqual(frames[2].getOrigin(), "");
            assert.strictEqual(frames[2].getPc()!.getUri(), url.pathToFileURL(caller).toString());
            assert.strictEqual(frames[2].getPc()!.getLine(), 30);
            assert.strictEqual(frames[3].getModule(), "ts-node");
            assert.strictEqual(frames[3].getOrigin(), "runtime");
            assert.strictEqual(frames[3].getPc()!.getLine(), 1618);
            assert.strictEqual(
                frames[3].getPc()!.getUri(),
                url.pathToFileURL(path.join(tsNode, "src", "index.ts")).toString(),
            );
            assert.strictEqual(frames[4].getModule(), "node:internal/process/task_queues");
            assert.strictEqual(frames[4].getOrigin(), "runtime");
            assert.strictEqual(frames[4].getPc(), undefined);
            assert.strictEqual(frames[5].getPc(), undefined);
            assert.strictEqual(frames[5].getOrigin(), "");
        } finally {
            fs.rmSync(root, { recursive: true, force: true });
        }
    });

    it("leaves unreadable package identities unclassified", () => {
        const root = fs.mkdtempSync(path.join(os.tmpdir(), "pulumi-stack-trace-"));
        try {
            const nested = path.join(root, "nested");
            fs.mkdirSync(nested);
            fs.writeFileSync(path.join(root, "package.json"), JSON.stringify({ name: "@pulumi/pulumi" }));
            fs.writeFileSync(path.join(nested, "package.json"), "invalid json");
            const frames = syntheticStack([`    at f (${path.join(nested, "index.js")}:1:1)`]);
            assert.strictEqual(frames[0].getModule(), "");
            assert.strictEqual(frames[0].getOrigin(), "");
        } finally {
            fs.rmSync(root, { recursive: true, force: true });
        }
    });

    it("uses original package identity when source maps point outside the package", () => {
        const root = fs.mkdtempSync(path.join(os.tmpdir(), "pulumi-stack-trace-"));
        const prepare = Error.prepareStackTrace;
        try {
            fs.writeFileSync(path.join(root, "package.json"), JSON.stringify({ name: "example-app" }));
            const mapped = path.join(root, "resource.ts");
            Error.prepareStackTrace = (_, sites) =>
                ["Error", ...sites.map(() => `    at mapped (${mapped}:10:2)`)].join("\n");
            const frames = marshalStackTrace(getStackTrace(0))!.getFramesList();
            assert.strictEqual(frames[0].getPc()!.getUri(), url.pathToFileURL(mapped).toString());
            assert.strictEqual(frames[0].getModule(), "@pulumi/pulumi");
            assert.strictEqual(frames[0].getOrigin(), "pulumi");
        } finally {
            Error.prepareStackTrace = prepare;
            fs.rmSync(root, { recursive: true, force: true });
        }
    });
});
