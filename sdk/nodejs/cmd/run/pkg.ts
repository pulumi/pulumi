/** 
 *  Returns true if the package.json file has installation instructions
 *  for the given package name.
 * @interal 
 */
export function hasPkgDeclared(name: string, pkg: Record<string, any>): boolean {
  const deps = pkg['dependencies'] ?? {};
  return name in deps;
}
