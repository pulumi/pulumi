// Copyright 2026, Pulumi Corporation. All rights reserved.

"use strict";

const { describe, it, beforeEach, afterEach } = require("node:test");
const assert = require("node:assert/strict");
const fs = require("fs");
const os = require("os");
const path = require("path");
const { findSystemPulumi, isExecutable, resolve } = require("../lib/resolve");

function makeExecutable(filePath, content = "binary") {
    fs.mkdirSync(path.dirname(filePath), { recursive: true });
    fs.writeFileSync(filePath, content);
    fs.chmodSync(filePath, 0o755);
}

// Simulate what installCLI does: create {root}/bin/pulumi.
function fakeInstall(_version, root) {
    makeExecutable(path.join(root, "bin", "pulumi"));
}

describe("isExecutable()", () => {
    let tmpDir;
    beforeEach(() => {
        tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "pulumi-test-"));
    });
    afterEach(() => {
        fs.rmSync(tmpDir, { recursive: true, force: true });
    });

    it("returns true for a non-empty executable file", () => {
        const f = path.join(tmpDir, "bin");
        makeExecutable(f);
        assert.ok(isExecutable(f));
    });

    it("returns false for a non-existent file", () => {
        assert.ok(!isExecutable(path.join(tmpDir, "missing")));
    });

    it("returns false for a non-executable file", () => {
        const f = path.join(tmpDir, "noexec");
        fs.writeFileSync(f, "content");
        fs.chmodSync(f, 0o644);
        assert.ok(!isExecutable(f));
    });

    it("returns false for an empty file", () => {
        const f = path.join(tmpDir, "empty");
        fs.writeFileSync(f, "");
        fs.chmodSync(f, 0o755);
        assert.ok(!isExecutable(f));
    });

    it("returns false for a symlink to this package's own entry point", () => {
        const link = path.join(tmpDir, "pulumi");
        fs.symlinkSync(path.resolve(__dirname, "..", "run.js"), link);
        assert.ok(!isExecutable(link));
    });
});

describe("findSystemPulumi()", () => {
    let tmpDir;
    beforeEach(() => {
        tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "pulumi-test-"));
    });
    afterEach(() => {
        fs.rmSync(tmpDir, { recursive: true, force: true });
    });

    it("finds an executable on PATH", () => {
        const binDir = path.join(tmpDir, "bin");
        makeExecutable(path.join(binDir, "pulumi"));
        assert.equal(findSystemPulumi(binDir), path.join(binDir, "pulumi"));
    });

    it("skips this package's own entry point when symlinked onto PATH", () => {
        const binDir = path.join(tmpDir, "bin");
        fs.mkdirSync(binDir, { recursive: true });
        fs.symlinkSync(path.resolve(__dirname, "..", "run.js"), path.join(binDir, "pulumi"));
        assert.equal(findSystemPulumi(binDir), null);
    });

    it("skips paths containing node_modules", () => {
        const binDir = path.join(tmpDir, "node_modules", ".bin");
        makeExecutable(path.join(binDir, "pulumi"));
        assert.equal(findSystemPulumi(binDir), null);
    });

    it("returns null when PATH is empty", () => {
        assert.equal(findSystemPulumi(""), null);
    });

    it("searches multiple PATH entries", () => {
        const dirA = path.join(tmpDir, "a");
        const dirB = path.join(tmpDir, "b");
        fs.mkdirSync(dirA, { recursive: true });
        makeExecutable(path.join(dirB, "pulumi"));
        assert.equal(findSystemPulumi([dirA, dirB].join(path.delimiter)), path.join(dirB, "pulumi"));
    });
});

describe("resolve()", () => {
    let tmpDir;

    beforeEach(() => {
        tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "pulumi-test-"));
        process.env.npm_config_cache = path.join(tmpDir, "npm-cache");
    });

    afterEach(() => {
        fs.rmSync(tmpDir, { recursive: true, force: true });
        delete process.env.npm_config_cache;
    });

    it("returns system binary when found on PATH", async () => {
        const binDir = path.join(tmpDir, "bin");
        makeExecutable(path.join(binDir, "pulumi"));

        const result = await resolve({
            pathEnv: binDir,
            version: "3.99.0",
            install: async () => { throw new Error("should not install"); },
        });
        assert.equal(result, path.join(binDir, "pulumi"));
    });

    it("returns cached binary on cache hit", async () => {
        const { cacheDir } = require("../lib/cache");
        // install.sh creates {root}/bin/pulumi
        const cached = path.join(cacheDir("3.99.0"), "bin", "pulumi");
        makeExecutable(cached);

        const result = await resolve({
            pathEnv: "",
            version: "3.99.0",
            install: async () => { throw new Error("should not install"); },
        });
        assert.equal(result, cached);
    });

    it("installs on cache miss and returns binary path", async () => {
        let installedVersion;
        let installedRoot;

        const result = await resolve({
            pathEnv: "",
            version: "3.99.0",
            install: async (version, root) => {
                installedVersion = version;
                installedRoot = root;
                fakeInstall(version, root);
            },
        });

        assert.equal(installedVersion, "3.99.0");
        assert.ok(installedRoot.includes("3.99.0"), "install root should contain version");
        assert.ok(result.endsWith("bin/pulumi") || result.endsWith("bin\\pulumi.exe"),
            `expected bin/pulumi in path, got ${result}`);
        assert.ok(fs.existsSync(result), "binary should exist after install");
    });

    it("fetches latest version when no version is set", async () => {
        let installedVersion;

        await resolve({
            pathEnv: "",
            version: "",
            install: async (version, root) => {
                installedVersion = version;
                fakeInstall(version, root);
            },
            getLatestVersion: async () => "3.50.0",
        });

        assert.equal(installedVersion, "3.50.0");
    });

    it("prefers pkg.version over latest when set", async () => {
        let installedVersion;

        await resolve({
            pathEnv: "",
            version: "3.99.0",
            install: async (version, root) => {
                installedVersion = version;
                fakeInstall(version, root);
            },
            getLatestVersion: async () => { throw new Error("should not fetch latest"); },
        });

        assert.equal(installedVersion, "3.99.0");
    });

    it("prefers system binary over cached binary", async () => {
        const binDir = path.join(tmpDir, "bin");
        makeExecutable(path.join(binDir, "pulumi"));

        const { cacheDir } = require("../lib/cache");
        makeExecutable(path.join(cacheDir("3.99.0"), "bin", "pulumi"));

        const result = await resolve({
            pathEnv: binDir,
            version: "3.99.0",
            install: async () => { throw new Error("should not install"); },
        });
        assert.equal(result, path.join(binDir, "pulumi"));
    });
});
