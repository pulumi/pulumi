// Copyright 2026, Pulumi Corporation. All rights reserved.

"use strict";

const os = require("os");
const path = require("path");

// npmDefaultCache returns npm's default cache directory when npm_config_cache
// is not set in the environment. These are npm's own documented defaults:
// https://docs.npmjs.com/cli/v11/using-npm/config#cache
function npmDefaultCache() {
    if (process.platform === "win32") {
        return path.join(process.env.APPDATA || path.join(os.homedir(), "AppData", "Roaming"), "npm-cache");
    }
    return path.join(os.homedir(), ".npm");
}

// cacheDir returns the directory where the pulumi binaries for a specific
// version should be cached. Follows npm's _subdirectory naming convention
// (e.g. _npx, _cacache, _prebuilds) so binaries live alongside other
// npm-managed caches.
//
// When invoked via npx or an npm script, npm injects npm_config_cache into
// the environment — that is the authoritative cache path. Otherwise we fall
// back to npm's documented default so the cache location is consistent.
function cacheDir(version) {
    const base = process.env.npm_config_cache || npmDefaultCache();
    return path.join(base, "_pulumi", version);
}

module.exports = { cacheDir };
