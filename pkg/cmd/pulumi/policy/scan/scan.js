// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// The `pulumi policy scan` driver.
//
// Runs a Pulumi Node.js program in-process against a recording resource monitor (no engine, no providers, no
// credentials) and writes the resource graph it registers, as a preview would see it: inputs as the SDK sends them,
// known inputs echoed back as outputs, and every other output unknown.
//
// The program's own dependencies don't need to be installed. `@pulumi/pulumi` always resolves to the SDK given with --sdk,
// other `@pulumi/*` packages to a shared cache (installed on demand with --ignore-scripts), anything else to the
// program's node_modules if present, and otherwise to an inert stub.
//
// Usage: node scan.js --program <dir> --sdk <@pulumi/pulumi dir> --cache <dir> --project <name> --stack <name>
//                     [--main <entry>] [--config <json file>] --out <graph.json>
"use strict";

const fs = require("fs");
const path = require("path");
const Module = require("module");
const childProcess = require("child_process");

const t0 = Date.now();
const timings = {};
function mark(name, since) {
    timings[name] = Date.now() - since;
}

function parseArgs(argv) {
    const args = {};
    for (let i = 0; i < argv.length; i++) {
        if (argv[i].startsWith("--")) {
            args[argv[i].slice(2)] = argv[i + 1];
            i++;
        }
    }
    return args;
}

const args = parseArgs(process.argv.slice(2));
const programDir = fs.realpathSync(path.resolve(args.program));
const sdkDir = path.resolve(args.sdk);
const cacheDir = path.resolve(args.cache);
const project = args.project;
const stack = args.stack || "scan";
const outFile = args.out;

const sdkRequire = Module.createRequire(path.join(sdkDir, "package.json"));
// The scan state the SDK shares with this driver (runtime/policyScan.ts).
const scanState = () => sdkRequire("./runtime/policyScan").policyScanState();

// ---------------------------------------------------------------------------------------------------------------
// Workspace: pnpm workspaces (catalog: versions, workspace: packages) and local file: packages.
// ---------------------------------------------------------------------------------------------------------------

function readJSON(f) {
    try {
        return JSON.parse(fs.readFileSync(f, "utf8"));
    } catch {
        return undefined;
    }
}

