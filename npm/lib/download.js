// Copyright 2026, Pulumi Corporation. All rights reserved.

"use strict";

const crypto = require("crypto");
const fs = require("fs");
const { pipeline } = require("stream/promises");
const os = require("os");
const path = require("path");
const { execFileSync } = require("child_process");
const { archiveExt, exeName } = require("./platform");

function archiveName(version, targetOS, targetArch) {
    return `pulumi-v${version}-${targetOS}-${targetArch}.${archiveExt(targetOS)}`;
}

// Primary download origin. GitHub releases are the fallback.
const GET_PULUMI_BASE = "https://get.pulumi.com/releases/sdk";
const GITHUB_BASE = "https://github.com/pulumi/pulumi/releases/download";

function archiveURL(version, targetOS, targetArch) {
    const name = archiveName(version, targetOS, targetArch);
    return `${GET_PULUMI_BASE}/${name}`;
}

function archiveURLFallback(version, targetOS, targetArch) {
    const name = archiveName(version, targetOS, targetArch);
    return `${GITHUB_BASE}/v${version}/${name}`;
}

function checksumURL(version) {
    // Note: the filename has no "v" prefix even though the release tag does.
    return `${GET_PULUMI_BASE}/pulumi-${version}-checksums.txt`;
}

function checksumURLFallback(version) {
    return `${GITHUB_BASE}/v${version}/pulumi-${version}-checksums.txt`;
}

// parseChecksums parses a sha256sum-format file: "<hex>  <filename>".
// Returns a Map<filename, sha256hex>.
function parseChecksums(text) {
    const map = new Map();
    for (const line of text.trim().split("\n")) {
        const trimmed = line.trim();
        if (!trimmed) continue;
        const space = trimmed.indexOf(" ");
        if (space === -1) continue;
        const hash = trimmed.slice(0, space).toLowerCase();
        const name = trimmed.slice(space).trimStart();
        if (hash && name) {
            map.set(name, hash);
        }
    }
    return map;
}

// computeSHA256 returns the hex SHA-256 digest of a file.
async function computeSHA256(filePath) {
    const hash = crypto.createHash("sha256");
    await pipeline(fs.createReadStream(filePath), hash);
    return hash.digest("hex");
}

async function defaultFetchText(url) {
    const res = await fetch(url);
    if (!res.ok) throw new Error(`HTTP ${res.status} fetching ${url}`);
    return res.text();
}

async function defaultFetchFile(url, dest) {
    const res = await fetch(url);
    if (!res.ok) throw new Error(`HTTP ${res.status} fetching ${url}`);
    await pipeline(res.body, fs.createWriteStream(dest));
}

// extractedBinDir locates the directory containing pulumi binaries after
// archive extraction. Handles both archive structures used across releases:
//   Current format: pulumi/
//   Legacy format:  pulumi/bin/
function extractedBinDir(extractDir) {
    const legacy = path.join(extractDir, "pulumi", "bin");
    if (fs.existsSync(legacy)) return legacy;
    const current = path.join(extractDir, "pulumi");
    if (fs.existsSync(current)) return current;
    throw new Error(`Could not find pulumi binaries in ${extractDir}`);
}

function defaultExtract(archive, targetOS, extractDir) {
    if (targetOS === "windows") {
        execFileSync("powershell", [
            "-NoProfile",
            "-Command",
            `Expand-Archive -Force -LiteralPath '${archive}' -DestinationPath '${extractDir}'`,
        ]);
    } else {
        execFileSync("tar", ["-xzf", archive, "-C", extractDir]);
    }
}

// fetchLatestVersion returns the latest stable Pulumi release version string.
async function fetchLatestVersion(fetchText = defaultFetchText) {
    const text = await fetchText("https://api.github.com/repos/pulumi/pulumi/releases/latest");
    const { tag_name } = JSON.parse(text);
    return tag_name.replace(/^v/, "");
}

// downloadBinary downloads, checksum-verifies, and caches the pulumi binary.
// IO functions are injectable for testing.
async function downloadBinary(
    version,
    targetOS,
    targetArch,
    dest,
    { fetchText = defaultFetchText, fetchFile = defaultFetchFile, extract = defaultExtract } = {},
) {
    const name = archiveName(version, targetOS, targetArch);

    let checksumText;
    try {
        checksumText = await fetchText(checksumURL(version));
    } catch (primaryErr) {
        try {
            checksumText = await fetchText(checksumURLFallback(version));
        } catch (fallbackErr) {
            throw new AggregateError([primaryErr, fallbackErr], "Failed to fetch checksums from both sources");
        }
    }
    const checksums = parseChecksums(checksumText);
    const expected = checksums.get(name);
    if (!expected) {
        throw new Error(`No checksum found for ${name}`);
    }

    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "pulumi-dl-"));
    try {
        const archive = path.join(tmpDir, name);
        const primary = archiveURL(version, targetOS, targetArch);
        const fallback = archiveURLFallback(version, targetOS, targetArch);
        try {
            await fetchFile(primary, archive);
        } catch (primaryErr) {
            try {
                await fetchFile(fallback, archive);
            } catch (fallbackErr) {
                throw new AggregateError([primaryErr, fallbackErr], "Failed to download archive from both sources");
            }
        }

        const actual = await computeSHA256(archive);
        if (actual !== expected) {
            throw new Error(`Checksum mismatch for ${name}: expected ${expected}, got ${actual}`);
        }

        extract(archive, targetOS, tmpDir);

        // Copy all binaries so language hosts (pulumi-language-nodejs, etc.)
        // are available alongside the CLI in the cache directory.
        const binDir = extractedBinDir(tmpDir);
        const destDir = path.dirname(dest);
        fs.mkdirSync(destDir, { recursive: true });
        for (const entry of fs.readdirSync(binDir)) {
            const src = path.join(binDir, entry);
            if (!fs.statSync(src).isFile()) continue;
            const out = path.join(destDir, entry);
            const outTmp = out + "." + process.pid + ".tmp";
            fs.copyFileSync(src, outTmp);
            if (targetOS !== "windows") {
                fs.chmodSync(outTmp, 0o755);
            }
            try {
                fs.renameSync(outTmp, out);
            } catch {
                // Another concurrent invocation likely won the race.
                fs.rmSync(outTmp, { force: true });
            }
        }
    } finally {
        fs.rmSync(tmpDir, { recursive: true, force: true });
    }
}

module.exports = {
    archiveName,
    archiveURL,
    archiveURLFallback,
    checksumURL,
    extractedBinDir,
    checksumURLFallback,
    parseChecksums,
    computeSHA256,
    downloadBinary,
    fetchLatestVersion,
};
