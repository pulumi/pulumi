#!/usr/bin/env node
// Copyright 2026, Pulumi Corporation. All rights reserved.

"use strict";

const path = require("path");
const { spawn } = require("child_process");
const { resolve } = require("./lib/resolve");

resolve()
    .then((bin) => {
        // Prepend the directory containing the CLI to PATH so that language
        // hosts (pulumi-language-nodejs, etc.) installed alongside it are found.
        const env = { ...process.env, PATH: path.dirname(bin) + path.delimiter + (process.env.PATH || "") };
        const child = spawn(bin, process.argv.slice(2), { stdio: "inherit", env });
        child.on("exit", (code, signal) => {
            if (signal) {
                // Propagate signal to the parent process.
                process.kill(process.pid, signal);
            } else {
                process.exit(code ?? 0);
            }
        });
        child.on("error", (err) => {
            process.stderr.write(`pulumi: failed to start: ${err.message}\n`);
            process.exit(1);
        });
    })
    .catch((err) => {
        process.stderr.write(`pulumi: ${err.message}\n`);
        process.exit(1);
    });
