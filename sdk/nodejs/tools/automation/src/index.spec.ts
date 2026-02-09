// Copyright 2016-2026, Pulumi Corporation.
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

import { __import, up, PulumiImportOptions, PulumiUpOptions } from "../output";

describe("Command examples", () => {
  it("import", () => {
    const options: PulumiImportOptions = {
    };

    const command = __import(options, "'aws:iam/user:User'", "name", "id");
    expect(command).toBe("pulumi import 'aws:iam/user:User' name id");
  });

  it("up", () => {
    const options: PulumiUpOptions = {
      target: ["urnA", "urnB"],
    };

    const command = up(options, "https://pulumi.com");
    expect(command).toBe("pulumi up https://pulumi.com --target urnA --target urnB");
  });
});

