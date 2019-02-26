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

/**
 * Tags are informational categories that can be applied to a policy. This can be useful for
 * generating views of policy failures, reports, etc.
 */
export enum Tags {
    /** Cost indicates a policy is related to cost management. */
    Cost = "cost",
    /** Security indicates a policy is related to security. */
    Security = "security",
}

/** Represents how a policy violation should be handled (e.g., blocking deployment). */
export enum EnforcementLevel {
    /** Warning represents a policy that displays a warning on violation. */
    Warning = "warning",
    /**
     * SoftMandatory represents a policy that prevents deployment on violation, but can be
     * overridden with appropriate permissions.
     */
    SoftMandatory = "mandatory",
    /**
     * HardMandatory represents a policy that prevents deployment on violation, and cannot be
     * overridden.
     */
    HardMandatory = "hardMandatory",
}

/** A policy rule that validates `inputs`, returning a boolean indicating success. */
export type Rule = (inputs: any) => boolean;

/**
 * Decides whether a resource operation should proceed ("be admitted") or not. Admission policies
 * are checked both in the preview stage, and just before any resource operation occurs, allowing
 * the policy to prevent problematic resource operations from occurring. Metadata is provided to
 * allow meaningful error messages when a policy violation occurs.
 */
export interface AdmissionPolicy {
    /**
     * A brief description of the policy rule. e.g., "S3 buckets should have default encryption
     * enabled."
     */
    description: string;

    /**
     * A detailed message to display on policy violation. Typically includes an explanation of the
     * policy, and steps to take to remediate.
     */
    message: string;

    /**
     * Tags to help sort and filter policy violations for reporting purposes, e.g., filtering on the
     * "cost" label.
     */
    tags: Tags[];

    /**
     * Indicates what to do on policy violation, e.g., block deployment but allow override with
     * proper permissions.
     */
    enforcementLevel: EnforcementLevel;

    /** The core policy logic, checking whether a resource violates the policy. */
    rule: Rule;
}

/**
 * Decides whether a resource operation on a specific type (e.g., AWS S3 Bucket) should proceed ("be
 * admitted") or not. Admission policies are checked both in the preview stage, and just before any
 * resource operation occurs, allowing the policy to prevent problematic resource operations from
 * occurring. Metadata is provided to allow meaningful error messages when a policy violation
 * occurs.
 */
export interface TypedAdmissionPolicy extends AdmissionPolicy {
    /**
     * The type of the resource to apply the policy to. e.g., Kubernetes
     * `kubernetes:core/v1:Service`.
     */
    pulumiType: string;
}

/**
 * A TypedAdmissionPolicy, paired with information about resource operations that have failed. This
 * is used primarily to aggregate a list of resources that have failed a specific policy rule, so
 * that they can be reported all at once (e.g., at the end of a preview).
 */
class AdmissionPolicyRecord implements TypedAdmissionPolicy {
    public readonly description: string;
    public readonly tags: Tags[];
    public readonly message: string;
    public readonly enforcementLevel: EnforcementLevel;
    public readonly rule: Rule;

    public readonly pulumiType: string;

    public failures: string[];

    constructor(policy: TypedAdmissionPolicy) {
        Object.assign(this, policy);

        this.failures = [];
    }

    public toString(): string {
        return `${this.failures.length} violations of rule '${
            this.description
        }':\n           - ${this.failures
            .map(name => `${this.pulumiType} ${name}`)
            .join("\n           - ")}`;
    }
}

/**
 * Maintains a set of TypedAdmissionPolicy, a `validate` function that allows external sources to
 * validate resources against the policy corpus as they please, and information about which
 * resources have violated those policies, and the reporting of those policy violations.
 *
 * The reporting semantics differ in two important cases:
 *
 *   - On preview, we will aggregate all known policy violations, and report all of them as errors
 *     at the end of the preview.
 *   - If the preview for a resource operation is skipped, any policy violation will occur
 *     immediately, halting the deployment.
 *
 * The "aggregated report" approach of the preview is accomplished by using the Node.js exit hooks.
 */
export class TypedAdmissionPolicySet {
    private readonly policies: AdmissionPolicyRecord[] = [];

    constructor(private readonly aggregateErrors: boolean) {
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

    public addPolicy(policy: TypedAdmissionPolicy) {
        this.policies.push(new AdmissionPolicyRecord(policy));
    }

    public validate(typ: string, id: string, inputs: any) {
        this.policies.forEach(policy => {
            if (typ !== policy.pulumiType) {
                return;
            }

            const policyViolated = policy.rule(inputs);
            if (policyViolated) {
                if (this.aggregateErrors) {
                    policy.failures.push(id);
                } else {
                    throw Error(`Policy '${policy.description}' violated by resource ${id}`);
                }
            }
        });
    }
}
