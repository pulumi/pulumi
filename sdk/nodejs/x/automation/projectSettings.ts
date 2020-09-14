import * as yaml from "js-yaml";

export const settingsExtensions = [".yaml", ".yml", ".json"];

export class ProjectSettings {
    name: string
    runtime: ProjectRuntimeInfo
    main?: string
    description?: string
    author?: string
    website?: string
    license?: string
    config?: string
    template?: ProjectTemplate
    backend?: ProjectBackend

    constructor() {
        this.name = ""
        this.runtime = new ProjectRuntimeInfo();
    }

    public static fromJSON(obj: any) {
        const proj = new ProjectSettings();

        if (!obj.runtime || !obj.name) {
            throw new Error("could not deserialize ProjectSettings, missing required properties");
        }

        proj.name = obj.name;
        proj.runtime = ProjectRuntimeInfo.fromJSON(obj.runtime)
        proj.main = obj.main;
        proj.description = obj.description;
        proj.author = obj.author;
        proj.website = obj.website;
        proj.license = obj.license;
        proj.config = obj.config;
        proj.template = obj.template;
        proj.backend = obj.backend;

        return proj
    }

    public static fromYAML(text: string) {
        const res = yaml.safeLoad(text, { json: true });
        return ProjectSettings.fromJSON(res);
    }
    
    toYAML(): string {
        const copy = Object.assign({}, this);
        copy.runtime = copy.runtime.toJSON();
        return yaml.safeDump(copy, { skipInvalid: true });
    }

}

export class ProjectRuntimeInfo {
    name: string
    options?: {[key: string]: any}

    constructor() {
        this.name = "";
    }

    public static fromJSON(obj: any) {
        const info = new ProjectRuntimeInfo();

        if (typeof obj === "string") {
            info.name = obj;
        }
        else {
            info.name = obj.name;
            info.options = obj.options;
        }

        if (!info.name) {
            throw new Error("could not deserialize invalid ProjectRuntimeInfo object")
        }

        return info;
    }

    toJSON(): any {
        if (!this.options) {
            return this.name
        }

        return this
    }
}

export type ProjectTemplate = {
    description?: string,
    quickstart?: string,
    config?: {[key: string]: ProjectTemplateConfigValue},
    important?: boolean,
}

export type ProjectTemplateConfigValue = {
    description?: string,
    default?: string,
    secret?: boolean,
}

export type ProjectBackend = {
    url?: string,
}