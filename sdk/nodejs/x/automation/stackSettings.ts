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

export class StackSettings {
    secretsProvider?: string;
    encryptedKey?: string;
    encryptionSalt?: string;
    config?: {[key: string]: StackSettingsConfigValue};

    public static fromJSON(obj: any) {
        const stack = new StackSettings();
        if (obj.config) {
            Object.keys(obj.config).forEach(k => {
                obj.config[k] = StackSettingsConfigValue.fromJSON(obj.config![k]);
            });
        }

        stack.secretsProvider = obj.secretsProvider;
        stack.encryptedKey = obj.encryptedKey;
        stack.encryptionSalt = obj.encryptionSalt;
        stack.config = obj.config;

        return stack;
    }
    public static fromYAML(text: string) {
        const res = yaml.safeLoad(text, { json: true });
        return StackSettings.fromJSON(res);
    }
    toYAML(): string {
        const copy = <StackSettings>Object.assign({}, this);
        if (copy.config) {
            Object.keys(copy.config).forEach(k => {
                copy.config![k] = copy.config![k].toJSON();
            });
        }
        return yaml.safeDump(copy, { skipInvalid: true });
    }
}

export class StackSettingsConfigValue {
    value?: string;
    secure?: string;
    public static fromJSON(obj: any) {
        const config = new StackSettingsConfigValue();

        if (typeof obj === "string") {
            config.value = obj;
        }
        else {
            config.secure = obj.secure;
        }

        if (!config.value && !config.secure) {
            throw new Error("could not deserialize invalid StackSettingsConfigValue object");
        }

        return config;
    }
    toJSON(): any {
        if (this.secure) {
            return {
                secure: this.secure,
            };
        }
        return this.value;
    }
}
