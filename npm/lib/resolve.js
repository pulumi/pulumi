// Copyright 2026, Pulumi Corporation. All rights reserved.

"use strict";

const fs = require("fs");
const path = require("path");
const { os: currentOS, arch: currentArch, exeName } = require("./platform");
const { cacheDir } = require("./cache");
const { downloadBinary, fetchLatestVersion } = require("./download");

const pkg = require("../package.json");

// Canonical path of this package's entry point. Used by isExecutable to skip
// our own wrapper when a global `npm install` symlinks it onto PATH, which
// would otherwise cause infinite recursion.
const SELF = fs.realpathSync(path.resolve(__dirname, "..", "run.js"));

// isExecutable returns true if filePath is a non-empty executable that does
// not resolve to this package's own entry point.
function isExecutable(filePath) {
    try {
        fs.accessSync(filePath, fs.constants.X_OK);
        return fs.realpathSync(filePath) !== SELF && fs.statSync(filePath).size > 0;
    } catch {
        return false;
    }
}

// findSystemPulumi searches PATH for a native pulumi binary, skipping any
// path component that contains "node_modules" to avoid calling ourselves.
// pathEnv defaults to process.env.PATH and is injectable for testing.
function findSystemPulumi(pathEnv) {
    const search = pathEnv !== undefined ? pathEnv : process.env.PATH || "";
    const exe = exeName();
    for (const dir of search.split(path.delimiter)) {
        if (!dir || dir.includes("node_modules")) continue;
        const candidate = path.join(dir, exe);
        if (isExecutable(candidate)) return candidate;
    }
    return null;
}

// resolve returns the path to the pulumi binary to invoke, using the
// following priority:
//   1. A native pulumi binary found on PATH (defers to existing installs).
//   2. A version-pinned binary in the local cache.
//   3. A freshly downloaded binary (cached for future invocations).
//
// IO functions are injectable for testing.
async function resolve(
    {
        pathEnv,
        version = process.env.PULUMI_VERSION || pkg.version,
        targetOS = currentOS(),
        targetArch = currentArch(),
        download = downloadBinary,
        getLatestVersion = fetchLatestVersion,
    } = {},
) {
    const system = findSystemPulumi(pathEnv);
    if (system) return system;

    if (!version) {
        version = await getLatestVersion();
    }

    const dest = path.join(cacheDir(version), exeName(targetOS));
    if (isExecutable(dest)) return dest;

    process.stderr.write(`Downloading pulumi v${version}...\n`);
    await download(version, targetOS, targetArch, dest);
    return dest;
}

module.exports = { resolve, findSystemPulumi, isExecutable };
