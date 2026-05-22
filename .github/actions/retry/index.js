// Copyright 2026, Pulumi Corporation.  All rights reserved.

// Retry wrapper for GitHub Actions — zero npm dependencies.
// Downloads a target action, reads its action.yml for the Node.js entry
// point, then executes it with retry logic.  If the wrapped action has a
// post: step (used for caching, credential cleanup, etc.), state is saved
// so that post.js can run it at the end of the job.

"use strict";

const { spawnSync, execSync } = require("child_process");
const fs = require("fs");
const path = require("path");
const os = require("os");

// ── GitHub Actions helpers (no @actions/core dependency) ────────────────────

function getInput(name) {
  return (process.env[`INPUT_${name.toUpperCase()}`] || "").trim();
}

function info(msg) {
  process.stdout.write(msg + os.EOL);
}
function warning(msg) {
  process.stdout.write(`::warning::${msg}${os.EOL}`);
}
function logError(msg) {
  process.stdout.write(`::error::${msg}${os.EOL}`);
}

function saveState(name, value) {
  const filePath = process.env.GITHUB_STATE;
  if (!filePath) return;
  const delim = `ghadelim_${Date.now()}_${Math.random().toString(36).slice(2)}`;
  fs.appendFileSync(
    filePath,
    `${name}<<${delim}${os.EOL}${String(value)}${os.EOL}${delim}${os.EOL}`,
  );
}

// ── Minimal YAML helpers ────────────────────────────────────────────────────
// Just enough to parse action.yml and flat key-value `with:` inputs.

