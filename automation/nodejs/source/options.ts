import * as morph from 'ts-morph'
import joi from 'joi'
import pascalize from 'pascalize'

import * as utilities from './utilities'
import * as commands from './commands'

// -- TYPES

// A command-line option. Right now, rwe only recognise two types of
// command-line options: regular options (a key and a string value) and flags,
// which are implicitly boolean flags whose presence implies `true`.
export type Option = utilities.RequireAtLeastOne<Option_, 'longName' | 'shortName'>
export type Option_ = {
  longName: string | null,
  shortName: string | null,
  repeatable?: boolean
  documentation?: string,
  // TODO: we need to think about handling arrays. Specifically, if we think
  // about the `--target` flag, we can supply it many types to `pulumi` to add
  // values to the `targets` array.
  type: 'string' | 'int' | 'number' | 'boolean',
}

// -- VALIDATION

// A parser for validating option specifications.
export const schema = joi.object({
  type: joi.string().valid('string', 'int', 'number', 'boolean').default('boolean'),

  longName: joi.string(),
  shortName: joi.string(),
  repeatable: joi.boolean().default(false),
  documentation: joi.string(),
}).or('longName', 'shortName')

// -- RENDERING

// Generate an options type for the given command, including all the known
// availabel flags.
export const create = (structure: commands.Structure, file: morph.SourceFile, state: commands.State) => {
  file.addInterface({
    name: generateOptionsInterfaceName(state.path),
    isExported: true,
    properties: Object.values(structure.flags || {}).map(flag => ({
      docs: flag.documentation ? [ flag.documentation ] : undefined,
      hasQuestionToken: true,
      name: utilities.identifier(flag.longName || flag.shortName || ''),
      type: utilities.toTypeScriptType(flag.type),
    })),
    extends: [generateParentOptionsInterfaceName(state.path) || ''],
  })

}

const generateOptionsInterfaceName = (path: string[]): string =>
  pascalize([... path, "options"].join('_'))

const generateParentOptionsInterfaceName = (path: string[]): string | null => {
  if (path.length === 0) {
    return null
  }

  const parent = path.slice(0, -1)
  return generateOptionsInterfaceName(parent)
}