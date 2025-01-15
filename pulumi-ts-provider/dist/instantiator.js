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
exports.instantiateComponent = instantiateComponent;
const path = __importStar(require("path"));
const analyzer_1 = require("./analyzer");
async function findComponentFile(directoryPath, componentName) {
    const analyzer = new analyzer_1.ComponentAnalyzer(directoryPath);
    // Check if this component exists in the analyzed components
    const components = analyzer.analyzeComponents();
    if (!components[componentName]) {
        return null;
    }
    // Find the source file containing this component
    const sourceFile = analyzer['program'].getSourceFiles().find(sf => !sf.fileName.includes('node_modules') &&
        !sf.fileName.endsWith('.d.ts') &&
        sf.getText().includes(`class ${componentName}`));
    return sourceFile ? sourceFile.fileName : null;
}
async function instantiateComponent(directoryPath, componentName, name, args, options) {
    const componentFile = await findComponentFile(directoryPath, componentName);
    if (!componentFile) {
        throw new Error(`Component ${componentName} not found in any source files at ${directoryPath}`);
    }
    try {
        const relativePath = path.relative(__dirname, componentFile);
        const module = await Promise.resolve(`${relativePath}`).then(s => __importStar(require(s)));
        const ComponentClass = module[componentName];
        if (!ComponentClass) {
            throw new Error(`Component ${componentName} found in ${componentFile} but could not be imported`);
        }
        return new ComponentClass(name, args, options);
    }
    catch (error) {
        const errorMessage = error instanceof Error ? error.message : String(error);
        throw new Error(`Failed to instantiate component ${componentName}: ${errorMessage}`);
    }
}
//# sourceMappingURL=instantiator.js.map