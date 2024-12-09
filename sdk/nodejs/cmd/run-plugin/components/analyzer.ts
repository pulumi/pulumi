import * as ts from "typescript";
import * as path from "path";

// Enhanced schema types
type TypeDefinition = {
    type: "object";
    properties: Record<string, SchemaProperty>;
    description?: string;
};

export type SchemaProperty = {
    type?: string;
    ref?: string;  // Reference to a type in typeDefinitions
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

export class ComponentAnalyzer {
    private checker: ts.TypeChecker;
    private program: ts.Program;

    constructor(dir: string) {
        const configPath = `${dir}/tsconfig.json`;
        const config = ts.readConfigFile(configPath, ts.sys.readFile);
        const parsedConfig = ts.parseJsonConfigFileContent(
            config.config,
            ts.sys,
            path.dirname(configPath)
        );
        
        this.program = ts.createProgram({
            rootNames: parsedConfig.fileNames,
            options: parsedConfig.options,
        });
        this.checker = this.program.getTypeChecker();
    }

    public analyzeComponents(): AnalyzedComponents {
        const components: AnalyzedComponents = {};

        this.program.getSourceFiles().forEach(sourceFile => {
            if (sourceFile.fileName.includes('node_modules') || 
                sourceFile.fileName.endsWith('.d.ts')) {
                return;
            }

            this.findComponentsInFile(sourceFile, components);
        });

        return components;
    }

    private findComponentsInFile(sourceFile: ts.SourceFile, components: AnalyzedComponents) {
        const visit = (node: ts.Node) => {
            if (ts.isClassDeclaration(node) && node.name && this.isPulumiComponent(node)) {
                const componentName = node.name.text;
                
                components[componentName] = {
                    inputs: {},
                    outputs: {},
                    typeDefinitions: {},
                    description: this.getJSDocComment(node)
                };

                this.analyzeComponentClass(node, components[componentName], sourceFile);
                
                const argsInterfaceName = `${componentName}Args`;
                this.findAndAnalyzeArgsInterface(sourceFile, argsInterfaceName, components[componentName]);
            }

            ts.forEachChild(node, visit);
        };

        visit(sourceFile);
    }

    private getJSDocComment(node: ts.Node): string | undefined {
        const nodeFullText = node.getFullText();
        const commentRanges = ts.getLeadingCommentRanges(nodeFullText, 0);
        
        if (commentRanges) {
            const comments = commentRanges
                .map(range => {
                    const text = nodeFullText.slice(range.pos, range.end);
                    return text;
                })
                .map(text => {
                    if (text.startsWith('/**')) {
                        return text.replace(/\/\*\*|\*\/|\* ?/g, '').trim();
                    } else if (text.startsWith('//')) {
                        return text.replace(/\/\/ ?/g, '').trim();
                    }
                    return text.trim();
                })
                .filter(text => text.length > 0);
            
            if (comments.length > 0) {
                return comments[0];
            }
        }
        return undefined;
    }

    private analyzeType(type: ts.Type, schema: ComponentSchema, parentName: string): SchemaProperty {
        const typeString = this.checker.typeToString(type);
        
        // Handle arrays
        if (type.symbol?.name === "Array" || typeString.includes("[]")) {
            const typeRef = type as ts.TypeReference;
            let elementType: ts.Type | undefined;
            
            if (typeRef.typeArguments && typeRef.typeArguments.length > 0) {
                elementType = typeRef.typeArguments[0];
            }
            
            if (elementType) {
                const elementTypeInfo = this.analyzeType(elementType, schema, parentName);
                if (elementTypeInfo.ref) {
                    return { ref: `${elementTypeInfo.ref}[]` };
                }
                return { type: `${elementTypeInfo.type}[]` };
            }
        }
    
        // Handle Input<T> and Output<T>
        if (typeString.includes('Input<') || typeString.includes('Output<') || typeString.includes('OutputInstance<')) {
            const typeRef = type as ts.TypeReference;
            
            if (typeRef.typeArguments && typeRef.typeArguments.length > 0) {
                const innerType = typeRef.typeArguments[0];
                return this.analyzeType(innerType, schema, parentName);
            }
            
            // Try parsing from the type string if typeArguments isn't available
            const match = typeString.match(/<(.+)>/);
            if (match) {
                const innerTypeStr = match[1];
                if (this.isPrimitiveType(innerTypeStr)) {
                    return { type: innerTypeStr.toLowerCase() };
                }
            }
        }
    
        // Handle union types (e.g., string | undefined)
        if (type.flags & ts.TypeFlags.Union) {
            const unionType = type as ts.UnionType;
            // Use the first non-undefined type in the union
            const nonUndefinedType = unionType.types.find(t => !(t.flags & ts.TypeFlags.Undefined));
            if (nonUndefinedType) {
                return this.analyzeType(nonUndefinedType, schema, parentName);
            }
        }
    
        // Handle primitive types
        if (this.isPrimitiveType(typeString)) {
            return { type: typeString.toLowerCase() };
        }
    
        // Handle interface/object types
        if (type.isClassOrInterface() || (type.flags & ts.TypeFlags.Object)) {
            const properties = type.getProperties();
            if (properties.length > 0) {
                const typeName = `${parentName}_${this.generateTypeName(typeString)}`;
                
                const typeDef: TypeDefinition = {
                    type: "object",
                    properties: {},
                };
    
                properties.forEach(prop => {
                    const declaration = prop.valueDeclaration;
                    if (declaration) {
                        const propType = this.checker.getTypeOfSymbolAtLocation(prop, declaration);
                        const description = declaration && this.getJSDocComment(declaration);
                        const optional = !!(prop.flags & ts.SymbolFlags.Optional);
                        
                        typeDef.properties[prop.name] = {
                            ...this.analyzeType(propType, schema, typeName),
                            optional,
                            ...(description && { description })
                        };
                    }
                });
    
                schema.typeDefinitions[typeName] = typeDef;
                return { ref: typeName };
            }
        }
    
        return { type: "unknown" };
    }
    
    private isPrimitiveType(type: string): boolean {
        // Clean up the type string
        const cleanType = type.replace(/Input<|Output<|OutputInstance<|>/g, '').trim();
        const primitives = ['string', 'number', 'boolean', 'null', 'undefined'];
        return primitives.includes(cleanType.toLowerCase());
    }

    private generateTypeName(typeString: string): string {
        return typeString
            .replace(/[^a-zA-Z0-9_]/g, '_')
            .replace(/_+/g, '_');
    }

    private analyzeArgsInterface(node: ts.InterfaceDeclaration, schema: ComponentSchema) {
        node.members.forEach(member => {
            if (ts.isPropertySignature(member)) {
                const propName = member.name.getText();
                const propType = this.checker.getTypeAtLocation(member);
                const optional = !!(member.questionToken);
                const description = this.getJSDocComment(member);

                const typeInfo = this.analyzeType(propType, schema, `input_${propName}`);

                schema.inputs[propName] = {
                    ...typeInfo,
                    optional,
                    ...(description && { description })
                };
            }
        });
    }

    private analyzeComponentClass(node: ts.ClassDeclaration, schema: ComponentSchema, sourceFile: ts.SourceFile) {
        const classType = this.checker.getTypeAtLocation(node);
        const properties = this.checker.getPropertiesOfType(classType);
        
        properties.forEach(prop => {
            if (prop.flags & ts.SymbolFlags.Method) return;
            
            const declarations = prop.declarations;
            if (!declarations || declarations.length === 0) return;
            
            const declaration = declarations[0];
            const propType = this.checker.getTypeOfSymbolAtLocation(prop, declaration);
            
            if (this.isPulumiOutput(propType)) {
                const description = this.getJSDocComment(declaration);
                const typeInfo = this.analyzeType(propType, schema, `output_${prop.name}`);

                schema.outputs[prop.name] = {
                    ...typeInfo,
                    ...(description && { description })
                };
            }
        });

        // Filter out internal Pulumi properties
        const internalProps = ['urn', 'id'];
        internalProps.forEach(prop => {
            delete schema.outputs[prop];
        });
    }

    private isPulumiComponent(node: ts.ClassDeclaration): boolean {
        if (!node.heritageClauses) return false;

        return node.heritageClauses.some(clause => {
            return clause.types.some(type => {
                const text = type.expression.getText();
                return text.includes('ComponentResource') || text.includes('pulumi.ComponentResource');
            });
        });
    }

    private findAndAnalyzeArgsInterface(sourceFile: ts.SourceFile, argsInterfaceName: string, schema: ComponentSchema) {
        const visit = (node: ts.Node) => {
            if (ts.isInterfaceDeclaration(node) && node.name.text === argsInterfaceName) {
                this.analyzeArgsInterface(node, schema);
            }
            ts.forEachChild(node, visit);
        };

        visit(sourceFile);
    }

    private isPulumiOutput(type: ts.Type): boolean {
        const typeString = this.checker.typeToString(type);
        return typeString.includes('Output<') || typeString.includes('OutputInstance<');
    }
}
