import * as path from 'path';
import { ComponentAnalyzer } from './analyzer';

async function findComponentFile(directoryPath: string, componentName: string): Promise<string | null> {
    const analyzer = new ComponentAnalyzer(directoryPath);
    
    // Check if this component exists in the analyzed components
    const components = analyzer.analyzeComponents();
    if (!components[componentName]) {
        return null;
    }
    
    // Find the source file containing this component
    const sourceFile = analyzer['program'].getSourceFiles().find(sf => 
        !sf.fileName.includes('node_modules') && 
        !sf.fileName.endsWith('.d.ts') &&
        sf.getText().includes(`class ${componentName}`)
    );
    
    return sourceFile ? sourceFile.fileName : null;
}

export async function instantiateComponent(
    directoryPath: string,
    componentName: string,
    name: string,
    args: Record<string, any>,
    options: any
): Promise<any> {
    const componentFile = await findComponentFile(directoryPath, componentName);
    
    if (!componentFile) {
        throw new Error(`Component ${componentName} not found in any source files at ${directoryPath}`);
    }
    
    try {
        const relativePath = path.relative(__dirname, componentFile);
        const module = await import(relativePath);
        const ComponentClass = module[componentName];
        
        if (!ComponentClass) {
            throw new Error(`Component ${componentName} found in ${componentFile} but could not be imported`);
        }
        
        return new ComponentClass(name, args, options);
    } catch (error) {
        const errorMessage = error instanceof Error ? error.message : String(error);
        throw new Error(`Failed to instantiate component ${componentName}: ${errorMessage}`);
    }
}
