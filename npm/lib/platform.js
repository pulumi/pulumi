// Copyright 2026, Pulumi Corporation. All rights reserved.

"use strict";

function os() {
    switch (process.platform) {
        case "darwin":
            return "darwin";
        case "linux":
            return "linux";
        case "win32":
            return "windows";
        default:
            throw new Error(`Unsupported platform: ${process.platform}`);
    }
}

function arch() {
    switch (process.arch) {
        case "x64":
            return "x64";
        case "arm64":
            return "arm64";
        default:
            throw new Error(`Unsupported architecture: ${process.arch}`);
    }
}

function exeName(targetOS) {
    return (targetOS ?? os()) === "windows" ? "pulumi.exe" : "pulumi";
}

function archiveExt(targetOS) {
    return (targetOS ?? os()) === "windows" ? "zip" : "tar.gz";
}

module.exports = { os, arch, exeName, archiveExt };
