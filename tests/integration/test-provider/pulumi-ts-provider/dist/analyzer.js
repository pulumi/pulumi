"use strict";
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || (function () {
    var ownKeys = function(o) {
        ownKeys = Object.getOwnPropertyNames || function (o) {
            var ar = [];
            for (var k in o) if (Object.prototype.hasOwnProperty.call(o, k)) ar[ar.length] = k;
            return ar;
        };
        return ownKeys(o);
    };
    return function (mod) {
        if (mod && mod.__esModule) return mod;
        var result = {};
        if (mod != null) for (var k = ownKeys(mod), i = 0; i < k.length; i++) if (k[i] !== "default") __createBinding(result, mod, k[i]);
        __setModuleDefault(result, mod);
        return result;
    };
})();
Object.defineProperty(exports, "__esModule", { value: true });
exports.ComponentAnalyzer = void 0;
const ts = __importStar(require("typescript"));
const path = __importStar(require("path"));
class ComponentAnalyzer {
    constructor(dir) {
        const configPath = `${dir}/tsconfig.json`;
        const config = ts.readConfigFile(configPath, ts.sys.readFile);
        const parsedConfig = ts.parseJsonConfigFileContent(config.config, ts.sys, path.dirname(configPath));
        this.program = ts.createProgram({
            rootNames: parsedConfig.fileNames,
            options: parsedConfig.options,
        });
        this.checker = this.program.getTypeChecker();
    }
    analyzeComponents() {
        const components = {};
        this.program.getSourceFiles().forEach(sourceFile => {
            if (sourceFile.fileName.includes('node_modules') ||
                sourceFile.fileName.endsWith('.d.ts')) {
                return;
            }
            this.findComponentsInFile(sourceFile, components);
        });
        return components;
    }
    findComponentsInFile(sourceFile, components) {
        const visit = (node) => {
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
    getJSDocComment(node) {
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
                }
                else if (text.startsWith('//')) {
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
    analyzeType(type, schema, parentName) {
        var _a;
        const typeString = this.checker.typeToString(type);
        // Handle arrays
        if (((_a = type.symbol) === null || _a === void 0 ? void 0 : _a.name) === "Array" || typeString.includes("[]")) {
            const typeRef = type;
            let elementType;
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
            const typeRef = type;
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
            const unionType = type;
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
                const typeDef = {
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
    isPrimitiveType(type) {
        // Clean up the type string
        const cleanType = type.replace(/Input<|Output<|OutputInstance<|>/g, '').trim();
        const primitives = ['string', 'number', 'boolean', 'null', 'undefined'];
        return primitives.includes(cleanType.toLowerCase());
    }
    generateTypeName(typeString) {
        return typeString
            .replace(/[^a-zA-Z0-9_]/g, '_')
            .replace(/_+/g, '_');
    }
    analyzeArgsInterface(node, schema) {
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
    analyzeComponentClass(node, schema, sourceFile) {
        const classType = this.checker.getTypeAtLocation(node);
        const properties = this.checker.getPropertiesOfType(classType);
        properties.forEach(prop => {
            if (prop.flags & ts.SymbolFlags.Method)
                return;
            const declarations = prop.declarations;
            if (!declarations || declarations.length === 0)
                return;
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
    isPulumiComponent(node) {
        if (!node.heritageClauses)
            return false;
        return node.heritageClauses.some(clause => {
            return clause.types.some(type => {
                const text = type.expression.getText();
                return text.includes('ComponentResource') || text.includes('pulumi.ComponentResource');
            });
        });
    }
    findAndAnalyzeArgsInterface(sourceFile, argsInterfaceName, schema) {
        const visit = (node) => {
            if (ts.isInterfaceDeclaration(node) && node.name.text === argsInterfaceName) {
                this.analyzeArgsInterface(node, schema);
            }
            ts.forEachChild(node, visit);
        };
        visit(sourceFile);
    }
    isPulumiOutput(type) {
        const typeString = this.checker.typeToString(type);
        return typeString.includes('Output<') || typeString.includes('OutputInstance<');
    }
}
exports.ComponentAnalyzer = ComponentAnalyzer;
//# sourceMappingURL=analyzer.js.map