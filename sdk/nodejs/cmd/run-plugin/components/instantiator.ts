function isPulumiComponent(componentClass: any): boolean {
    // Check if the class extends ComponentResource
    try {
        let prototype = componentClass.prototype;
        while (prototype) {
            const constructor = prototype.constructor;
            if (constructor.name === 'ComponentResource') {
                return true;
            }
            prototype = Object.getPrototypeOf(prototype);
        }
    } catch (error) {
        // If any error occurs during prototype chain traversal, assume it's not a Pulumi component
        return false;
    }

    return false;
}

export function hasPulumiComponents(moduleExports: any): boolean {
    // Check all exports in the module
    for (const key in moduleExports) {
        const exportedItem = moduleExports[key];
        
        // Check if the export directly is a Pulumi component
        if (isPulumiComponent(exportedItem)) {
            return true;
        }
        
        // Check nested exports if it's an object
        if (typeof exportedItem === 'object' && exportedItem !== null) {
            for (const nestedKey in exportedItem) {
                if (isPulumiComponent(exportedItem[nestedKey])) {
                    return true;
                }
            }
        }
    }
    
    return false;
}

function findComponentClass(moduleExports: any, componentName: string): any {
    // First try direct access
    if (moduleExports[componentName]) {
        const ComponentClass = moduleExports[componentName];
        if (isPulumiComponent(ComponentClass)) {
            return ComponentClass;
        }
    }

    // Search nested exports if not found directly
    for (const key in moduleExports) {
        const exportedItem = moduleExports[key];
        if (typeof exportedItem === 'object' && exportedItem !== null) {
            if (exportedItem[componentName] && isPulumiComponent(exportedItem[componentName])) {
                return exportedItem[componentName];
            }
        }
    }

    return null;
} 

export async function instantiateComponent(moduleExports: any, componentName: string,
    name: string, args: Record<string, any>, options: any): Promise<any> {
    const ComponentClass = findComponentClass(moduleExports, componentName);        
    if (!ComponentClass) {
        throw new Error(`Component ${componentName} not found in the provided module`);
    }

    try {
        // Create a new instance with the provided args
        return new ComponentClass(name, args, options);
    } catch (error) {
        const errorMessage = error instanceof Error ? error.message : String(error);
        throw new Error(`Failed to instantiate component ${componentName}: ${errorMessage}`);
    }
}