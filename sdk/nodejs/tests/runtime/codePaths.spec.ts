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
import * as asset from "../../asset";
import { computeCodePaths, packageArchivePath } from "../../runtime/closure/codePaths";

describe("packageArchivePath", () => {
    it("keeps plain node_modules paths as-is", () => {
        assert.strictEqual(packageArchivePath("node_modules/semver", "semver"), "node_modules/semver");
    });

    it("keeps nested node_modules paths as-is", () => {
        assert.strictEqual(
            packageArchivePath("node_modules/execa/node_modules/semver", "semver"),
            "node_modules/execa/node_modules/semver",
        );
    });

    it("keeps scoped package paths as-is", () => {
        assert.strictEqual(packageArchivePath("node_modules/@scope/name", "@scope/name"), "node_modules/@scope/name");
    });

    it("keeps workspace paths as-is", () => {
        assert.strictEqual(packageArchivePath("../node_modules/semver", "semver"), "../node_modules/semver");
    });

    it("flattens pnpm store paths", () => {
        assert.strictEqual(
            packageArchivePath("node_modules/.pnpm/semver@7.5.4/node_modules/semver", "semver"),
            "node_modules/semver",
        );
    });

    it("flattens scoped pnpm store paths", () => {
        assert.strictEqual(
            packageArchivePath("node_modules/.pnpm/@scope+name@1.0.0/node_modules/@scope/name", "@scope/name"),
            "node_modules/@scope/name",
        );
    });

    it("flattens peer-suffixed pnpm store paths", () => {
        assert.strictEqual(
            packageArchivePath("node_modules/.pnpm/foo@1.0.0_react@18.2.0/node_modules/foo", "foo"),
            "node_modules/foo",
        );
    });

    it("only matches .pnpm as a full path segment", () => {
        assert.strictEqual(packageArchivePath("node_modules/.pnpm-utils", ".pnpm-utils"), "node_modules/.pnpm-utils");
    });
});

