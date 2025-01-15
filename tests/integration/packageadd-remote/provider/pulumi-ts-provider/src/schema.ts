import * as analyzer from "./analyzer";
import * as schema from "./schemaSpec";

interface PropertySchema {
    description?: string;
    type?: string;
    willReplaceOnChanges?: boolean;
    items?: { type: string }; // For arrays
    $ref?: string; // For references to other schemas
}

interface ResourceSchema {
    isComponent: boolean;
    description?: string;
    inputProperties: {
        [propertyName: string]: PropertySchema;
    };
    requiredInputs: string[];
    properties: {
        [propertyName: string]: PropertySchema;
    };
    required: string[];
}

interface TypeSchema {
    type: "object";
    properties: {
        [propertyName: string]: PropertySchema;
    };
    required: string[];
}

interface LanguageSchema {
    [language: string]: {
        dependencies?: { [packageName: string]: string };
        devDependencies?: { [packageName: string]: string };
        respectSchemaVersion?: boolean;
    };
}

interface Schema {
    name: string;
    displayName: string;
    version: string;
    resources: {
        [resourceName: string]: ResourceSchema;
    };
    types: {
        [typeName: string]: TypeSchema;
    };
    language: LanguageSchema;
}


function generateComponent(pkg: string, component: analyzer.ComponentSchema): schema.ResourceDefinition {
    const result: schema.ResourceDefinition = {
        isComponent: true,
        description: component.description,
        inputProperties: {},
        requiredInputs: Object.keys(component.inputs).filter((k) => !component.inputs[k].optional),
        properties: {},
        required: [],
    };
    for (const propName in component.inputs) {
        const inputProp = component.inputs[propName];
        const prop = generateProperty(pkg, inputProp);
        result.inputProperties![propName] = prop;
    }
    for (const output in component.outputs) {
        const outputSchema = component.outputs[output];
        result.properties![output] = {
            description: outputSchema.description,
            type: outputSchema.type,
        };
        if (!outputSchema.optional) {
            result.required!.push(output);
        }
    }
    return result;
}

export function generateSchema(pack: any, path: string): schema.PulumiPackage {
    const result: schema.PulumiPackage = {
        name: pack.name,
        displayName: pack.description,
        pluginDownloadURL: path,
        version: pack.version,
        resources: {},
        types: {},
        language: {
            nodejs: {
                dependencies: {},
                devDependencies: {
                    "typescript": "^3.7.0",
                },
                respectSchemaVersion: true,
            },
        },
    };
    const components = new analyzer.ComponentAnalyzer(path).analyzeComponents();    
    for (const component in components) {
        const tok = `${pack.name}:index:${component}`;
        result.resources![tok] = generateComponent(pack.name, components[component]);
        for (const type in components[component].typeDefinitions) {
            const typeDef = components[component].typeDefinitions[type];
            const typ: schema.TypeDefinition = {
                type: "object",
                properties: typeDef.properties as Record<string, schema.PropertyDefinition>,
                required: Object.keys(typeDef.properties).filter((k) => !typeDef.properties[k].optional),
            };
            for (const propName in typeDef.properties) {
                const prop = generateProperty(pack.name, typeDef.properties[propName]);
                typ.properties![propName] = prop;
            }
            result.types![`${pack.name}:index:${type}`] = typ;
        }
    }
    return result;
}

function generateProperty(pkg: string, inputSchema: analyzer.SchemaProperty): schema.PropertyDefinition {
    let type = inputSchema.type;
    let items: schema.TypeDefinition | undefined = undefined;
    let ref = undefined;
    if (inputSchema.ref) {
        ref = `#/types/${pkg}:index:${inputSchema.ref}`;
    } else if (type && type.endsWith("[]")) {
        items = { type: type.slice(0, -2) as schema.Type };
        type = "array";
    }
    return {
        description: inputSchema.description,
        type: type,
        items: items,
        $ref: ref,
    };
}