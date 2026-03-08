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

// This file exports metadata about the context in which a program is being run.

import * as settings from "./runtime/settings";

/**
 * Returns the current organization name.
 */
export function getOrganization(): string {
    return settings.getOrganization();
}
/**
 * Returns the current project name. Throws an exception if none is registered.
 */
export function getProject(): string {
    return settings.getProject();
}
/**
 * Returns the current stack name. Throws an exception if none is registered.
 */
export function getStack(): string {
    return settings.getStack();
}
/**
 * Returns the root directory of the current Pulumi project.
 */
export function getRootDirectory(): string {
    return settings.getRootDirectory();
}

/**
 * Checks if the engine we are connected to is compatible with the passed in version range. If the version is not
 * compatible with the specified range, an exception is raised.
 *
 * @param range
 *  The range to check. The supported syntax for the range is that of
 *  https://pkg.go.dev/github.com/blang/semver#ParseRange. For example ">=3.0.0", or "!3.1.2". Ranges can be AND-ed
 *  together by concatenating with spaces ">=3.5.0 !3.7.7", meaning greater-or-equal to 3.5.0 and not exactly 3.7.7.
 *  Ranges can be OR-ed with the `||` operator: "<3.4.0 || >3.8.0", meaning less-than 3.4.0 or greater-than 3.8.0.
 */
export function requirePulumiVersion(range: string): Promise<void> {
    return settings.requirePulumiVersion(range);
}