/** Parse a flat YAML mapping (key: value) with support for block scalars (|). */
function parseFlat(text) {
  const result = {};
  const lines = text.split("\n");
  let i = 0;

  while (i < lines.length) {
    const line = lines[i];
    if (line.trim() === "" || line.trim().startsWith("#")) {
      i++;
      continue;
    }

    const m = line.match(/^([a-zA-Z_][a-zA-Z0-9_.-]*)\s*:\s*(.*)/);
    if (!m) {
      i++;
      continue;
    }

    const key = m[1];
    const rawVal = m[2].trim();

    if (rawVal === "|" || rawVal === "|-" || rawVal === ">") {
      const chomp = rawVal === "|-";
      const contentLines = [];
      let contentIndent = -1;
      i++;
      while (i < lines.length) {
        const next = lines[i];
        if (next.trim() === "") {
          contentLines.push("");
          i++;
          continue;
        }
        const ni = next.search(/\S/);
        if (ni <= 0) break;
        if (contentIndent < 0) contentIndent = ni;
        contentLines.push(next.slice(contentIndent));
        i++;
      }
      while (
        contentLines.length &&
        contentLines[contentLines.length - 1] === ""
      )
        contentLines.pop();
      result[key] = contentLines.join("\n") + (chomp ? "" : "\n");
      continue;
    }

    let val = rawVal;
    if (/^(['"]).*\1$/.test(val)) val = val.slice(1, -1);
    result[key] = val;
    i++;
  }
  return result;
}

/** Extract runs.{using,main,post} and input defaults from an action.yml. */
function parseActionYml(text) {
  const result = { main: null, using: null, post: null, inputs: {} };
  const lines = text.split("\n");
  let section = null; // "runs" | "inputs"
  let currentInput = null;
  let currentInputIndent = -1;

  for (const line of lines) {
    if (line.trim() === "" || line.trim().startsWith("#")) continue;
    const indent = line.search(/\S/);

    // Top-level key
    if (indent === 0) {
      if (line.startsWith("runs:")) section = "runs";
      else if (line.startsWith("inputs:")) section = "inputs";
      else section = null;
      currentInput = null;
      continue;
    }

    if (section === "runs") {
      const m = line
        .trim()
        .match(/^(using|main|post)\s*:\s*['"]?([^'"#]+)['"]?/);
      if (m) result[m[1].trim()] = m[2].trim();
    }

    if (section === "inputs") {
      if (currentInput === null || indent <= currentInputIndent) {
        const m = line.trim().match(/^([a-zA-Z_][a-zA-Z0-9_-]*)\s*:/);
        if (m) {
          currentInput = m[1];
          currentInputIndent = indent;
          result.inputs[currentInput] = {};
        }
      } else if (indent > currentInputIndent) {
        const m = line.trim().match(/^default\s*:\s*(.*)/);
        if (m) {
          let val = m[1].trim();
          // YAML null values should not become string defaults.
          if (/^(null|~|Null|NULL)$/.test(val) || val === "") continue;
          if (/^(['"]).*\1$/.test(val)) val = val.slice(1, -1);
          result.inputs[currentInput].default = val;
        }
      }
    }
  }
  return result;
}

// ── Expression evaluator ────────────────────────────────────────────────────
// Handles simple ${{ github.xxx }} by mapping to GITHUB_* env vars.

function evaluateExpr(value) {
  if (!value || !value.includes("${{")) return { value, ok: true };

  const evaluated = value.replace(
    /\$\{\{\s*github\.(\w+)\s*\}\}/g,
    (_, key) => {
      const envKey = `GITHUB_${key.toUpperCase()}`;
      return process.env[envKey] || "";
    },
  );

  // If there are still unevaluated expressions, mark as not fully resolved.
  return { value: evaluated, ok: !evaluated.includes("${{") };
}

// ── Download & extract ──────────────────────────────────────────────────────

async function downloadAction(owner, repo, ref) {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "retry-action-"));
  const tarPath = path.join(tmpDir, "action.tar.gz");
  const extractDir = path.join(tmpDir, "src");
  fs.mkdirSync(extractDir);

  const url = `https://github.com/${owner}/${repo}/archive/${ref}.tar.gz`;
  info(`Downloading ${owner}/${repo}@${ref}`);

  const headers = { "User-Agent": "pulumi/retry-action" };
  const token = process.env.GITHUB_TOKEN;
  if (token) headers["Authorization"] = `token ${token}`;

  const resp = await fetch(url, { redirect: "follow", headers });
  if (!resp.ok)
    throw new Error(`Download failed: ${resp.status} ${resp.statusText}`);

  fs.writeFileSync(tarPath, Buffer.from(await resp.arrayBuffer()));
  // On Windows, convert backslash paths to forward slashes for tar (MSYS2/Git tar
  // interprets backslashes as escape characters with --force-local).
  const tarFile =
    process.platform === "win32" ? tarPath.replace(/\\/g, "/") : tarPath;
  const tarDest =
    process.platform === "win32" ? extractDir.replace(/\\/g, "/") : extractDir;
  execSync(`tar -xzf "${tarFile}" -C "${tarDest}"`, { stdio: "pipe" });

  const entries = fs.readdirSync(extractDir);
  if (!entries.length) throw new Error("Archive was empty");

  return { dir: path.join(extractDir, entries[0]), tmpDir };
}

// ── Build the environment for the child action ──────────────────────────────

function buildChildEnv(meta, withInput) {
  const env = { ...process.env };

  // Remove our own inputs so they don't leak to the child.
  delete env["INPUT_ACTION"];
  delete env["INPUT_WITH"];
  delete env["INPUT_ATTEMPT_LIMIT"];
  delete env["INPUT_ATTEMPT_DELAY"];
  delete env["INPUT_GITHUB_TOKEN"];

  // Ensure GITHUB_TOKEN is available for evaluating ${{ github.token }} defaults.
  // Set on both process.env (so evaluateExpr can resolve it) and env (child inherits it).
  const githubToken = getInput("github_token");
  if (githubToken && !process.env["GITHUB_TOKEN"]) {
    process.env["GITHUB_TOKEN"] = githubToken;
    env["GITHUB_TOKEN"] = githubToken;
  }

  // Apply defaults from the child action's action.yml.
  for (const [name, cfg] of Object.entries(meta.inputs)) {
    if (cfg.default === undefined) continue;
    const envKey = `INPUT_${name.toUpperCase()}`;
    if (envKey in env) continue; // caller already set it
    const { value, ok } = evaluateExpr(cfg.default);
    if (ok) env[envKey] = value;
  }

  // Apply caller-provided inputs (override defaults).
  if (withInput) {
    for (const [key, value] of Object.entries(parseFlat(withInput))) {
      env[`INPUT_${key.toUpperCase()}`] = value;
    }
  }

  return env;
}

// ── Main ────────────────────────────────────────────────────────────────────

async function main() {
  const actionRef = getInput("action");
  const withInput = getInput("with");
  const attemptLimit = parseInt(getInput("attempt_limit") || "3", 10);
  const attemptDelay = parseInt(getInput("attempt_delay") || "5000", 10);

  if (!actionRef) {
    logError('"action" input is required');
    process.exitCode = 1;
    return;
  }

  // Parse owner/repo@ref or owner/repo/subpath@ref
  const at = actionRef.lastIndexOf("@");
  if (at < 0) {
    logError("Action must include @ref (e.g. actions/checkout@v6)");
    process.exitCode = 1;
    return;
  }
  const fullPath = actionRef.slice(0, at);
  const ref = actionRef.slice(at + 1);
  const parts = fullPath.split("/");
  if (parts.length < 2) {
    logError("Action must be in owner/repo format");
    process.exitCode = 1;
    return;
  }
  const owner = parts[0];
  const repo = parts[1];
  const subPath = parts.slice(2).join("/");

  // Download & extract the action.
  const { dir: repoDir, tmpDir } = await downloadAction(owner, repo, ref);
  const actionDir = subPath ? path.join(repoDir, subPath) : repoDir;

  // Always save tmpDir so post.js can clean it up.
  saveState("tmpDir", tmpDir);

  try {
    // Read action.yml / action.yaml
    const ymlFile = ["action.yml", "action.yaml"]
      .map((f) => path.join(actionDir, f))
      .find((f) => fs.existsSync(f));
    if (!ymlFile) throw new Error(`No action.yml found in ${actionDir}`);

    const meta = parseActionYml(fs.readFileSync(ymlFile, "utf8"));
    if (!meta.main) throw new Error("No runs.main in action.yml");
    if (meta.using && !meta.using.startsWith("node"))
      throw new Error(
        `Unsupported runtime "${meta.using}": only Node.js actions are supported`,
      );

    const env = buildChildEnv(meta, withInput);

    // Tell the child action where its own files live.
    env["GITHUB_ACTION_PATH"] = actionDir;

    // If the wrapped action has a post step, save state so post.js can run it.
    if (meta.post) {
      saveState("actionDir", actionDir);
      saveState("postScript", meta.post);
      saveState("attemptLimit", String(attemptLimit));
      saveState("attemptDelay", String(attemptDelay));
      // Save the INPUT_* vars the child needs (post step runs in a fresh env).
      const inputVars = {};
      for (const [key, value] of Object.entries(env)) {
        if (key.startsWith("INPUT_")) inputVars[key] = value;
      }
      inputVars["GITHUB_ACTION_PATH"] = actionDir;
      saveState("childEnv", JSON.stringify(inputVars));
    }

    // Resolve entry point.
    const mainScript = path.join(actionDir, meta.main);
    if (!fs.existsSync(mainScript))
      throw new Error(`Entry point not found: ${meta.main}`);

    // ── Retry loop ──────────────────────────────────────────────────────
    let lastErr;
    for (let attempt = 1; attempt <= attemptLimit; attempt++) {
      if (attempt > 1) {
        warning(`Attempt ${attempt - 1} failed: ${lastErr.message}`);
        info(
          `::group::Retry attempt ${attempt}/${attemptLimit} after ${attemptDelay}ms`,
        );
        await new Promise((r) => setTimeout(r, attemptDelay));
      }

      const res = spawnSync(process.execPath, [mainScript], {
        env,
        stdio: "inherit",
        cwd: process.env.GITHUB_WORKSPACE || process.cwd(),
      });

      if (res.status === 0) {
        if (attempt > 1) {
          info("::endgroup::");
          info(`Succeeded on attempt ${attempt}/${attemptLimit}`);
        }
        return; // success — tmpDir left for post.js to clean up
      }

      lastErr = new Error(
        `Exit code ${res.status}${res.signal ? ` (signal: ${res.signal})` : ""}`,
      );
      if (attempt > 1) info("::endgroup::");
    }

    logError(`All ${attemptLimit} attempts failed. Last: ${lastErr.message}`);
    process.exitCode = 1;
  } catch (err) {
    logError(err.message);
    process.exitCode = 1;
  }
  // tmpDir cleanup is handled by post.js — don't delete here so the post
  // step can still run the wrapped action's post script.
}

main().catch((err) => {
  logError(`Unexpected error: ${err.message}`);
  process.exitCode = 1;
});
