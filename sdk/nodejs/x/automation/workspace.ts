import { ProjectSettings } from "./projectSettings";
import { StackSettings } from "./stackSettings";
import { ConfigValue } from "./config";

export interface Workspace{
    projectSettings(): Promise<ProjectSettings>
    saveProjectSettings(settings: ProjectSettings): Promise<void>
    stackSettings(): Promise<StackSettings>
    saveStackSettings(settings: StackSettings, fqsn: string): Promise<void>
    serializeArgsForOp(fqsn: string): string[]
    postCommandCallback(fqsn: string): void
    getConfig(fqsn: string,key: string): Promise<ConfigValue>
}