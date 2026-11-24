// Copyright 2026, Pulumi Corporation. All rights reserved.

"use strict";

const { describe, it, beforeEach, afterEach } = require("node:test");
const assert = require("node:assert/strict");
const os = require("os");
const path = require("path");
const { cacheDir } = require("../lib/cache");

describe("cacheDir()", () => {
    let savedPulumiHome;

    beforeEach(() => { savedPulumiHome = process.env.PULUMI_HOME; });
    afterEach(() => {
        if (savedPulumiHome !== undefined) process.env.PULUMI_HOME = savedPulumiHome;
        else delete process.env.PULUMI_HOME;
    });

    it("uses ~/.pulumi/versions/{version} by default", () => {
        delete process.env.PULUMI_HOME;
        assert.equal(cacheDir("3.99.0"), path.join(os.homedir(), ".pulumi", "versions", "3.99.0"));
    });

    it("respects PULUMI_HOME", () => {
        process.env.PULUMI_HOME = "/custom/pulumi";
        assert.equal(cacheDir("3.99.0"), path.join("/custom/pulumi", "versions", "3.99.0"));
    });

    it("different versions produce different paths", () => {
        process.env.PULUMI_HOME = "/pulumi";
        assert.notEqual(cacheDir("3.1.0"), cacheDir("3.2.0"));
    });
});
