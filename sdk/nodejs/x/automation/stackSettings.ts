// Copyright 2016-2020, Pulumi Corporation.
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

import * as yaml from "js-yaml";

/**
 * A description of the Stack's configuration and encryption metadata.
 */
export interface StackSettings {
    secretsProvider?: string;
    encryptedKey?: string;
    encryptionSalt?: string;
    config?: {[key: string]: StackSettingsConfigValue};
}

/**
 * Stack configuration entry
 */
export type StackSettingsConfigValue = string | StackSettingsSecureConfigValue | any;

/**
 * A secret Stack config entry
 */
export interface StackSettingsSecureConfigValue {
    secure: string;
}

/** @internal */
export const stackSettingsSerDeKeys = [
    ["secretsprovider", "secretsProvider"],
    ["encryptedkey", "encryptedKey"],
    ["encryptionsalt", "encryptionSalt"],
];