// A small reader for the two pnpm-workspace.yaml sections we need: `packages:` (a list) and `catalog:` (a map).
function readPnpmWorkspace(f) {
    const out = { packages: [], catalog: {} };
    let section = "";
    for (const raw of fs.readFileSync(f, "utf8").split("\n")) {
        const line = raw.replace(/\s+#.*$/, "");
        if (!line.trim()) {
            continue;
        }
        if (!/^\s/.test(line)) {
            section = line.replace(/:.*$/, "").trim();
            continue;
        }
        const unq = (v) => v.trim().replace(/^['"]|['"]$/g, "");
        if (section === "packages" && /^\s*-\s*/.test(line)) {
            out.packages.push(unq(line.replace(/^\s*-\s*/, "")));
        } else if (section === "catalog") {
            const m = /^\s*(['"]?[^'":]+['"]?)\s*:\s*(.+)$/.exec(line);
            if (m) {
                out.catalog[unq(m[1])] = unq(m[2]);
            }
        }
    }
    return out;
}

function expandGlob(root, pattern) {
    if (pattern.startsWith("!")) {
        return [];
    }
    let dirs = [root];
    for (const part of pattern.split("/")) {
        const next = [];
        for (const d of dirs) {
            if (part === "*" || part === "**") {
                let entries = [];
                try {
                    entries = fs.readdirSync(d, { withFileTypes: true });
                } catch {
                    // Not a directory.
                }
                for (const e of entries) {
                    if (e.isDirectory() && e.name !== "node_modules" && !e.name.startsWith(".")) {
                        next.push(path.join(d, e.name));
                        if (part === "**") {
                            next.push(...expandGlob(path.join(d, e.name), "**"));
                        }
                    }
                }
            } else {
                next.push(path.join(d, part));
            }
        }
        dirs = next;
    }
    return dirs;
}

function findWorkspace() {
    for (let d = programDir; ; d = path.dirname(d)) {
        const f = path.join(d, "pnpm-workspace.yaml");
        if (fs.existsSync(f)) {
            return Object.assign({ root: d }, readPnpmWorkspace(f));
        }
        const pkg = readJSON(path.join(d, "package.json"));
        if (pkg && pkg.workspaces) {
            const globs = Array.isArray(pkg.workspaces) ? pkg.workspaces : pkg.workspaces.packages || [];
            return { root: d, packages: globs, catalog: {} };
        }
        if (path.dirname(d) === d) {
            return undefined;
        }
    }
}

const workspace = findWorkspace();
// TypeScript tooling is installed and registered only for programs that need it.
const isTypeScript = (f) => /\.[cm]?tsx?$/.test(f) && !/\.d\.[cm]?ts$/.test(f);
const usesTypeScript =
    fs.existsSync(path.join(programDir, "tsconfig.json")) ||
    isTypeScript(args.main || "") ||
    fs.readdirSync(programDir).some(isTypeScript);
// The program's own code: the workspace (shared packages included), or just the program directory.
const codeRoot = workspace ? workspace.root : programDir;
// Local packages by name: workspace members and file:/link: dependencies.
const localPackages = new Map();
if (workspace) {
    for (const g of workspace.packages) {
        for (const d of expandGlob(workspace.root, g)) {
            const pkg = readJSON(path.join(d, "package.json"));
            if (pkg && pkg.name && !localPackages.has(pkg.name)) {
                localPackages.set(pkg.name, d);
            }
        }
    }
}
function addFileDeps(dir) {
    const pkg = readJSON(path.join(dir, "package.json")) || {};
    for (const [name, range] of Object.entries(Object.assign({}, pkg.devDependencies, pkg.dependencies))) {
        const m = /^(?:file|link):(.*)$/.exec(String(range));
        if (m && !localPackages.has(name)) {
            localPackages.set(name, path.resolve(dir, m[1]));
            addFileDeps(path.resolve(dir, m[1]));
        }
    }
}
addFileDeps(programDir);
for (const d of [...localPackages.values()]) {
    addFileDeps(d);
}

// Hermetic by default (PULUMI_POLICY_SCAN_ALLOW_NETWORK=1 turns it off).
const allowNetwork = !!process.env.PULUMI_POLICY_SCAN_ALLOW_NETWORK;
const NETWORK_PACKAGES =
    /^(@pulumi\/esc-sdk|axios|node-fetch|got|ky|superagent|request|undici|octokit|@octokit\/.+|@aws-sdk\/.+|aws-sdk|@azure\/.+|@google-cloud\/.+|googleapis|@slack\/.+|stripe|@sentry\/node|pg|mysql2?|redis|ioredis|mongodb)$/;
const networkBlocked = new Set();
let fixtures = { invokes: {}, stackReferences: {} };

const resources = [];
const stubs = new Set();
const errors = [];
const warnings = [];

function writeGraph() {
    timings.total = Date.now() - t0;
    const state = globalThis.__pulumiPolicyScan || {};
    const sorted = (set) => [...(set || [])].sort();
    const graph = {
        project,
        stack,
        programDir,
        codeRoot,
        resources,
        stubs: [...stubs].sort(),
        missingConfig: sorted(state.missingConfig),
        assumedArtifacts: sorted(assumedArtifacts),
        networkBlocked: sorted(networkBlocked),
        usedFixtures: sorted(state.usedFixtures),
        unknownInvokes: sorted(state.unknownInvokes),
        probedApplies: state.probedApplies || 0,
        failedApplies: state.failedApplies || 0,
        errors,
        warnings,
        timings,
    };
    fs.writeFileSync(outFile, JSON.stringify(graph, null, 1));
}

// ---------------------------------------------------------------------------------------------------------------
// SDK cache. Tools (typescript, ts-node) live in <cache>/tools. Provider SDKs live in one directory per set of
// "package@major" the program needs (<cache>/sets/<hash>), so programs pinned to different majors don't fight
// over one node_modules; within a set, later programs reuse what's there when it's the same major and new enough.
// ---------------------------------------------------------------------------------------------------------------

const toolsDir = path.join(cacheDir, "tools");
let sdkSetDir = path.join(cacheDir, "sets", "empty");

function npmInstall(dir, specs) {
    fs.mkdirSync(dir, { recursive: true });
    if (!fs.existsSync(path.join(dir, "package.json"))) {
        fs.writeFileSync(path.join(dir, "package.json"), JSON.stringify({ name: "pulumi-policy-scan-cache", private: true }));
    }
    process.stderr.write(`policy scan: installing into the SDK cache: ${specs.join(" ")}\n`);
    childProcess.execFileSync(
        "npm",
        ["install", "--ignore-scripts", "--no-audit", "--no-fund", "--loglevel=error", "--legacy-peer-deps", ...specs],
        { cwd: dir, stdio: ["ignore", "inherit", "inherit"] },
    );
}

function ensureCache() {
    const since = Date.now();
    const semver = sdkRequire("semver");

    const tools = usesTypeScript ? { typescript: "^5.0.0", "ts-node": "^10.9.0" } : {};
    const missingTools = Object.entries(tools)
        .filter(([name]) => !fs.existsSync(path.join(toolsDir, "node_modules", name, "package.json")))
        .map(([n, r]) => `${n}@${r}`);
    if (missingTools.length > 0) {
        npmInstall(toolsDir, missingTools);
    }

    // The program's @pulumi/* dependencies, and those of the local packages it uses (transitively).
    const wanted = {};
    const seen = new Set();
    const visit = (dir) => {
        if (seen.has(dir)) {
            return;
        }
        seen.add(dir);
        const pkg = readJSON(path.join(dir, "package.json")) || {};
        for (const [name, r] of Object.entries(Object.assign({}, pkg.devDependencies, pkg.dependencies))) {
            let range = String(r);
            if (localPackages.has(name)) {
                visit(localPackages.get(name));
                continue;
            }
            // @pulumi/pulumi comes from --sdk.
            if (!name.startsWith("@pulumi/") || name === "@pulumi/pulumi" || name === "@pulumi/policy") {
                continue;
            }
            if (range.startsWith("catalog:")) {
                range = (workspace && workspace.catalog[name]) || "latest";
            }
            if (/^(file:|link:|workspace:|\.|\/)/.test(range)) {
                continue;
            }
            if (!wanted[name]) {
                wanted[name] = range;
            }
        }
    };
    visit(programDir);

    const minOf = (range) => {
        try {
            return semver.minVersion(range);
        } catch {
            return null;
        }
    };
    const key = Object.entries(wanted)
        .map(([n, r]) => `${n}@${minOf(r) ? semver.major(minOf(r)) : r}`)
        .sort()
        .join(",");
    sdkSetDir = path.join(cacheDir, "sets", require("crypto").createHash("sha256").update(key).digest("hex").slice(0, 12));
    // Same major and at least the version asked for: good enough for a scan.
    const usable = (name, range) => {
        const pkg = readJSON(path.join(sdkSetDir, "node_modules", name, "package.json"));
        if (!pkg) {
            return false;
        }
        const min = minOf(range);
        return !min || semver.satisfies(pkg.version, range) || (semver.major(pkg.version) === semver.major(min) && semver.gte(pkg.version, min));
    };
    const missing = Object.entries(wanted)
        .filter(([n, r]) => !usable(n, r))
        .map(([n, r]) => `${n}@${r}`);
    if (missing.length > 0) {
        npmInstall(sdkSetDir, missing);
    }
    cacheParent = cacheParentFor(sdkSetDir);
    mark("cache", since);
}

// ---------------------------------------------------------------------------------------------------------------
// Module resolution.
// ---------------------------------------------------------------------------------------------------------------

const origResolve = Module._resolveFilename;
// Where cached SDKs resolve from (set by ensureCache, before the program loads anything).
const cacheParentFor = (dir) => ({ id: path.join(dir, "index.js"), filename: path.join(dir, "index.js"), paths: Module._nodeModulePaths(dir) });
let cacheParent = cacheParentFor(sdkSetDir);
const STUB_PREFIX = path.join(path.sep, "__pulumi_policy_scan_stub__");

function makeStub(name) {
    const target = function () {};
    return new Proxy(target, {
        get(_t, p) {
            if (p === "__esModule" || p === Symbol.for("pulumi.policyScan.unknown")) {
                // The SDK serializes a stub as an unknown (runtime/policyScan.ts).
                return true;
            }
            if (p === "then" || p === "toJSON") {
                return undefined;
            }
            if (p === "length") {
                return 0;
            }
            if (p === Symbol.iterator) {
                return function* () {};
            }
            if (p === Symbol.toPrimitive || p === "toString" || p === "valueOf") {
                return () => `<stub ${name}>`;
            }
            if (typeof p === "symbol") {
                return undefined;
            }
            return makeStub(`${name}.${String(p)}`);
        },
        apply() {
            return makeStub(`${name}()`);
        },
        construct() {
            return makeStub(`new ${name}`);
        },
    });
}

function stubFor(request) {
    const filename = path.join(STUB_PREFIX, request);
    if (!Module._cache[filename]) {
        const m = new Module(filename);
        m.filename = filename;
        m.loaded = true;
        m.exports = makeStub(request);
        Module._cache[filename] = m;
        stubs.add(request);
    }
    return filename;
}

function isBare(request) {
    return !(
        request.startsWith(".") ||
        request.startsWith("/") ||
        request.startsWith("node:") ||
        Module.isBuiltin(request) ||
        path.isAbsolute(request)
    );
}

Module._resolveFilename = function (request, parent, isMain, options) {
    if (request.startsWith(STUB_PREFIX)) {
        return request;
    }
    if (request === "@pulumi/pulumi" || request.startsWith("@pulumi/pulumi/")) {
        return origResolve.call(this, sdkDir + request.slice("@pulumi/pulumi".length), parent, isMain, options);
    }
    if (!isBare(request)) {
        try {
            return origResolve.call(this, request, parent, isMain, options);
        } catch (err) {
            // ESM-style TypeScript imports name the emitted file: "./acl.js" is ./acl.ts on disk.
            const m = /^(.*)\.(m|c)?js$/.exec(request);
            if (m && (request.startsWith(".") || path.isAbsolute(request))) {
                for (const ext of [".ts", ".tsx", `.${m[2] || ""}ts`]) {
                    try {
                        return origResolve.call(this, m[1] + ext, parent, isMain, options);
                    } catch {
                        // Try the next extension.
                    }
                }
            }
            throw err;
        }
    }
    // Workspace members and file: packages resolve to their sources, installed or not.
    const pkgName = request.startsWith("@") ? request.split("/").slice(0, 2).join("/") : request.split("/")[0];
    if (localPackages.has(pkgName)) {
        try {
            return origResolve.call(this, localPackages.get(pkgName) + request.slice(pkgName.length), parent, isMain, options);
        } catch {
            // Fall through to the usual lookup.
        }
    }
    const from = (parent && parent.filename) || "";
    const fromProgram = from.startsWith(codeRoot + path.sep) && !from.includes(`${path.sep}node_modules${path.sep}`);
    // Hermetic: the program's HTTP clients and cloud/SaaS SDKs are stubs, so their calls give unknowns instead of
    // reaching the network.
    if (fromProgram && !allowNetwork && NETWORK_PACKAGES.test(pkgName)) {
        networkBlocked.add(`module ${pkgName}`);
        return stubFor(request);
    }
    // The program's @pulumi/* imports come from the cache; libraries resolve their own (possibly nested) versions.
    const attempts = fromProgram && request.startsWith("@pulumi/") ? [cacheParent, parent] : [parent, cacheParent];
    let lastErr;
    for (const p of attempts) {
        try {
            return origResolve.call(this, request, p, isMain, options);
        } catch (err) {
            lastErr = err;
        }
    }
    // Only the program's own code gets stubs; libraries often probe optional modules in a try/catch.
    if (!fromProgram) {
        throw lastErr;
    }
    return stubFor(request);
};

// ESM default-imports a CommonJS module as the whole module (`import pulumi from "@pulumi/pulumi"`). Transpiled to
// CommonJS, that reads `.default`, which a module marked __esModule without a default export doesn't have: give it
// one, for modules the program's own code loads.
const origLoad = Module._load;
Module._load = function (request, parent, isMain) {
    const exp = origLoad.apply(this, arguments);
    const from = (parent && parent.filename) || "";
    if (
        exp &&
        (typeof exp === "object" || typeof exp === "function") &&
        exp.__esModule &&
        !("default" in exp) &&
        from.startsWith(codeRoot + path.sep) &&
        !from.includes(`${path.sep}node_modules${path.sep}`)
    ) {
        try {
            Object.defineProperty(exp, "default", { value: exp, enumerable: false, configurable: true });
        } catch {
            // Frozen exports: leave them.
        }
    }
    return exp;
};

// ---------------------------------------------------------------------------------------------------------------
// The recording monitor.
// ---------------------------------------------------------------------------------------------------------------

function loadMonitorClass() {
    const { MockMonitor } = sdkRequire("./runtime/mocks");
    const resproto = sdkRequire("./proto/resource_pb");
    const structproto = sdkRequire("google-protobuf/google/protobuf/struct_pb");

    class ScanMonitor extends MockMonitor {
        constructor() {
            super({
                newResource: () => ({ id: undefined, state: {} }),
                call: () => ({}),
            });
        }

        engineUrn(parent, type, name) {
            let qualified = type;
            if (parent) {
                const parentType = parent.split("::")[2];
                if (parentType && parentType !== "pulumi:pulumi:Stack") {
                    qualified = `${parentType}$${type}`;
                }
            }
            return `urn:pulumi:${stack}::${project}::${qualified}::${name}`;
        }

        record(req, urn, object, extra) {
            const propertyDependencies = {};
            if (req.getPropertydependenciesMap) {
                req.getPropertydependenciesMap().forEach((v, k) => {
                    propertyDependencies[k] = v.getUrnsList();
                });
            }
            // The resource's position in the program: for resources a library creates (components, awsx classic),
            // the innermost stack frame in the program's own code, not the library's.
            let sp = req.getSourceposition ? req.getSourceposition() : undefined;
            const inProgram = (pos) => {
                if (!pos || !pos.getUri()) {
                    return false;
                }
                let file;
                try {
                    file = require("url").fileURLToPath(pos.getUri());
                } catch {
                    return false;
                }
                return file.startsWith(codeRoot + path.sep) && !file.includes(`${path.sep}node_modules${path.sep}`);
            };
            if (!inProgram(sp) && req.getStacktrace && req.getStacktrace()) {
                const frame = req
                    .getStacktrace()
                    .getFramesList()
                    .map((f) => f.getPc())
                    .find(inProgram);
                if (frame) {
                    sp = frame;
                }
            }
            resources.push(
                Object.assign(
                    {
                        urn,
                        type: req.getType(),
                        name: req.getName(),
                        custom: req.getCustom ? req.getCustom() : true,
                        remote: req.getRemote ? req.getRemote() : false,
                        parent: req.getParent(),
                        dependencies: req.getDependenciesList ? req.getDependenciesList() : [],
                        propertyDependencies,
                        provider: req.getProvider(),
                        sourcePosition: sp
                            ? { uri: sp.getUri(), line: sp.getLine(), column: sp.getColumn() }
                            : undefined,
                        object: object ? object.toJavaScript() : {},
                    },
                    extra,
                ),
            );
            this.resources.set(urn, { urn, id: null, state: {} });
        }

        async registerResource(req, callback) {
            try {
                const urn = this.engineUrn(req.getParent(), req.getType(), req.getName());
                this.record(req, urn, req.getObject());
                const resp = new resproto.RegisterResourceResponse();
                resp.setUrn(urn);
                resp.setId("");
                // As a provider does in preview: known inputs come back as known outputs, the rest is unknown.
                resp.setObject(req.getCustom() ? req.getObject() : structproto.Struct.fromJavaScript({}));
                callback(null, resp);
            } catch (err) {
                callback(err, undefined);
            }
        }

        async readResource(req, callback) {
            try {
                const urn = this.engineUrn(req.getParent(), req.getType(), req.getName());
                this.record(req, urn, req.getProperties(), { read: true, id: req.getId() });
                const resp = new resproto.ReadResourceResponse();
                resp.setUrn(urn);
                let props = {};
                if (req.getType() === "pulumi:pulumi:StackReference") {
                    // Outputs by stack name: the full name, or its trailing segments ("network.production",
                    // "service-network/network.production").
                    const id = req.getId();
                    const refs = fixtures.stackReferences || {};
                    const key = Object.keys(refs).find((k) => id === k || id.endsWith(`/${k}`));
                    if (key !== undefined) {
                        scanState().usedFixtures.add(`stackReference ${key}`);
                        props = { name: id, outputs: refs[key], secretOutputNames: [] };
                    } else {
                        scanState().unknownInvokes.add(`stackReference ${id}`);
                    }
                }
                resp.setProperties(structproto.Struct.fromJavaScript(props));
                callback(null, resp);
            } catch (err) {
                callback(err, undefined);
            }
        }

        registerResourceOutputs(_req, callback) {
            callback(null, {});
        }

        // Hooks and component method calls: nothing runs them here.
        registerResourceHook(_req, callback) {
            callback(null, {});
        }

        registerErrorHook(_req, callback) {
            callback(null, {});
        }

        call(_req, callback) {
            const resp = new resproto.CallResponse ? new resproto.CallResponse() : {};
            callback(null, resp);
        }

        supportsFeature(_req, callback) {
            callback(null, { getHassupport: () => true });
        }

        getDeploymentInfo(_req, callback) {
            const resp = new resproto.DeploymentInfo();
            resp.setSupportedfeaturesList(
                Object.values(resproto.ResourceMonitorFeature).filter((v) => typeof v === "number" && v !== 0),
            );
            callback(null, resp);
        }
    }
    return ScanMonitor;
}

// ---------------------------------------------------------------------------------------------------------------
// Main.
// ---------------------------------------------------------------------------------------------------------------

function loadConfig() {
    if (!args.config) {
        return;
    }
    // { "<ns>:<key>": "<string value>", ... } with secure values already replaced by the caller.
    const cfg = JSON.parse(fs.readFileSync(args.config, "utf8"));
    process.env.PULUMI_CONFIG = JSON.stringify(cfg.values || {});
    process.env.PULUMI_CONFIG_SECRET_KEYS = JSON.stringify(cfg.secretKeys || []);
    fixtures = Object.assign({ invokes: {}, stackReferences: {} }, cfg.fixtures || {});
    // The SDK reads invoke fixtures from the shared scan state (runtime/policyScan.ts).
    scanState().invokes = fixtures.invokes || {};
}

function registerTypeScript() {
    const since = Date.now();
    const cacheRequire = Module.createRequire(path.join(toolsDir, "index.js"));
    const tsNode = cacheRequire("ts-node");
    tsNode.register({
        transpileOnly: true,
        skipProject: true,
        compiler: cacheRequire.resolve("typescript"),
        // Everything runs as CommonJS, whatever the package's "type": ESM programs (`"type": "module"`, .mjs/.mts)
        // are transpiled like the rest, so the resolve hook and the single SDK copy apply to them too.
        moduleTypes: { "**/*": "cjs" },
        // Only the program's own files (paths are relative to the code root, the working directory).
        ignore: ["(?:^|/)node_modules/", "^\\.\\./"],
        compilerOptions: {
            module: "commonjs",
            moduleResolution: "node",
            target: "es2022",
            esModuleInterop: true,
            experimentalDecorators: true,
            sourceMap: true,
            allowJs: true,
        },
    });
    // Top-level await (ESM programs) isn't valid CommonJS: such a module's body runs inside an async function
    // whose promise the driver awaits. The wrapper goes on the first line so source positions don't shift.
    const vm = require("vm");
    const origCompile = Module.prototype._compile;
    Module.prototype._compile = function (content, filename) {
        const own = filename.startsWith(codeRoot + path.sep) && !filename.includes(`${path.sep}node_modules${path.sep}`);
        if (own && content.includes("import.meta")) {
            // ESM's import.meta in CommonJS terms (else Node reloads the file as ESM, without `exports`).
            content = content.replace(
                /\bimport\.meta\b/g,
                '({ url: require("url").pathToFileURL(__filename).href, dirname: __dirname, filename: __filename, ' +
                    'resolve: (s) => require("url").pathToFileURL(require.resolve(s)).href })',
            );
        }
        if (own && /\bawait\b/.test(content)) {
            try {
                new vm.Script(Module.wrap(content), { filename });
            } catch (err) {
                if (err instanceof SyntaxError) {
                    content = `exports.__pulumiScanTLA = (async () => {${content}\n})();`;
                }
            }
        }
        return origCompile.call(this, content, filename);
    };
    // Program .js files written as ESM need the same treatment; library code in node_modules is left alone.
    for (const ext of [".mjs", ".mts", ".cjs", ".cts"]) {
        if (!require.extensions[ext]) {
            require.extensions[ext] = require.extensions[ext.endsWith("ts") ? ".ts" : ".js"];
        }
    }
    mark("typescript", since);
}

// A source checkout has no build outputs. Paths under dist/, build/ or out/ in the code root that don't exist
// are assumed built: they stat as an empty file (or directory, for readdir) modified now. Each one is reported.
const assumedArtifacts = new Set();
function assumeBuildArtifacts() {
    const isArtifact = (p) => {
        if (typeof p !== "string" && !(p instanceof URL)) {
            return false;
        }
        const abs = path.resolve(p instanceof URL ? require("url").fileURLToPath(p) : p);
        return (
            abs.startsWith(codeRoot + path.sep) &&
            !abs.includes(`${path.sep}node_modules${path.sep}`) &&
            new RegExp(`\\${path.sep}(dist|build|out)(\\${path.sep}|$)`).test(path.sep + path.relative(codeRoot, abs) + path.sep) &&
            !fs.existsSync.__orig(abs)
        );
    };
    const note = (p) => {
        assumedArtifacts.add(path.relative(codeRoot, path.resolve(String(p))));
    };
    const origExists = fs.existsSync;
    fs.existsSync = function (p) {
        if (isArtifact(p)) {
            note(p);
            return true;
        }
        return origExists.apply(this, arguments);
    };
    fs.existsSync.__orig = origExists;
    const fakeStat = (dir) => {
        const now = new Date();
        const st = Object.create(fs.Stats.prototype);
        Object.assign(st, { size: 0, mode: dir ? 0o40755 : 0o100644, mtimeMs: now.getTime(), mtime: now, ctime: now, atime: now, birthtime: now });
        st.isFile = () => !dir;
        st.isDirectory = () => dir;
        return st;
    };
    for (const name of ["statSync", "lstatSync"]) {
        const orig = fs[name];
        fs[name] = function (p) {
            if (isArtifact(p)) {
                note(p);
                return fakeStat(path.extname(String(p)) === "");
            }
            return orig.apply(this, arguments);
        };
    }
    const origReaddir = fs.readdirSync;
    fs.readdirSync = function (p) {
        if (isArtifact(p)) {
            note(p);
            return [];
        }
        return origReaddir.apply(this, arguments);
    };
    const origRead = fs.readFileSync;
    fs.readFileSync = function (p, opts) {
        if (isArtifact(p)) {
            note(p);
            return (typeof opts === "string" || (opts && opts.encoding)) ? "" : Buffer.alloc(0);
        }
        return origRead.apply(this, arguments);
    };
}

function blockNetwork() {
    const { EventEmitter } = require("events");
    const urlOf = (a) => {
        try {
            if (typeof a === "string" || a instanceof URL) {
                return new URL(String(a)).host;
            }
            return (a && (a.hostname || a.host)) || "?";
        } catch {
            return "?";
        }
    };
    for (const [modName, mod] of [["http", require("http")], ["https", require("https")]]) {
        for (const fn of ["request", "get"]) {
            mod[fn] = function (a) {
                networkBlocked.add(`${modName} ${urlOf(a)}`);
                const req = new EventEmitter();
                for (const m of ["end", "write", "setTimeout", "setHeader", "setNoDelay", "setSocketKeepAlive", "flushHeaders"]) {
                    req[m] = () => req;
                }
                req.destroy = req.abort = () => req;
                setImmediate(() => {
                    const err = new Error("policy scan: network access is disabled (set PULUMI_POLICY_SCAN_ALLOW_NETWORK=1)");
                    err.code = "ENETUNREACH";
                    req.emit("error", err);
                });
                return req;
            };
        }
    }
    const { scanUnknown } = sdkRequire("./runtime/policyScan");
    globalThis.fetch = async (input) => {
        networkBlocked.add(`fetch ${urlOf(typeof input === "object" && input.url ? input.url : input)}`);
        return {
            ok: true,
            status: 200,
            statusText: "OK",
            url: String((input && input.url) || input),
            headers: new Map(),
            json: async () => scanUnknown(),
            text: async () => scanUnknown(),
            arrayBuffer: async () => new ArrayBuffer(0),
            blob: async () => scanUnknown(),
            clone() {
                return this;
            },
        };
    };
}

// Hermetic: the program's child processes (builds, git, CLIs) don't run. Sync calls return empty output, async ones
// exit 0 with no output; each one is reported. Called after the SDK cache is ready (it uses npm).
function blockProcesses() {
    const cp = require("child_process");
    const { EventEmitter } = require("events");
    const { Readable, Writable } = require("stream");
    const note = (file, argv) => networkBlocked.add(`process ${[file, ...(argv || [])].join(" ").slice(0, 80)}`);
    const empty = (opts) => (opts && (typeof opts === "string" || opts.encoding) && opts.encoding !== "buffer" ? "" : Buffer.alloc(0));
    const argsOf = (a, b) => (Array.isArray(a) ? [a, b] : [[], a]);
    cp.execSync = (cmd, opts) => (note(cmd), empty(opts));
    cp.execFileSync = (file, a, b) => {
        const [argv, opts] = argsOf(a, b);
        note(file, argv);
        return empty(opts);
    };
    cp.spawnSync = (file, a, b) => {
        const [argv, opts] = argsOf(a, b);
        note(file, argv);
        const out = empty(opts);
        return { pid: 0, status: 0, signal: null, stdout: out, stderr: out, output: [null, out, out] };
    };
    const fakeChild = () => {
        const c = new EventEmitter();
        c.stdout = Readable.from([]);
        c.stderr = Readable.from([]);
        c.stdin = new Writable({ write: (_c, _e, cb) => cb() });
        c.pid = 0;
        c.kill = () => true;
        setImmediate(() => {
            c.emit("exit", 0, null);
            c.emit("close", 0, null);
        });
        return c;
    };
    cp.spawn = (file, a) => (note(file, Array.isArray(a) ? a : []), fakeChild());
    cp.execFile = (file, ...rest) => {
        note(file, Array.isArray(rest[0]) ? rest[0] : []);
        const cb = rest.find((x) => typeof x === "function");
        setImmediate(() => cb && cb(null, "", ""));
        return fakeChild();
    };
    cp.exec = (cmd, ...rest) => {
        note(cmd);
        const cb = rest.find((x) => typeof x === "function");
        setImmediate(() => cb && cb(null, "", ""));
        return fakeChild();
    };
}

async function main() {
    process.env.PULUMI_POLICY_SCAN = "1";
    if (!process.env.PULUMI_POLICY_SCAN_NO_ASSUMED_ARTIFACTS) {
        assumeBuildArtifacts();
    }
    if (!allowNetwork) {
        blockNetwork();
    }
    loadConfig();
    ensureCache();
    if (!allowNetwork) {
        blockProcesses();
    }
    // ts-node resolves its moduleTypes and ignore patterns against the working directory at registration: the
    // code root, so shared workspace packages are transpiled too. The program then runs in its own directory.
    if (usesTypeScript) {
        process.chdir(codeRoot);
        registerTypeScript();
    }
    process.chdir(programDir);

    let since = Date.now();
    const runtime = require("@pulumi/pulumi/runtime");
    const ScanMonitor = loadMonitorClass();
    runtime.setMockOptions(new ScanMonitor(), project, stack, true /* preview */, "organization");
    const { getStore } = require("@pulumi/pulumi/runtime/state");
    const store = getStore();
    for (const k of Object.keys(store)) {
        if (k.startsWith("supports")) {
            store[k] = true;
        }
    }
    mark("sdk", since);

    since = Date.now();
    const entry = path.resolve(programDir, args.main || ".");
    try {
        await runtime.runInPulumiStack(async () => {
            let m = require(entry.endsWith(path.sep) ? entry : entry + (args.main ? "" : path.sep));
            if (m && m.__pulumiScanTLA) {
                await m.__pulumiScanTLA;
                delete m.__pulumiScanTLA;
            }
            if (m && typeof m.default === "function" && Object.keys(m).length <= 2) {
                m = m.default;
            }
            if (typeof m === "function") {
                m = await m();
            }
            return m;
        });
        await runtime.waitForRPCs();
    } catch (err) {
        errors.push({ message: String((err && err.stack) || err) });
    }
    mark("program", since);
}

// The event loop ran dry with the program still pending (it awaits something no recorder can resolve): report
// what was registered so far rather than dying on the SDK's promise-leak check.
let finished = false;
process.on("beforeExit", () => {
    if (!finished) {
        finished = true;
        errors.push({ message: "the program did not finish: it waits on a value that is only known after deployment" });
        writeGraph();
        process.exit(0);
    }
});

process.on("unhandledRejection", (err) => {
    errors.push({ message: `unhandled rejection: ${String((err && err.stack) || err)}` });
});

main()
    .catch((err) => {
        errors.push({ message: String((err && err.stack) || err) });
    })
    .finally(() => {
        finished = true;
        writeGraph();
        // Pending timers or handles in the program must not keep the scan alive.
        process.exit(0);
    });