describe("computeCodePaths", () => {
    let directory: string;
    let previousDirectory: string;

    beforeEach(() => {
        previousDirectory = process.cwd();
        directory = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), "pulumi-code-paths-")));
        process.chdir(directory);
    });

    afterEach(() => {
        process.chdir(previousDirectory);
        fs.rmSync(directory, { recursive: true, force: true });
    });

    function manifest(location: string, contents: any) {
        fs.mkdirSync(location, { recursive: true });
        fs.writeFileSync(path.join(location, "package.json"), JSON.stringify(contents));
    }

    function link(target: string, location: string) {
        fs.mkdirSync(path.dirname(location), { recursive: true });
        fs.symlinkSync(path.resolve(target), location, "junction");
    }

    async function sources(options = {}) {
        const result = new Map<string, string>();
        for (const [location, archive] of await computeCodePaths(options)) {
            assert.ok(archive instanceof asset.FileArchive);
            const source = await archive.path;
            assert.ok(fs.lstatSync(source).isDirectory());
            result.set(location, source);
        }
        return result;
    }

    it("includes packages whose package.json is hidden by exports or that have no entry point", async () => {
        // app/
        // ├── package.json                 dependencies: @scope/hidden, assets
        // └── node_modules/
        //     ├── @scope/hidden/
        //     │   ├── package.json         exports exposes only index.js
        //     │   └── index.js             working entry point; package.json is hidden
        //     └── assets/
        //         └── package.json         no entry point
        manifest(".", { dependencies: { "@scope/hidden": "*", assets: "*" } });
        manifest("node_modules/@scope/hidden", { name: "@scope/hidden", exports: { ".": "./index.js" } });
        fs.writeFileSync("node_modules/@scope/hidden/index.js", "module.exports = {};\n");
        manifest("node_modules/assets", { name: "assets" });
        assert.deepStrictEqual(
            await sources(),
            new Map([
                ["node_modules/@scope/hidden", "node_modules/@scope/hidden"],
                ["node_modules/assets", "node_modules/assets"],
            ]),
        );
    });

    it("resolves workspace-local dependencies before hoisted dependencies", async () => {
        // workspace/
        // ├── package.json                 workspaces: packages/*
        // ├── node_modules/
        // │   ├── shared/                  v1
        // │   └── linked -> ../packages/linked
        // └── packages/
        //     ├── app/                     cwd
        //     │   ├── package.json         dependencies: shared, linked
        //     │   └── node_modules/shared/ v2
        //     └── linked/
        //         └── package.json         dependencies: shared (resolves to v1)
        manifest(".", { workspaces: ["packages/*"] });
        manifest("packages/app", { dependencies: { shared: "*", linked: "*" } });
        manifest("node_modules/shared", { version: "1.0.0" });
        manifest("packages/app/node_modules/shared", { version: "2.0.0" });
        manifest("packages/linked", { dependencies: { shared: "*" } });
        link("packages/linked", "node_modules/linked");
        process.chdir("packages/app");
        assert.deepStrictEqual(
            await sources(),
            new Map([
                ["node_modules/shared", "node_modules/shared"],
                ["../../node_modules/linked", "../linked"],
                ["../../node_modules/shared", "../../node_modules/shared"],
            ]),
        );
    });

    it("follows pnpm links and preserves conflicting versions and dependency cycles", async () => {
        // app/
        // ├── package.json                 dependencies: parent, shared
        // └── node_modules/
        //     ├── parent -> .pnpm/parent@1/node_modules/parent
        //     ├── shared -> .pnpm/shared@1/node_modules/shared
        //     └── .pnpm/
        //         ├── parent@1/node_modules/
        //         │   ├── parent/          dependencies: shared
        //         │   └── shared -> ../../shared@2/node_modules/shared
        //         ├── shared@1/node_modules/shared/
        //         └── shared@2/node_modules/
        //             ├── shared/          dependencies: parent (cycle)
        //             └── parent -> ../../parent@1/node_modules/parent
        manifest(".", { dependencies: { parent: "*", shared: "*" } });
        const parent = "node_modules/.pnpm/parent@1/node_modules/parent";
        const shared1 = "node_modules/.pnpm/shared@1/node_modules/shared";
        const shared2 = "node_modules/.pnpm/shared@2/node_modules/shared";
        manifest(parent, { name: "parent", dependencies: { shared: "*" } });
        manifest(shared1, { name: "shared", version: "1.0.0" });
        manifest(shared2, { name: "shared", version: "2.0.0", dependencies: { parent: "*" } });
        link(parent, "node_modules/parent");
        link(shared1, "node_modules/shared");
        link(shared2, "node_modules/.pnpm/parent@1/node_modules/shared");
        link(parent, "node_modules/.pnpm/shared@2/node_modules/parent");
        assert.deepStrictEqual(
            await sources(),
            new Map([
                ["node_modules/parent", parent],
                ["node_modules/shared", shared1],
                ["node_modules/parent/node_modules/shared", shared2],
            ]),
        );
    });

    it("preserves deployment-only packages, explicit includes, and exclusions", async () => {
        // app/
        // ├── package.json                 dependencies: deployment, excluded; devDependencies: dev
        // └── node_modules/
        //     ├── deployment/
        //     │   └── package.json         dependencies: unused; pulumi.runtimeDependencies: runtime
        //     ├── runtime/                 included through deployment
        //     ├── unused/                  omitted with deployment
        //     ├── excluded/                explicitly excluded
        //     ├── dev/                     omitted dev dependency
        //     └── extra/                   explicitly included
        manifest(".", { dependencies: { deployment: "*", excluded: "*" }, devDependencies: { dev: "*" } });
        manifest("node_modules/deployment", {
            dependencies: { unused: "*" },
            pulumi: { runtimeDependencies: { runtime: "*" } },
        });
        for (const name of ["runtime", "unused", "excluded", "dev", "extra"]) {
            manifest(`node_modules/${name}`, { name });
        }
        assert.deepStrictEqual(
            await sources({
                extraIncludePackages: ["extra"],
                extraExcludePackages: ["excluded"],
            }),
            new Map([
                ["node_modules/extra", "node_modules/extra"],
                ["node_modules/runtime", "node_modules/runtime"],
            ]),
        );
    });

    it("preserves npm's nested versions inside their parent archive", async () => {
        // app/
        // ├── package.json                 dependencies: parent, shared
        // └── node_modules/
        //     ├── shared/                  v1
        //     ├── nested-only/             used only by shared v2
        //     └── parent/
        //         ├── package.json         dependencies: shared
        //         └── node_modules/shared/ v2; dependencies: nested-only; carried inside parent
        manifest(".", { dependencies: { parent: "*", shared: "*" } });
        manifest("node_modules/shared", { name: "shared", version: "1.0.0" });
        manifest("node_modules/parent", { name: "parent", dependencies: { shared: "*" } });
        manifest("node_modules/parent/node_modules/shared", {
            name: "shared",
            version: "2.0.0",
            dependencies: { "nested-only": "*" },
        });
        manifest("node_modules/nested-only", { name: "nested-only" });
        assert.deepStrictEqual(
            await sources(),
            new Map([
                ["node_modules/parent", "node_modules/parent"],
                ["node_modules/shared", "node_modules/shared"],
                ["node_modules/nested-only", "node_modules/nested-only"],
            ]),
        );
    });

    it("uses the npm alias when nesting conflicting pnpm versions", async () => {
        // app/
        // ├── package.json                 dependencies: parent, alias = npm:actual@1.0.0
        // └── node_modules/
        //     ├── parent -> .pnpm/parent@1/node_modules/parent
        //     ├── alias -> .pnpm/actual@1/node_modules/actual
        //     └── .pnpm/
        //         ├── parent@1/node_modules/
        //         │   ├── parent/          dependencies: alias = npm:actual@2.0.0
        //         │   └── alias -> ../../actual@2/node_modules/actual
        //         ├── actual@1/node_modules/actual/  name: actual; v1
        //         └── actual@2/node_modules/actual/  name: actual; v2
        manifest(".", { dependencies: { parent: "*", alias: "npm:actual@1.0.0" } });
        const parent = "node_modules/.pnpm/parent@1/node_modules/parent";
        const actual1 = "node_modules/.pnpm/actual@1/node_modules/actual";
        const actual2 = "node_modules/.pnpm/actual@2/node_modules/actual";
        manifest(parent, { name: "parent", dependencies: { alias: "npm:actual@2.0.0" } });
        manifest(actual1, { name: "actual", version: "1.0.0" });
        manifest(actual2, { name: "actual", version: "2.0.0" });
        link(parent, "node_modules/parent");
        link(actual1, "node_modules/alias");
        link(actual2, "node_modules/.pnpm/parent@1/node_modules/alias");
        assert.deepStrictEqual(
            await sources(),
            new Map([
                ["node_modules/parent", parent],
                ["node_modules/alias", actual1],
                ["node_modules/parent/node_modules/alias", actual2],
            ]),
        );
    });

    it("resolves dependencies from a linked package's real directory", async () => {
        // fixture/
        // ├── app/                         cwd
        // │   ├── package.json             dependencies: linked
        // │   └── node_modules/
        // │       ├── linked -> ../../external/linked
        // │       └── shared/              v1, must not be used by linked
        // └── external/
        //     ├── linked/
        //     │   └── package.json         dependencies: shared
        //     └── node_modules/shared/     v2, resolved from linked's real directory
        manifest("app", { dependencies: { linked: "file:../external/linked" } });
        manifest("app/node_modules/shared", { name: "shared", version: "1.0.0" });
        manifest("external/linked", { name: "linked", dependencies: { shared: "*" } });
        manifest("external/node_modules/shared", { name: "shared", version: "2.0.0" });
        link("external/linked", "app/node_modules/linked");
        process.chdir("app");
        assert.deepStrictEqual(
            await sources(),
            new Map([
                ["node_modules/linked", "../external/linked"],
                ["../external/node_modules/shared", "../external/node_modules/shared"],
            ]),
        );
    });

    it("warns and continues for missing dependencies and broken links", async () => {
        // app/
        // ├── package.json                 dependencies: installed, missing, broken
        // └── node_modules/
        //     ├── installed/
        //     └── broken -> ../absent       dangling link; missing is not installed either
        manifest(".", { dependencies: { installed: "*", missing: "*", broken: "*" } });
        manifest("node_modules/installed", { name: "installed" });
        link("absent", "node_modules/broken");
        const warnings: string[] = [];
        const originalWarn = console.warn;
        console.warn = (message: string) => warnings.push(message);
        try {
            assert.deepStrictEqual(await sources(), new Map([["node_modules/installed", "node_modules/installed"]]));
        } finally {
            console.warn = originalWarn;
        }
        assert.strictEqual(warnings.length, 2);
        assert.match(warnings[0], /Could not include required dependency 'missing'/);
        assert.match(warnings[1], /Could not include required dependency 'broken'/);
    });
});
