// Copyright 2026, Pulumi Corporation. All rights reserved.

"use strict";

const { describe, it } = require("node:test");
const assert = require("node:assert/strict");
const { os, arch, exeName, archiveExt } = require("../lib/platform");

describe("os()", () => {
    it("maps darwin", () => {
        const orig = process.platform;
        Object.defineProperty(process, "platform", { value: "darwin", configurable: true });
        assert.equal(os(), "darwin");
        Object.defineProperty(process, "platform", { value: orig, configurable: true });
    });

    it("maps linux", () => {
        const orig = process.platform;
        Object.defineProperty(process, "platform", { value: "linux", configurable: true });
        assert.equal(os(), "linux");
        Object.defineProperty(process, "platform", { value: orig, configurable: true });
    });

    it("maps win32 to windows", () => {
        const orig = process.platform;
        Object.defineProperty(process, "platform", { value: "win32", configurable: true });
        assert.equal(os(), "windows");
        Object.defineProperty(process, "platform", { value: orig, configurable: true });
    });

    it("throws on unsupported platform", () => {
        const orig = process.platform;
        Object.defineProperty(process, "platform", { value: "freebsd", configurable: true });
        assert.throws(() => os(), /Unsupported platform/);
        Object.defineProperty(process, "platform", { value: orig, configurable: true });
    });
});

describe("arch()", () => {
    it("maps x64", () => {
        const orig = process.arch;
        Object.defineProperty(process, "arch", { value: "x64", configurable: true });
        assert.equal(arch(), "x64");
        Object.defineProperty(process, "arch", { value: orig, configurable: true });
    });

    it("maps arm64", () => {
        const orig = process.arch;
        Object.defineProperty(process, "arch", { value: "arm64", configurable: true });
        assert.equal(arch(), "arm64");
        Object.defineProperty(process, "arch", { value: orig, configurable: true });
    });

    it("throws on unsupported arch", () => {
        const orig = process.arch;
        Object.defineProperty(process, "arch", { value: "mips", configurable: true });
        assert.throws(() => arch(), /Unsupported architecture/);
        Object.defineProperty(process, "arch", { value: orig, configurable: true });
    });
});

describe("exeName()", () => {
    it("returns pulumi for non-windows", () => {
        assert.equal(exeName("darwin"), "pulumi");
        assert.equal(exeName("linux"), "pulumi");
    });

    it("returns pulumi.exe for windows", () => {
        assert.equal(exeName("windows"), "pulumi.exe");
    });
});

describe("archiveExt()", () => {
    it("returns tar.gz for non-windows", () => {
        assert.equal(archiveExt("darwin"), "tar.gz");
        assert.equal(archiveExt("linux"), "tar.gz");
    });

    it("returns zip for windows", () => {
        assert.equal(archiveExt("windows"), "zip");
    });
});
