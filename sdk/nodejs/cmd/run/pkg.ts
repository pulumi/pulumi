import * as log from "../../log";

/** 
 * @internal 
 * Returns true if the package.json file has installation instructions
 * for the given package name.
 */
export function hasPkgDeclared(name: string, pkg: Record<string, any>): boolean {
  const deps = pkg['dependencies'] ?? {};
  return name in deps;
}

interface LoaderArgs {
  moduleName: string;
  defaultLoader(name: string): any; // TODO(@Robbie): Change this type.
  pkg: Record<string, any>;
}


/** 
 * @internal 
 * loadOrDefault accepts a module name, a thunk to load a fallback module,
 * and a package.json file. If the module name isn't found in the package.json
 * file (i.e. if the user hasn't installed the given package), then the fallback
 * package is loaded instead using the thunk. This allows us to prefer a user's
 * package version if they provide it, or fall back to a backup version if they
 * do not.
 */
export function loadOrDefault(args: LoaderArgs): any {
  const name = args.moduleName;
  if(hasPkgDeclared(name, args.pkg)) {
    return require(name);
  }
  return args.defaultLoader(name);
}

const defaultLoader = (name: string): any => require(name);

/**
  * @internal
  * This function will load TS-Node. It will use a version of TS-Node
  * provided by the user if one exists. Otherwise, it will use a fallback
  * copy installed with this package.
  * It accepts the user's loaded package.json file as an argument.
  */
export function loadTSNode(pkg: Record<string, any>): any {
  log.warn("Loading TS-Node");
  return loadOrDefault({
    moduleName: "ts-node",
    defaultLoader: (name: string) => {
      log.warn("Falling backt o Pulumi TS-Node.");
      return defaultLoader("pulumi-ts-node");
    },
    pkg,
  });  
}

export function loadTypeScript(pkg: Record<string, any>): any {
  log.warn("Loading TypeScript.");
  return loadOrDefault({
    moduleName: "typescript",
    defaultLoader: (name: string) => {
      log.warn("Falling back to Pulumi TypeScript.");
      return defaultLoader("pulumi-typescript");
    },
    pkg,
  });  
}
