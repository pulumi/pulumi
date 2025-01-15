type TypeDefinition = {
    type: "object";
    properties: Record<string, SchemaProperty>;
    description?: string;
};
export type SchemaProperty = {
    type?: string;
    ref?: string;
    optional?: boolean;
    description?: string;
};
export type ComponentSchema = {
    description?: string;
    inputs: Record<string, SchemaProperty>;
    outputs: Record<string, SchemaProperty>;
    typeDefinitions: Record<string, TypeDefinition>;
};
type AnalyzedComponents = Record<string, ComponentSchema>;
export declare class ComponentAnalyzer {
    private checker;
    private program;
    constructor(dir: string);
    analyzeComponents(): AnalyzedComponents;
    private findComponentsInFile;
    private getJSDocComment;
    private analyzeType;
    private isPrimitiveType;
    private generateTypeName;
    private analyzeArgsInterface;
    private analyzeComponentClass;
    private isPulumiComponent;
    private findAndAnalyzeArgsInterface;
    private isPulumiOutput;
}
export {};
//# sourceMappingURL=analyzer.d.ts.map