// Copyright 2026, Pulumi Corporation. All rights reserved.

"use strict";

const { describe, it } = require("node:test");
const assert = require("node:assert/strict");
const crypto = require("crypto");
const fs = require("fs");
const os = require("os");
const path = require("path");
const {
    archiveName,
    archiveURL,
    archiveURLFallback,
    checksumURL,
    checksumURLFallback,
    parseChecksums,
    computeSHA256,
    downloadBinary,
    fetchLatestVersion,
} = require("../lib/download");

describe("archiveName()", () => {
    const cases = [
        ["3.99.0", "darwin", "arm64", "pulumi-v3.99.0-darwin-arm64.tar.gz"],
        ["3.99.0", "darwin", "x64", "pulumi-v3.99.0-darwin-x64.tar.gz"],
        ["3.99.0", "linux", "arm64", "pulumi-v3.99.0-linux-arm64.tar.gz"],
        ["3.99.0", "linux", "x64", "pulumi-v3.99.0-linux-x64.tar.gz"],
        ["3.99.0", "windows", "x64", "pulumi-v3.99.0-windows-x64.zip"],
        ["3.99.0", "windows", "arm64", "pulumi-v3.99.0-windows-arm64.zip"],
    ];
    for (const [version, targetOS, targetArch, expected] of cases) {
        it(`${targetOS}/${targetArch}`, () => {
            assert.equal(archiveName(version, targetOS, targetArch), expected);
        });
    }
});

describe("archiveURL()", () => {
    it("uses get.pulumi.com as primary", () => {
        const url = archiveURL("3.99.0", "linux", "x64");
        assert.equal(url, "https://get.pulumi.com/releases/sdk/pulumi-v3.99.0-linux-x64.tar.gz");
    });
});

describe("archiveURLFallback()", () => {
    it("uses GitHub releases as fallback", () => {
        const url = archiveURLFallback("3.99.0", "linux", "x64");
        assert.equal(url, "https://github.com/pulumi/pulumi/releases/download/v3.99.0/pulumi-v3.99.0-linux-x64.tar.gz");
    });
});

describe("checksumURL()", () => {
    it("uses get.pulumi.com as primary", () => {
        assert.equal(checksumURL("3.99.0"), "https://get.pulumi.com/releases/sdk/pulumi-3.99.0-checksums.txt");
    });
});

describe("checksumURLFallback()", () => {
    it("uses GitHub releases as fallback", () => {
        assert.equal(
            checksumURLFallback("3.99.0"),
            "https://github.com/pulumi/pulumi/releases/download/v3.99.0/pulumi-3.99.0-checksums.txt",
        );
    });
});

describe("fetchLatestVersion()", () => {
    it("parses tag_name from GitHub releases API response", async () => {
        const fakeFetch = async () => JSON.stringify({ tag_name: "v3.99.0", name: "v3.99.0" });
        assert.equal(await fetchLatestVersion(fakeFetch), "3.99.0");
    });

    it("strips leading v from tag_name", async () => {
        const fakeFetch = async () => JSON.stringify({ tag_name: "v3.1.0" });
        assert.equal(await fetchLatestVersion(fakeFetch), "3.1.0");
    });
});

describe("parseChecksums()", () => {
    it("parses standard sha256sum format", () => {
        const text = ["abc123  pulumi-v3.99.0-linux-x64.tar.gz", "def456  pulumi-v3.99.0-darwin-arm64.tar.gz"].join(
            "\n",
        );
        const map = parseChecksums(text);
        assert.equal(map.get("pulumi-v3.99.0-linux-x64.tar.gz"), "abc123");
        assert.equal(map.get("pulumi-v3.99.0-darwin-arm64.tar.gz"), "def456");
    });

    it("normalizes hash to lowercase", () => {
        const map = parseChecksums("ABCDEF  somefile.tar.gz");
        assert.equal(map.get("somefile.tar.gz"), "abcdef");
    });

    it("handles trailing newlines and blank lines", () => {
        const map = parseChecksums("\nabc123  file.tar.gz\n\n");
        assert.equal(map.get("file.tar.gz"), "abc123");
        assert.equal(map.size, 1);
    });

    it("returns empty map for empty input", () => {
        assert.equal(parseChecksums("").size, 0);
    });
});

describe("computeSHA256()", () => {
    it("computes correct hash for known content", async () => {
        const tmp = path.join(os.tmpdir(), `pulumi-test-${process.pid}.txt`);
        try {
            fs.writeFileSync(tmp, "hello pulumi\n");
            const expected = crypto.createHash("sha256").update("hello pulumi\n").digest("hex");
            const actual = await computeSHA256(tmp);
            assert.equal(actual, expected);
        } finally {
            fs.rmSync(tmp, { force: true });
        }
    });
});

