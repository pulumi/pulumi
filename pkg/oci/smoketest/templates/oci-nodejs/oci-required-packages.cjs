#!/usr/bin/env node
// oci-required-packages.cjs — generate the best-effort required-packages manifest
// baked into an OCI Node program image. It walks the installed node_modules for
// Pulumi plugin metadata (each dependency's package.json `pulumi` field — the same
// convention pulumi-language-nodejs reads to discover plugins) and writes a manifest
// {"plugins":[<PulumiPluginJSON>...]} to a well-known path. Each entry conforms to
// Pulumi's PulumiPluginJSON shape ({resource,name,version,server?,parameterization?})
// so the host parses it into the existing type and nothing is lost in translation.
//
// This is template-owned, on purpose: the per-package-manager knowledge (where deps
// live, how to read their metadata) belongs in the build, not in any language host.
// It runs inside the image build, where Node is present and jq is not, so it is pure
// Node with no dependencies.
//
// Best-effort: it reports what static inspection of the installed deps can see. Lazy
// discovery at RegisterResource time stays authoritative, so a missing or incomplete
// manifest is safe — it just means fewer provider images can be pre-fetched.
//
// Usage: node oci-required-packages.cjs [node_modules_dir] [output_path]
//   node_modules_dir defaults to /workspace/node_modules
//   output_path, if omitted, writes the manifest to stdout
'use strict';
const fs = require('fs');
const path = require('path');

const nodeModules = process.argv[2] || '/workspace/node_modules';
const outPath = process.argv[3] || '';

// walkPackages yields every package.json path under root, descending into scoped
// (@scope) directories and nested node_modules (npm may leave deps un-hoisted).
function* walkPackages(root) {
  let entries;
  try {
    entries = fs.readdirSync(root, { withFileTypes: true });
  } catch {
    return; // no node_modules (or unreadable): nothing to report
  }
  for (const entry of entries) {
    if (!entry.isDirectory()) continue;
    const dir = path.join(root, entry.name);
    if (entry.name.startsWith('@')) {
      yield* walkPackages(dir); // scope dir: its children are the packages
      continue;
    }
    const pkgJson = path.join(dir, 'package.json');
    if (fs.existsSync(pkgJson)) yield pkgJson;
    const nested = path.join(dir, 'node_modules');
    if (fs.existsSync(nested)) yield* walkPackages(nested);
  }
}

// pluginName derives the Pulumi plugin name exactly as pulumi-language-nodejs does
// (getPluginName): an explicit pulumi.name wins; otherwise an @pulumi-scoped package
// uses its simple (unscoped) name. A third-party package without pulumi.name cannot
// be named — the host errors there; we omit it instead (best-effort: better to miss a
// pre-fetch hint than to record a wrong, unresolvable name like "@vendor/thing").
function pluginName(pulumi, packageName) {
  if (pulumi.name) return pulumi.name;
  if (packageName && packageName.startsWith('@pulumi/')) {
    return packageName.slice(packageName.indexOf('/') + 1);
  }
  return '';
}

// Dedup by name@version: hoisting can surface the same package from several paths.
const found = new Map();
for (const pkgJsonPath of walkPackages(nodeModules)) {
  let info;
  try {
    info = JSON.parse(fs.readFileSync(pkgJsonPath, 'utf8'));
  } catch {
    continue; // a malformed package.json is not worth failing the build over
  }
  const pulumi = info && info.pulumi;
  // Only packages that declare an associated resource plugin are provider packages.
  if (!pulumi || pulumi.resource !== true) continue;
  const name = pluginName(pulumi, info.name);
  if (!name) continue;
  const version = pulumi.version || info.version || '';
  // Emit a PulumiPluginJSON: resolve name/version (Node derives them from the package
  // when the pulumi block omits them), pass server/parameterization through verbatim.
  const entry = { resource: true, name };
  if (version) entry.version = version;
  if (pulumi.server) entry.server = pulumi.server;
  if (pulumi.parameterization) entry.parameterization = pulumi.parameterization;
  found.set(`${name}@${version}`, entry);
}

const plugins = [...found.values()].sort((a, b) =>
  a.name < b.name ? -1
    : a.name > b.name ? 1
      : (a.version || '').localeCompare(b.version || ''));
const manifest = JSON.stringify({ plugins }, null, 2) + '\n';

if (outPath) {
  fs.mkdirSync(path.dirname(outPath), { recursive: true });
  fs.writeFileSync(outPath, manifest);
  process.stderr.write(`oci-required-packages: wrote ${plugins.length} plugin(s) to ${outPath}\n`);
} else {
  process.stdout.write(manifest);
}
