import * as morph from 'ts-morph'
import joi from 'joi'

import * as commands from './commands'

// -- TYPES

// Top-level specification of a command-line application.
export type Specification = {
  // The name of (or path to) the executable CLI program.
  executable_path: string,
  // The structure of the command tree within the CLI application.
  structure: commands.Structure,
}

// -- VALIDATION

// A schema for parsing CLI specifications.
export const schema: joi.ObjectSchema<Specification> = joi.object({
  executable_path: joi.string().required(),
  structure: commands.schema.required(),
})

// -- RENDERING

// Create an API according to the given specification.
export const create = ({ executable_path, structure }: Specification, file: morph.SourceFile) =>
  commands.create(structure, file, { executable: executable_path, path: [], flags: {} })
