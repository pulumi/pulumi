// Copyright 2026, Pulumi Corporation. All rights reserved.

"use strict";

const fs = require("fs");
const os = require("os");
const path = require("path");
const { execFileSync } = require("child_process");

const INSTALL_SH_URL = "https://get.pulumi.com/install.sh";
const INSTALL_PS1_URL = "https://get.pulumi.com/install.ps1";

async function defaultFetchText(url) {
    const res = await fetch(url);
    if (!res.ok) throw new Error(`HTTP ${res.status} fetching ${url}`);
    return res.text();
}

function defaultExecScript(scriptPath, args) {
    if (process.platform === "win32") {
        const ps = process.env.SystemRoot
            ? path.join(process.env.SystemRoot, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
            : "powershell.exe";
        execFileSync(ps, ["-NoProfile", "-InputFormat", "None", "-ExecutionPolicy", "Bypass", "-File", scriptPath, ...args], {
            stdio: "inherit",
        });
    } else {
        execFileSync("/bin/sh", [scriptPath, ...args], { stdio: "inherit" });
    }
}

// installCLI downloads and runs the official Pulumi install script, installing
// the CLI and all language hosts into {root}/bin/. Mirrors the approach used
// by the Automation API (sdk/nodejs/automation/cmd.ts installPosix/installWindows).
// IO functions are injectable for testing.
async function installCLI(
    version,
    root,
    { fetchText = defaultFetchText, execScript = defaultExecScript } = {},
) {
    const isWindows = process.platform === "win32";
    const scriptContent = await fetchText(isWindows ? INSTALL_PS1_URL : INSTALL_SH_URL);

    const ext = isWindows ? ".ps1" : ".sh";
    const scriptPath = path.join(os.tmpdir(), `pulumi-install-${process.pid}${ext}`);
    try {
        fs.writeFileSync(scriptPath, scriptContent, { mode: 0o700 });
        const args = isWindows
            ? ["-NoEditPath", "-InstallRoot", root, "-Version", version]
            : ["--no-edit-path", "--install-root", root, "--version", version];
        execScript(scriptPath, args);
    } finally {
        fs.rmSync(scriptPath, { force: true });
    }
}

// fetchLatestVersion returns the latest stable Pulumi release version string.
async function fetchLatestVersion(fetchText = defaultFetchText) {
    const text = await fetchText("https://api.pulumi.com/api/cli/version");
    const { latestVersion } = JSON.parse(text);
    return latestVersion.replace(/^v/, "");
}

module.exports = { installCLI, fetchLatestVersion };
