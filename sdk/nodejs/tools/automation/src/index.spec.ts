// Copyright 2026-2026, Pulumi Corporation.
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

import * as childProcess from "child_process";
import * as fs from "fs";
import * as os from "os";
import * as path from "path";

// We are going to generate the code and then import it. This is a dynamic
// process, so we're on our own as far as types go. In practice, this is fine
// because the generated code _does_ have types, and we're not testing the
// actual production code in this file, so types wouldn't give us any more
// reassurance.
let generated: any;

beforeAll(() => {
    const root = path.resolve(process.cwd());
    const output = fs.mkdtempSync(path.join(os.tmpdir(), "automation-test-"));

    const specification = path.join(output, "specification.json");
    const boilerplate = path.resolve(root, "test", "boilerplate.ts");

    const cli = childProcess.execFileSync("pulumi", ["generate-cli-spec"], {
        cwd: root,
        encoding: "utf8",
        stdio: ["ignore", "pipe", "ignore"],
    });
    fs.writeFileSync(specification, cli);

    const args = ["start", "--", specification, "--boilerplate", boilerplate, "--output", output];
    childProcess.execFileSync("npm", args, { cwd: root, stdio: "ignore" });

    // eslint-disable-next-line @typescript-eslint/no-var-requires
    generated = require(path.join(output, "index.ts"));
});

describe("Command examples", () => {
    it("import", () => {
        const command = generated.__import({}, "'aws:iam/user:User'", "name", "id");
        expect(command).toBe("pulumi import 'aws:iam/user:User' name id");
    });

    it("up", () => {
        const options = { target: ["urnA", "urnB"] };
        const command = generated.up(options, "https://pulumi.com");
        expect(command).toBe("pulumi up https://pulumi.com --target urnA --target urnB");
    });
});
