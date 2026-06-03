// Copyright 2026, Pulumi Corporation. All rights reserved.

"use strict";

const fs = require("fs");
const path = require("path");
const { cacheDir } = require("./cache");
const { installCLI, fetchLatestVersion } = require("./download");

const pkg = require("../package.json");

function isExecutable(filePath) {
    try {
        fs.accessSync(filePath, fs.constants.X_OK);
        return fs.statSync(filePath).size > 0;
    } catch {
        return false;
    }
}

// resolve returns the path to the pulumi binary, installing it if not already
// cached. IO functions are injectable for testing.
async function resolve({
    version = process.env.PULUMI_VERSION || pkg.version,
    install = installCLI,
    getLatestVersion = fetchLatestVersion,
} = {}) {
    if (!version) {
        version = await getLatestVersion();
    }

    const exe = process.platform === "win32" ? "pulumi.exe" : "pulumi";
    const root = cacheDir(version);
    const dest = path.join(root, "bin", exe);
    if (isExecutable(dest)) return dest;

    process.stderr.write(`Downloading pulumi v${version}...\n`);
    await install(version, root);
    return dest;
}

module.exports = { resolve, isExecutable };
