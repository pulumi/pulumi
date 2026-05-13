// Copyright 2026, Pulumi Corporation. All rights reserved.

"use strict";

const { describe, it, beforeEach, afterEach } = require("node:test");
const assert = require("node:assert/strict");
const path = require("path");
const { cacheDir } = require("../lib/cache");

describe("cacheDir()", () => {
    let savedNpmConfigCache;

    beforeEach(() => {
        savedNpmConfigCache = process.env.npm_config_cache;
    });

    afterEach(() => {
        if (savedNpmConfigCache !== undefined) process.env.npm_config_cache = savedNpmConfigCache;
        else delete process.env.npm_config_cache;
    });

    it("uses npm_config_cache when set", () => {
        process.env.npm_config_cache = "/npm-cache";
        assert.equal(cacheDir("3.99.0"), path.join("/npm-cache", "_pulumi", "3.99.0"));
    });

    it("falls back to package directory when npm_config_cache is not set", () => {
        delete process.env.npm_config_cache;
        const dir = cacheDir("3.99.0");
        assert.ok(dir.includes("_pulumi"), `expected _pulumi in path, got ${dir}`);
        assert.ok(dir.includes("3.99.0"), `expected version in path, got ${dir}`);
    });

    it("different versions produce different paths", () => {
        process.env.npm_config_cache = "/npm-cache";
        assert.notEqual(cacheDir("3.1.0"), cacheDir("3.2.0"));
    });
});
