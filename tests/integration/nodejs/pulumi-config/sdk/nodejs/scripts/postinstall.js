const fs = require("node:fs");
const path = require("node:path")
const process = require("node:process")
const { execSync } = require('node:child_process');
try {
  const out = execSync('tsc')
  console.log(out.toString())
} catch (error) {
  console.error(error.message + ": " + error.stdout.toString() + "\n" + error.stderr.toString())
  process.exit(1)
}
// TypeScript is compiled to "./bin", copy package.json to that directory so it can be read in "getVersion".
fs.copyFileSync(path.join(__dirname, "..", "package.json"), path.join(__dirname, "..", "bin", "package.json"));
