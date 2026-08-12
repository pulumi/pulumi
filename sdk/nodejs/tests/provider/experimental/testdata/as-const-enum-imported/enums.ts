// Copyright 2026, Pulumi Corporation.  All rights reserved.

/** This demonstrates const enums defined in a separate file */
export const DeploymentMode = {
    Development: "dev",
    Staging: "staging",
    Production: "prod",
} as const;
export type DeploymentMode = (typeof DeploymentMode)[keyof typeof DeploymentMode];

/** This demonstrates numeric const enums defined in a separate file */
export const RetryCount = {
    Low: 3,
    Medium: 5,
    High: 10,
} as const;
export type RetryCount = (typeof RetryCount)[keyof typeof RetryCount];
