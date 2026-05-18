// Copyright 2026, Pulumi Corporation. All rights reserved.

"use strict";

const { describe, it, beforeEach, afterEach } = require("node:test");
const assert = require("node:assert/strict");
const fs = require("fs");
const os = require("os");
const path = require("path");
const { isExecutable, resolve } = require("../lib/resolve");

function makeExecutable(filePath, content = "binary") {
    fs.mkdirSync(path.dirname(filePath), { recursive: true });
    fs.writeFileSync(filePath, content);
    fs.chmodSync(filePath, 0o755);
}

function fakeInstall(_version, root) {
    makeExecutable(path.join(root, "bin", "pulumi"));
}

describe("isExecutable()", () => {
    let tmpDir;
    beforeEach(() => { tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "pulumi-test-")); });
    afterEach(() => { fs.rmSync(tmpDir, { recursive: true, force: true }); });

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
});

describe("resolve()", () => {
    let tmpDir;

    beforeEach(() => {
        tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "pulumi-test-"));
        process.env.PULUMI_HOME = path.join(tmpDir, "pulumi-home");
    });

    afterEach(() => {
        fs.rmSync(tmpDir, { recursive: true, force: true });
        delete process.env.PULUMI_HOME;
    });

    it("returns cached binary on cache hit", async () => {
        const { cacheDir } = require("../lib/cache");
        const cached = path.join(cacheDir("3.99.0"), "bin", "pulumi");
        makeExecutable(cached);

        const result = await resolve({
            version: "3.99.0",
            install: async () => { throw new Error("should not install"); },
        });
        assert.equal(result, cached);
    });

    it("installs on cache miss and returns binary path", async () => {
        let installedVersion;
        let installedRoot;

        const result = await resolve({
            version: "3.99.0",
            install: async (version, root) => {
                installedVersion = version;
                installedRoot = root;
                fakeInstall(version, root);
            },
        });

        assert.equal(installedVersion, "3.99.0");
        assert.ok(installedRoot.includes("3.99.0"));
        assert.ok(result.endsWith("bin/pulumi") || result.endsWith("bin\\pulumi.exe"),
            `expected bin/pulumi in path, got ${result}`);
        assert.ok(fs.existsSync(result));
    });

    it("fetches latest version when no version is set", async () => {
        let installedVersion;

        await resolve({
            version: "",
            install: async (version, root) => {
                installedVersion = version;
                fakeInstall(version, root);
            },
            getLatestVersion: async () => "3.50.0",
        });

        assert.equal(installedVersion, "3.50.0");
    });

    it("prefers explicit version over fetching latest", async () => {
        let installedVersion;

        await resolve({
            version: "3.99.0",
            install: async (version, root) => {
                installedVersion = version;
                fakeInstall(version, root);
            },
            getLatestVersion: async () => { throw new Error("should not fetch latest"); },
        });

        assert.equal(installedVersion, "3.99.0");
    });
});
