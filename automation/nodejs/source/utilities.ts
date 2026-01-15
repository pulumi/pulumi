import camelize from 'camelize'
import pascalize from 'pascalize'

// https://stackoverflow.com/a/49725198
export type RequireAtLeastOne<T, Ks extends keyof T = keyof T> =
    Pick<T, Exclude<keyof T, Ks>> 
    & { [K in Ks]-?: Required<Pick<T, K>> & Partial<Pick<T, Exclude<Ks, K>>> }[Ks]

const reservedWords = new Set([
  'abstract',
  'any',
  'as',
  'async',
  'await',
  'boolean',
  'break',
  'case',
  'catch',
  'class',
  'const',
  'continue',
  'constructor',
  'declare',
  'default',
  'delete',
  'do',
  'else',
  'enum',
  'export',
  'extends',
  'false',
  'finally',
  'for',
  'from',
  'function',
  'get',
  'if',
  'implements',
  'import',
  'in',
  'instanceof',
  'interface',
  'is',
  'let',
  'new',
  'null',
  'package',
  'private',
  'protected',
  'public',
  'return',
  'static',
  'super',
  'switch',
  'symbol',
  'this',
  'throw',
  'true',
  'try',
  'typeof',
  'undefined',
  'var',
  'void',
  'while',
  'with',
  'yield'
])

// Convert a name to a usable TypeScript identifier.
// TODO Try much harder to make this good.
export const identifier = (input: string): string =>
  camelize(sanitise(input))

// Convert a name to a usable TypeScript interface name.
// TODO Try much harder to make this good.
export const type = (input: string): string =>
  pascalize(sanitise(input))

const sanitise = (input: string): string => {
  const name = input.replace(/[^a-zA-Z0-9_]/g, '_')
  if (reservedWords.has(name)) {
    return `_${ name }`
  }

  return name
}

export const toTypeScriptType = (type: string): string => {
  switch (type) {
    case 'int': return 'number'
    default:
      return type
  }
}