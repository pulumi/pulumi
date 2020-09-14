import { Workspace } from "./workspace";
import { ProjectSettings } from "./projectSettings";
import { StackSettings } from "./stackSettings";

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
        // TODO
        return Promise.resolve(new ProjectSettings());
    }
}

export type LocalWorkspaceOpts = {
    workDir: string,
    pulumiHome: string,
    program: ()=>void,
    envVars: Map<string,string>,
}