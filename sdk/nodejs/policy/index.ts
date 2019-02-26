// Copyright 2016-2018, Pulumi Corporation.
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

import * as log from "../log";
import * as runtime from "../runtime";

export type Cost = "cost";
export type Security = "Security";
export type Tags = Cost | Security;

export type Rule = (inputs: any) => boolean;

export interface Policy {
    description: string;
    message: string;
    tags: Tags[];
    rule: Rule;
}

export interface TypedPolicy extends Policy {
    pulumiType: string;
}

class PolicyRecord implements TypedPolicy {
    description: string;
    tags: Tags[];
    message: string;
    rule: Rule;

    pulumiType: string;

    failures: string[];

    constructor(policy: TypedPolicy) {
        Object.assign(this, policy);

        this.failures = [];
    }

    public toString(): string {
        return `${this.failures.length} violations of rule '${
            this.description
        }':\n           - ${this.failures.map(name => `${this.pulumiType} ${name}`).join("\n           - ")}`;
    }
}

export class PolicySet {
    public readonly policies: PolicyRecord[] = [];

    constructor() {
        process.on("beforeExit", () => {
            const violations = this.policies.filter(policy => policy.failures.length > 0);
            if (violations.length > 0) {
                violations.forEach(policy => {
                    log.error(policy.toString());

                    // Clear policy failures so that when `log.error` causes us to schedule the exit
                    // again, we don't have more work to do.
                    policy.failures = [];
                });
                log.error("One or more policy violations occurred");
            }
        });
    }

    public addPolicy(policy: TypedPolicy) {
        this.policies.push(new PolicyRecord(policy));
    }

    public validate(typ: string, id: string, inputs: any) {
        this.policies.forEach(policy => {
            if (typ !== policy.pulumiType) {
                return;
            }

            const policyViolated = policy.rule(inputs);
            if (policyViolated) {
                console.log();
                if (runtime.isDryRun) {
                    policy.failures.push(id);
                } else {
                    throw Error(`Policy '${policy.description}' violated by resource ${id}`);
                }
            }
        });
    }
}
