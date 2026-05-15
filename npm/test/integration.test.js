// Copyright 2026, Pulumi Corporation. All rights reserved.
//
// Integration tests: real network requests and binary execution.
// Run via `make test_fast` (uses $(VERSION)) or directly:
//
//   PULUMI_VERSION=3.237.0 node --test test/integration.test.js

"use strict";

const { describe, it } = require("node:test");
const assert = require("node:assert/strict");
const { spawnSync } = require("child_process");
const fs = require("fs");
const os = require("os");
const path = require("path");

const version = process.env.PULUMI_VERSION;
if (!version) {
    throw new Error("PULUMI_VERSION must be set to a released version");
}

const { resolve } = require("../lib/resolve");
// Expected language hosts bundled in every Pulumi release.
const LANGUAGE_HOSTS = [
    "pulumi-language-nodejs",
    "pulumi-language-python",
    "pulumi-language-go",
    "pulumi-language-yaml",
];

function pulumi(bin, args, opts = {}) {
    const result = spawnSync(bin, args, {
        encoding: "utf8",
        ...opts,
        env: { PULUMI_SKIP_UPDATE_CHECK: "1", ...opts.env },
    });
    return result;
}

describe(`pulumi v${version} integration`, () => {
    let bin;
    let binDir;
    let env;

    it("resolves and downloads the CLI", async () => {
        bin = await resolve({
            pathEnv: "", // bypass PATH deferral — exercise the download path
            version,
            targetOS: process.platform === "win32" ? "windows" : process.platform,
            targetArch: process.arch,
        });
        binDir = path.dirname(bin);

        assert.ok(fs.existsSync(bin), `pulumi binary not found at ${bin}`);
        env = {
            ...process.env,
            PATH: binDir + path.delimiter + (process.env.PATH || ""),
            PULUMI_SKIP_UPDATE_CHECK: "1",
        };
    });

    it("installs language host binaries alongside the CLI", () => {
        assert.ok(binDir, "resolve must run first");
        const exe = currentOS() === "windows" ? ".exe" : "";
        for (const host of LANGUAGE_HOSTS) {
            const p = path.join(binDir, `${host}${exe}`);
            assert.ok(fs.existsSync(p), `expected ${host} to be installed at ${p}`);
        }
    });

    it("runs a YAML program, exercising pulumi-language-yaml discovery", () => {
        assert.ok(bin && env, "resolve must run first");

        const projectDir = fs.mkdtempSync(path.join(os.tmpdir(), "pulumi-itest-proj-"));
        const stateDir = fs.mkdtempSync(path.join(os.tmpdir(), "pulumi-itest-state-"));

        try {
            fs.writeFileSync(path.join(projectDir, "Pulumi.yaml"), "name: integration-test\nruntime: yaml\n");

            const localEnv = {
                ...env,
                PULUMI_BACKEND_URL: `file://${stateDir}`,
                PULUMI_CONFIG_PASSPHRASE: "",
            };

            const init = pulumi(bin, ["stack", "init", "test", "--non-interactive"], {
                cwd: projectDir,
                env: localEnv,
            });
            assert.equal(init.status, 0, `stack init failed:\n${init.stdout}\n${init.stderr}`);

            const preview = pulumi(bin, ["preview", "--non-interactive"], {
                cwd: projectDir,
                env: localEnv,
            });
            assert.equal(
                preview.status,
                0,
                `preview failed — pulumi-language-yaml was likely not found:\n${preview.stdout}\n${preview.stderr}`,
            );
        } finally {
            fs.rmSync(projectDir, { recursive: true, force: true });
            fs.rmSync(stateDir, { recursive: true, force: true });
        }
    });

    it("reports the correct version", () => {
        assert.ok(bin && env, "resolve must run first");
        const result = pulumi(bin, ["version"], { env });
        assert.equal(result.status, 0, `pulumi version exited ${result.status}\n${result.stderr}`);
        assert.ok(
            result.stdout.includes(version),
            `expected output to contain ${version}, got: ${result.stdout.trim()}`,
        );
    });
});
