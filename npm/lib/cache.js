// Copyright 2026, Pulumi Corporation. All rights reserved.

"use strict";

const path = require("path");

// cacheDir returns the directory where the pulumi binaries for a specific
// version should be cached. Follows npm's _subdirectory naming convention
// (e.g. _npx, _cacache, _prebuilds) so binaries live alongside other
// npm-managed caches.
//
// When invoked via npx or an npm script, npm injects npm_config_cache into
// the environment — that is the authoritative cache path. When running
// outside npm (e.g. after npm link), we fall back to the package directory
// itself so binaries are stored alongside the code that uses them.
function cacheDir(version) {
    const base = process.env.npm_config_cache || path.join(__dirname, "..");
    return path.join(base, "_pulumi", version);
}

module.exports = { cacheDir };
