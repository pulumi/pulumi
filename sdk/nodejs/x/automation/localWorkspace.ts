import { Workspace } from "./workspace";
import { ProjectSettings } from "./projectSettings";
import { StackSettings } from "./stackSettings";
import { settings } from "cluster";
import * as fs from "fs";
import * as upath from "upath";
import { promises } from "dns";


export class LocalWorkspace /*implements Workspace TODO */ {
    private workDir?: string;
    private pulumiHome?: string;
    private program?: () => void;
    private envVars?: Map<string, string>;
    constructor(opts?: LocalWorkspaceOpts){
        if (opts) {
            const {workDir, pulumiHome, program, envVars} = opts;
            this.workDir = workDir;
            this.pulumiHome = pulumiHome;
            this.program = program;
            this.envVars = envVars
        }
    }
    projectSettings(): Promise<ProjectSettings>{
        for (let ext of settingsExtensions) {
            const isJSON = ext === ".json";
            const path = upath.joinSafe(this.workDir, `Pulumi${ext}`);
            if (!fs.existsSync(path)) continue;
            const contents = fs.readFileSync(path).toString();
            if (isJSON) {
                return Promise.resolve(ProjectSettings.fromJSON(JSON.parse(contents)))
            }
            return Promise.resolve(ProjectSettings.fromYAML(contents))
        }
        return Promise.reject(new Error(`failed to find project settings file in workdir: ${this.workDir}`));
    }
    saveProjectSettings(settings: ProjectSettings): Promise<void> {
        let foundExt = ".yaml";
        for (let ext of settingsExtensions) {
           const path = upath.joinSafe(this.workDir, `Pulumi${ext}`);
           if (fs.existsSync(path)) {
               foundExt = ext;
               break;
           }
        }
        const path = upath.joinSafe(this.workDir, `Pulumi${foundExt}`);
        let contents;
        if (foundExt === ".json") {
            contents = JSON.stringify(settings, null, 4);
        }
        else {
            contents = settings.toYAML();
        }
        return Promise.resolve(fs.writeFileSync(path, contents))
    }
}

export type LocalWorkspaceOpts = {
    workDir?: string,
    pulumiHome?: string,
    program?: ()=>void,
    envVars?: Map<string,string>,
}

export const settingsExtensions = [".yaml", ".yml", ".json"];
