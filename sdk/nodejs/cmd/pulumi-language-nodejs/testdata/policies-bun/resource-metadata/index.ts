// Copyright 2026, Pulumi Corporation.  All rights reserved.

import { PolicyPack } from "@pulumi/policy";

type ReportViolation = (message: string, urn?: string) => void;

const resourceNamed = (args: any, name: string): boolean =>
    args.type === "simple:index:Resource" && args.name === name;

const opts = (resource: any): any => resource.opts || resource.options || {};

const provider = (resource: any): any => resource.provider || {};

const parent = (resource: any): string => resource.parent || opts(resource).parent || "";

const contains = (values: any, expected: string): boolean =>
    Array.isArray(values) && values.some(value => String(value) === expected);

const urnString = (value: any): string => String(value?.urn || value?.URN || value);

const containsURN = (values: any, name: string): boolean =>
    Array.isArray(values) && values.some(value => urnString(value).includes(`::${name}`));

const timeoutCreate = (resource: any): string => {
    const customTimeouts = opts(resource).customTimeouts || opts(resource).custom_timeouts || {};
    return String(customTimeouts.create || customTimeouts.createSeconds || "");
};

const propertyDependencies = (resource: any, key: string): any => {
    const deps = resource.propertyDependencies || resource.property_dependencies || {};
    if (deps instanceof Map) {
        return deps.get(key);
    }
    const value = deps[key];
    return value?.urns || value || [];
};

const reportIf = (args: any, name: string, ok: boolean, reportViolation: ReportViolation): void => {
    if (resourceNamed(args, name) && ok) {
        reportViolation("metadata matched");
    }
};

new PolicyPack("resource-metadata", {
    enforcementLevel: "advisory",
    policies: [
        {
            name: "name-type-urn-props",
            description: "Resource identity metadata is available",
            enforcementLevel: "advisory",
            validateResource: (args: any, reportViolation: ReportViolation) => {
                reportIf(args, "identity", args.props?.value === true && String(args.urn).includes("::identity"), reportViolation);
            },
        },
        {
            name: "protect",
            description: "Protect option metadata is available",
            enforcementLevel: "advisory",
            validateResource: (args: any, reportViolation: ReportViolation) => {
                reportIf(args, "protected", opts(args).protect === true, reportViolation);
            },
        },
        {
            name: "ignore-changes",
            description: "IgnoreChanges option metadata is available",
            enforcementLevel: "advisory",
            validateResource: (args: any, reportViolation: ReportViolation) => {
                reportIf(args, "ignoreChanges", contains(opts(args).ignoreChanges, "value"), reportViolation);
            },
        },
        {
            name: "delete-before-replace",
            description: "DeleteBeforeReplace option metadata is available",
            enforcementLevel: "advisory",
            validateResource: (args: any, reportViolation: ReportViolation) => {
                reportIf(args, "deleteBeforeReplace", opts(args).deleteBeforeReplace === true, reportViolation);
            },
        },
        {
            name: "additional-secret-outputs",
            description: "AdditionalSecretOutputs option metadata is available",
            enforcementLevel: "advisory",
            validateResource: (args: any, reportViolation: ReportViolation) => {
                reportIf(args, "secretOutput", contains(opts(args).additionalSecretOutputs, "value"), reportViolation);
            },
        },
        {
            name: "custom-timeouts",
            description: "CustomTimeouts option metadata is available",
            enforcementLevel: "advisory",
            validateResource: (args: any, reportViolation: ReportViolation) => {
                reportIf(args, "customTimeouts", ["5m", "5m0s", "300"].includes(timeoutCreate(args)), reportViolation);
            },
        },
        {
            name: "provider",
            description: "Provider metadata is available",
            enforcementLevel: "advisory",
            validateResource: (args: any, reportViolation: ReportViolation) => {
                const p = provider(args);
                reportIf(args, "explicitProvider", p.type === "pulumi:providers:simple" && p.name === "prov", reportViolation);
            },
        },
        {
            name: "parent",
            description: "Parent metadata is available",
            enforcementLevel: "advisory",
            validateResource: (args: any, reportViolation: ReportViolation) => {
                reportIf(args, "child", parent(args).includes("::parent"), reportViolation);
            },
        },
        {
            name: "dependencies",
            description: "Dependency metadata is available",
            enforcementLevel: "advisory",
            validateStack: (args: any, reportViolation: ReportViolation) => {
                const resource = args.resources.find((r: any) => resourceNamed(r, "dependsOn"));
                if (resource && containsURN(resource.dependencies, "dependency")) {
                    reportViolation("metadata matched", resource.urn);
                }
            },
        },
        {
            name: "property-dependencies",
            description: "Property dependency metadata is available",
            enforcementLevel: "advisory",
            validateStack: (args: any, reportViolation: ReportViolation) => {
                const resource = args.resources.find((r: any) => resourceNamed(r, "propertyDependency"));
                if (resource && containsURN(propertyDependencies(resource, "value"), "dependency")) {
                    reportViolation("metadata matched", resource.urn);
                }
            },
        },
    ],
});
