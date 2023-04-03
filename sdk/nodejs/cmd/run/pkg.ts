/** 
 * Returns true if the package.json file has installation instructions
 * for the given package name.
 * @interal 
 */
export function hasPkgDeclared(name: string, pkg: Record<string, any>): boolean {
  const deps = pkg['dependencies'] ?? {};
  return name in deps;
}

// TODO(@Robbie): Delete this const.
const loader = (name: string): boolean => require(name);

interface LoaderArgs {
  moduleName: string;
  defaultLoader(name: string): any; // TODO(@Robbie): Change this type.
  pkg: Record<string, any>;
}


/** 
 * loadOrDefault accepts a module name, a thunk to load a fallback module,
 * and a package.json file. If the module name isn't found in the package.json
 * file (i.e. if the user hasn't installed the given package), then the fallback
 * package is loaded instead using the thunk. This allows us to prefer a user's
 * package version if they provide it, or fall back to a backup version if they
 * do not.
 * @interal 
 */
export function loadOrDefault(args: LoaderArgs): any {
  const name = args.moduleName;
  if(hasPkgDeclared(name, args.pkg)) {
    return require(name);
  }
  return args.defaultLoader(name);
}