describe("downloadBinary()", () => {
    // buildFakeArchive creates a tar.gz with a realistic set of binaries and
    // returns { archivePath, binaries } where binaries is a map of name→content.
    async function buildFakeArchive(destDir, version, targetOS, targetArch) {
        const { execSync } = require("child_process");
        const ext = targetOS === "windows" ? ".exe" : "";
        const binaries = {
            [`pulumi${ext}`]: "#!/bin/sh\necho fake pulumi\n",
            [`pulumi-language-nodejs${ext}`]: "#!/bin/sh\necho fake nodejs\n",
            [`pulumi-language-python${ext}`]: "#!/bin/sh\necho fake python\n",
        };

        const srcDir = fs.mkdtempSync(path.join(os.tmpdir(), "pulumi-fake-"));
        const archiveName = `pulumi-v${version}-${targetOS}-${targetArch}.tar.gz`;
        const archivePath = path.join(destDir, archiveName);
        try {
            fs.mkdirSync(path.join(srcDir, "pulumi"));
            for (const [name, content] of Object.entries(binaries)) {
                fs.writeFileSync(path.join(srcDir, "pulumi", name), content);
            }
            execSync(`tar -czf "${archivePath}" -C "${srcDir}" pulumi`);
        } finally {
            fs.rmSync(srcDir, { recursive: true, force: true });
        }
        return { archivePath, binaries };
    }

    it("extracts the CLI and all language host binaries", async () => {
        const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "pulumi-dl-test-"));
        try {
            const { archivePath, binaries } = await buildFakeArchive(tmpDir, "3.99.0", "linux", "x64");
            const archiveHash = await computeSHA256(archivePath);

            const fakeName = path.basename(archivePath);
            const fakeFetchText = async () => `${archiveHash}  ${fakeName}\n`;
            const fakeFetchFile = async (_url, dest) => {
                fs.copyFileSync(archivePath, dest);
            };

            const dest = path.join(tmpDir, "bin", "pulumi");
            await downloadBinary("3.99.0", "linux", "x64", dest, {
                fetchText: fakeFetchText,
                fetchFile: fakeFetchFile,
            });

            const binDir = path.dirname(dest);
            for (const name of Object.keys(binaries)) {
                const p = path.join(binDir, name);
                assert.ok(fs.existsSync(p), `expected ${name} to be extracted alongside pulumi`);
                assert.equal(fs.readFileSync(p, "utf8"), binaries[name]);
            }
        } finally {
            fs.rmSync(tmpDir, { recursive: true, force: true });
        }
    });

    it("rejects on checksum mismatch", async () => {
        const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "pulumi-dl-test-"));
        try {
            const fakeName = "pulumi-v3.99.0-linux-x64.tar.gz";

            const fakeFetchText = async () => `deadbeef  ${fakeName}\n`;

            const fakeFetchFile = async (_url, dest) => {
                // Write something whose SHA256 won't match "deadbeef".
                const srcDir = fs.mkdtempSync(path.join(os.tmpdir(), "pulumi-fake-"));
                try {
                    fs.mkdirSync(path.join(srcDir, "pulumi"));
                    fs.writeFileSync(path.join(srcDir, "pulumi", "pulumi"), "wrong content");
                    const { execSync } = require("child_process");
                    execSync(`tar -czf "${dest}" -C "${srcDir}" pulumi`);
                } finally {
                    fs.rmSync(srcDir, { recursive: true, force: true });
                }
            };

            const dest = path.join(tmpDir, "bin", "pulumi");
            await assert.rejects(
                () =>
                    downloadBinary("3.99.0", "linux", "x64", dest, {
                        fetchText: fakeFetchText,
                        fetchFile: fakeFetchFile,
                    }),
                /Checksum mismatch/,
            );
        } finally {
            fs.rmSync(tmpDir, { recursive: true, force: true });
        }
    });

    it("falls back to GitHub when get.pulumi.com fails", async () => {
        const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "pulumi-dl-test-"));
        try {
            const { execSync } = require("child_process");
            const fakeContent = "#!/bin/sh\necho fake pulumi\n";
            const fakeName = "pulumi-v3.99.0-linux-x64.tar.gz";

            const archiveSrcDir = fs.mkdtempSync(path.join(os.tmpdir(), "pulumi-fake-"));
            const prebuiltArchive = path.join(tmpDir, fakeName);
            try {
                fs.mkdirSync(path.join(archiveSrcDir, "pulumi"));
                fs.writeFileSync(path.join(archiveSrcDir, "pulumi", "pulumi"), fakeContent);
                execSync(`tar -czf "${prebuiltArchive}" -C "${archiveSrcDir}" pulumi`);
            } finally {
                fs.rmSync(archiveSrcDir, { recursive: true, force: true });
            }

            const archiveHash = await computeSHA256(prebuiltArchive);
            const fakeFetchText = async () => `${archiveHash}  ${fakeName}\n`;

            let primaryAttempted = false;
            const fakeFetchFile = async (url, dest) => {
                if (url.includes("get.pulumi.com")) {
                    primaryAttempted = true;
                    throw new Error("simulated get.pulumi.com failure");
                }
                // Fallback (GitHub) succeeds.
                fs.copyFileSync(prebuiltArchive, dest);
            };

            const dest = path.join(tmpDir, "bin", "pulumi");
            await downloadBinary("3.99.0", "linux", "x64", dest, {
                fetchText: fakeFetchText,
                fetchFile: fakeFetchFile,
            });

            assert.ok(primaryAttempted, "should have tried get.pulumi.com first");
            assert.ok(fs.existsSync(dest), "binary should exist after fallback");
        } finally {
            fs.rmSync(tmpDir, { recursive: true, force: true });
        }
    });

    it("rejects when archive has no known checksum entry", async () => {
        const fakeFetchText = async () => `abc123  pulumi-v3.99.0-windows-x64.zip\n`;
        const dest = path.join(os.tmpdir(), "should-not-exist");

        await assert.rejects(
            () =>
                downloadBinary("3.99.0", "linux", "x64", dest, {
                    fetchText: fakeFetchText,
                    fetchFile: async () => {},
                    extract: () => {},
                }),
            /No checksum found/,
        );
    });
});
