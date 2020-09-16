import * as yaml from "js-yaml";

export class StackSettings {
    secretsProvider?: string
    encryptedKey?: string
    encryptionSalt?: string
    config?: {[key: string]: StackSettingsConfigValue}

    public static fromJSON(obj: any) {
        const stack = new StackSettings();
        if (obj.config) {
            Object.keys(obj.config).forEach(k => {
                obj.config[k] = StackSettingsConfigValue.fromJSON(obj.config![k])
            })
        }

        stack.secretsProvider = obj.secretsProvider;
        stack.encryptedKey = obj.encryptedKey;
        stack.encryptionSalt = obj.encryptionSalt;
        stack.config = obj.config

        return stack
    }
    public static fromYAML(text: string) {
        const res = yaml.safeLoad(text, { json: true });
        return StackSettings.fromJSON(res);
    }
    toYAML(): string {
        const copy = <StackSettings>Object.assign({}, this);
        if (copy.config) {
            Object.keys(copy.config).forEach(k => {
                copy.config![k] = copy.config![k].toJSON()
            })
        }
        return yaml.safeDump(copy, { skipInvalid: true });
    }
}

export class StackSettingsConfigValue {
    value?: string
    secure?: string
    public static fromJSON(obj: any) {
        const config = new StackSettingsConfigValue();

        if (typeof obj === "string") {
            config.value = obj;
        }
        else {
            config.secure = obj.secure
        }

        if (!config.value && !config.secure) {
            throw new Error("could not deserialize invalid StackSettingsConfigValue object")
        }

        return config;
    }
    toJSON(): any{
        if (this.secure){
            return {
                secure: this.secure
            };
        }
        return this.value
    }
}