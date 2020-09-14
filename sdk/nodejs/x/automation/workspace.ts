import { StackSettings } from "./stackSettings";
import { ProjectSettings } from "./projectSettings"
import { ConfigValue, ConfigMap } from "./config";

export interface Workspace {
    projectSettings(): Promise<ProjectSettings>
    saveProjectSettings(settings: ProjectSettings): Promise<void>
    stackSettings(): Promise<StackSettings>
    saveStackSettings(settings: StackSettings, stackName: string): Promise<void>
    serializeArgsForOp(stackName: string): string[]
    postCommandCallback(stackName: string): void
    getConfig(stackName: string, key: string): Promise<ConfigValue>
    getAllConfig(stackName: string): Promise<ConfigMap>
    setConfig(stackName: string, key: string, value: ConfigValue): Promise<void>
    setAllConfig(stackName: string, config: ConfigMap): Promise<void>
    removeConfig(stackName: string, key: string): Promise<void>
    removeAllConfig(stackName: string, keys: string[]): Promise<void>
    refreshConfig(stackName: string): Promise<ConfigMap>
    getEnvVars(): Map<string, string>
    setEnvVars(envs: Map<string, string>): void
    setEnvVar(key: string, value: string): void
    unsetEnvVar(key: string): void
    getWorkDir(): string
    getPulumiHome(): string
    whoAmI(): Promise<string>
    stack(): Promise<string>
    createStack(stackName: string): Promise<void>
    selectStack(stackName: string): Promise<void>
    removeStack(stackName: string): Promise<void>
    listStacks(): Promise<StackSummary[]>
    getProgram(): () => void
    setProgram(program: () => void): void
}

export type StackSummary = {
    name: string,
    current: boolean,
    lastUpdate: string,
    updateInProgress: boolean,
    resourceCount?: number,
    url: string,
}
