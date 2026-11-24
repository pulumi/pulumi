// Copyright 2026, Pulumi Corporation.  All rights reserved.

// Post-job step for the retry action.  Runs the wrapped action's post:
// entry point (if any) for cache saving, credential cleanup, etc., then
// removes the temporary directory.

"use strict";

const { spawnSync } = require("child_process");
const fs = require("fs");
const path = require("path");
const os = require("os");

function info(msg) {
  process.stdout.write(msg + os.EOL);
}
function warning(msg) {
  process.stdout.write(`::warning::${msg}${os.EOL}`);
}

function getState(name) {
  return process.env[`STATE_${name}`] || "";
}

function cleanup(tmpDir) {
  if (!tmpDir) return;
  try {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  } catch {
    // ignore
  }
}

function main() {
  const tmpDir = getState("tmpDir");
  const actionDir = getState("actionDir");
  const postScript = getState("postScript");
  const childEnvJson = getState("childEnv");

  // Run the wrapped action's post step if one was registered.
  if (actionDir && postScript) {
    const postPath = path.join(actionDir, postScript);
    if (fs.existsSync(postPath)) {
      // Reuse the same retry settings from the main step.
      const attemptLimit = parseInt(getState("attemptLimit") || "3", 10);
      const attemptDelay = parseInt(getState("attemptDelay") || "5000", 10);

      info(`Running post step: ${postScript}`);

      // Reconstruct the INPUT_* env vars the child action expects.
      const env = { ...process.env };
      if (childEnvJson) {
        try {
          const saved = JSON.parse(childEnvJson);
          for (const [key, value] of Object.entries(saved)) {
            env[key] = value;
          }
        } catch {
          warning("Failed to parse saved child environment");
        }
      }

      let lastErr;
      for (let attempt = 1; attempt <= attemptLimit; attempt++) {
        if (attempt > 1) {
          warning(`Post step attempt ${attempt - 1} failed: ${lastErr.message}`);
          info(
            `::group::Post retry attempt ${attempt}/${attemptLimit} after ${attemptDelay}ms`,
          );
          // spawnSync blocks, so use a sync sleep via spawnSync itself.
          spawnSync(process.execPath, [
            "-e",
            `setTimeout(()=>{},${attemptDelay})`,
          ]);
        }

        const res = spawnSync(process.execPath, [postPath], {
          env,
          stdio: "inherit",
          cwd: process.env.GITHUB_WORKSPACE || process.cwd(),
        });

        if (res.status === 0) {
          if (attempt > 1) {
            info("::endgroup::");
            info(`Post step succeeded on attempt ${attempt}/${attemptLimit}`);
          }
          lastErr = null;
          break;
        }

        lastErr = new Error(
          `Exit code ${res.status}${res.signal ? ` (signal: ${res.signal})` : ""}`,
        );
        if (attempt > 1) info("::endgroup::");
      }

      if (lastErr) {
        warning(
          `Post step failed after ${attemptLimit} attempts: ${lastErr.message}`,
        );
      }
    } else {
      warning(`Post script not found: ${postPath}`);
    }
  }

  cleanup(tmpDir);
}

main();
