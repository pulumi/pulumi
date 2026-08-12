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
import { readPackageManifest, searchupPackageManifest } from "../../runtime/manifest";

function makeTempDir(): string {
    return fs.mkdtempSync(path.join(os.tmpdir(), "pulumi-manifest-test-"));
}

describe("readPackageManifest", () => {
    it("reads package.json", () => {
        const dir = makeTempDir();
        fs.writeFileSync(path.join(dir, "package.json"), `{"name":"foo","version":"1.2.3"}`);

        const result = readPackageManifest(dir);
        assert.strictEqual(result.path, path.join(dir, "package.json"));
        assert.strictEqual(result.data.name, "foo");
        assert.strictEqual(result.data.version, "1.2.3");
    });

    it("reads package.yaml", () => {
        const dir = makeTempDir();
        fs.writeFileSync(path.join(dir, "package.yaml"), "name: foo\nversion: 1.2.3\n");

        const result = readPackageManifest(dir);
        assert.strictEqual(result.path, path.join(dir, "package.yaml"));
        assert.strictEqual(result.data.name, "foo");
        assert.strictEqual(result.data.version, "1.2.3");
    });

    it("prefers package.json when both exist", () => {
        const dir = makeTempDir();
        fs.writeFileSync(path.join(dir, "package.json"), `{"name":"from-json"}`);
        fs.writeFileSync(path.join(dir, "package.yaml"), "name: from-yaml\n");

        const result = readPackageManifest(dir);
        assert.strictEqual(result.path, path.join(dir, "package.json"));
        assert.strictEqual(result.data.name, "from-json");
    });

    it("throws when neither file exists", () => {
        const dir = makeTempDir();
        assert.throws(() => readPackageManifest(dir));
    });

    it("throws on malformed package.json", () => {
        const dir = makeTempDir();
        fs.writeFileSync(path.join(dir, "package.json"), `{not json`);
        assert.throws(() => readPackageManifest(dir), /could not parse/);
    });

    it("throws on malformed package.yaml", () => {
        const dir = makeTempDir();
        fs.writeFileSync(path.join(dir, "package.yaml"), "not: : yaml: ::\n");
        assert.throws(() => readPackageManifest(dir), /could not parse/);
    });
});

describe("searchupPackageManifest", () => {
    it("finds package.json in cwd", () => {
        const dir = makeTempDir();
        fs.writeFileSync(path.join(dir, "package.json"), `{}`);

        assert.strictEqual(searchupPackageManifest(dir), path.join(dir, "package.json"));
    });

    it("finds package.yaml in cwd", () => {
        const dir = makeTempDir();
        fs.writeFileSync(path.join(dir, "package.yaml"), "{}\n");

        assert.strictEqual(searchupPackageManifest(dir), path.join(dir, "package.yaml"));
    });

    it("finds a manifest in a parent directory", () => {
        const dir = makeTempDir();
        fs.writeFileSync(path.join(dir, "package.yaml"), "{}\n");
        const sub = path.join(dir, "sub", "deeper");
        fs.mkdirSync(sub, { recursive: true });

        assert.strictEqual(searchupPackageManifest(sub), path.join(dir, "package.yaml"));
    });
});
