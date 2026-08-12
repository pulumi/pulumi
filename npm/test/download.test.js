// Copyright 2026, Pulumi Corporation. All rights reserved.

"use strict";

const { describe, it } = require("node:test");
const assert = require("node:assert/strict");
const fs = require("fs");
const os = require("os");
const path = require("path");
const { installCLI, fetchLatestVersion } = require("../lib/download");

describe("fetchLatestVersion()", () => {
    it("parses latestVersion from the Pulumi API response", async () => {
        const fakeFetch = async () =>
            JSON.stringify({ latestVersion: "3.99.0", oldestWithoutWarning: "3.99.0" });
        assert.equal(await fetchLatestVersion(fakeFetch), "3.99.0");
    });

    it("strips a leading v from the version", async () => {
        const fakeFetch = async () => JSON.stringify({ latestVersion: "v3.1.0" });
        assert.equal(await fetchLatestVersion(fakeFetch), "3.1.0");
    });
});

describe("installCLI()", () => {
    it("fetches the install script and executes it with correct args", async () => {
        if (process.platform === "win32") return; // POSIX-only test

        const tmpRoot = fs.mkdtempSync(path.join(os.tmpdir(), "pulumi-install-test-"));
        try {
            let fetchedURL;
            let executedScript;
            let executedArgs;

            const fakeFetchText = async (url) => {
                fetchedURL = url;
                return "#!/bin/sh\n# fake install script\n";
            };

            const fakeExecScript = (scriptPath, args) => {
                executedScript = scriptPath;
                executedArgs = args;
                // Simulate what install.sh does: create bin/pulumi
                const binDir = path.join(tmpRoot, "bin");
                fs.mkdirSync(binDir, { recursive: true });
                fs.writeFileSync(path.join(binDir, "pulumi"), "#!/bin/sh\necho fake\n", { mode: 0o755 });
            };

            await installCLI("3.99.0", tmpRoot, {
                fetchText: fakeFetchText,
                execScript: fakeExecScript,
            });

            assert.equal(fetchedURL, "https://get.pulumi.com/install.sh");
            assert.ok(executedScript.endsWith(".sh"), `expected .sh script, got ${executedScript}`);
            assert.deepEqual(executedArgs, ["--no-edit-path", "--install-root", tmpRoot, "--version", "3.99.0"]);
        } finally {
            fs.rmSync(tmpRoot, { recursive: true, force: true });
        }
    });

    it("cleans up the temp script even if execScript throws", async () => {
        if (process.platform === "win32") return;

        let scriptPath;
        const fakeFetchText = async () => "#!/bin/sh\n";
        const fakeExecScript = (p) => {
            scriptPath = p;
            throw new Error("install failed");
        };

        await assert.rejects(
            () => installCLI("3.99.0", "/tmp/unused", { fetchText: fakeFetchText, execScript: fakeExecScript }),
            /install failed/,
        );

        assert.ok(!fs.existsSync(scriptPath), "temp script should be cleaned up after failure");
    });
});
