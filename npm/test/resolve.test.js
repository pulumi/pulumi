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

    it("returns false for a symlink to this package's own entry point", () => {
        const link = path.join(tmpDir, "pulumi");
        fs.symlinkSync(path.resolve(__dirname, "..", "run.js"), link);
        assert.ok(!isExecutable(link));
    });
});

describe("findSystemPulumi()", () => {
    let tmpDir;
    beforeEach(() => { tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "pulumi-test-")); });
    afterEach(() => { fs.rmSync(tmpDir, { recursive: true, force: true }); });

    it("finds an executable on PATH", () => {
        const binDir = path.join(tmpDir, "bin");
        makeExecutable(path.join(binDir, "pulumi"));
        const found = findSystemPulumi(binDir);
        assert.equal(found, path.join(binDir, "pulumi"));
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
        const found = findSystemPulumi([dirA, dirB].join(path.delimiter));
        assert.equal(found, path.join(dirB, "pulumi"));
    });
});

describe("resolve()", () => {
    let tmpDir;

    beforeEach(() => {
        tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "pulumi-test-"));
        process.env.npm_config_cache = path.join(tmpDir, "cache");
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
            targetOS: "linux",
            targetArch: "x64",
            download: async () => { throw new Error("should not download"); },
        });
        assert.equal(result, path.join(binDir, "pulumi"));
    });

    it("returns cached binary on cache hit", async () => {
        const { cacheDir } = require("../lib/cache");
        const cached = path.join(cacheDir("3.99.0"), "pulumi");
        makeExecutable(cached);

        const download = async () => { throw new Error("should not download"); };
        const result = await resolve({
            pathEnv: "",
            version: "3.99.0",
            targetOS: "linux",
            targetArch: "x64",
            download,
        });
        assert.equal(result, cached);
    });

    it("downloads on cache miss and returns cached path", async () => {
        let downloadCalled = false;
        const download = async (_version, _os, _arch, dest) => {
            downloadCalled = true;
            makeExecutable(dest);
        };

        const result = await resolve({
            pathEnv: "",
            version: "3.99.0",
            targetOS: "linux",
            targetArch: "x64",
            download,
        });

        assert.ok(downloadCalled, "download should have been called");
        assert.ok(result.includes("3.99.0"), "result path should contain version");
        assert.ok(fs.existsSync(result), "binary should exist after download");
    });

    it("fetches latest version when no version is set", async () => {
        let downloadedVersion;
        const download = async (version, _os, _arch, dest) => {
            downloadedVersion = version;
            makeExecutable(dest);
        };

        await resolve({
            pathEnv: "",
            version: "",   // simulate unset pkg.version
            targetOS: "linux",
            targetArch: "x64",
            download,
            getLatestVersion: async () => "3.50.0",
        });

        assert.equal(downloadedVersion, "3.50.0");
    });

    it("prefers pkg.version over latest when set", async () => {
        let downloadedVersion;
        const download = async (version, _os, _arch, dest) => {
            downloadedVersion = version;
            makeExecutable(dest);
        };

        await resolve({
            pathEnv: "",
            version: "3.99.0",
            targetOS: "linux",
            targetArch: "x64",
            download,
            getLatestVersion: async () => { throw new Error("should not fetch latest"); },
        });

        assert.equal(downloadedVersion, "3.99.0");
    });

    it("prefers system binary over cached binary", async () => {
        const binDir = path.join(tmpDir, "bin");
        makeExecutable(path.join(binDir, "pulumi"));

        // Also put something in cache.
        const { cacheDir } = require("../lib/cache");
        const cached = path.join(cacheDir("3.99.0"), "pulumi");
        makeExecutable(cached);

        const result = await resolve({
            pathEnv: binDir,
            version: "3.99.0",
            targetOS: "linux",
            targetArch: "x64",
            download: async () => { throw new Error("should not download"); },
        });
        assert.equal(result, path.join(binDir, "pulumi"));
    });
});
