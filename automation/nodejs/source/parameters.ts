import * as morph from 'ts-morph'
import joi from 'joi'
import pascalize from 'pascalize'
import pluralize from 'pluralize'

import * as commands from './commands'
import * as utilities from './utilities'

// -- TYPES

// Right now, we assume the only ways to express arguments in a CLI are either
// as a series of homogeneous values, /or/ as a series of heterogeneous values.
// TODO: This doesn't take into account things like "three strict parameters
// followed by any number of homegenous parameters".
export type Parameters = HeterogeneousParameters | HomogeneousParameters

// Heterogeneous parameters are those where the types or validation rules
// differ for each parameter. A number of them may be required, b,ut further
// commands can be seen as strictly optional, and if unsupplied, should not
// appear in the final output.
export type HeterogeneousParameters = {
  type: "heterogeneous",
  // The specifications for each individual argument.
  specifications: Parameter[],
  // How many arguments are required? After we pass this number of arguments,
  // the rest will assumed to be optional and missing values will not be passed
  // through to the CLI.
  required_arguments: number,
}

// Homogeneous parameters are parameters whose validation rules are all
// identical. Typically, there are some minimum and maximum bounds on how many
// we're allowed to supply, which we express as the `cardinality`.
export type HomogeneousParameters = {
  type: "homogeneous",
  // THe specification followed by all parameters.
  specification: Parameter,
  // The constraints on how many parameters we're allowed.
  cardinality: Cardinality,
}

// The specification of a positional CLI argument.
// TODO: how clever a type system do we need? Arguably, things are strings or
// booleans.
export type Parameter = {
  name: string,
  type: 'string' | 'int' | 'number' | 'boolean',
}

// Restrictions on how many homogeneous positional arguments we're allowed to
// apply. For exact numbers, we can simply set the `at_least` and `at_most`
// boundaries to the same number.
export type Cardinality = RequireAtLeastOne<Cardinality_, 'at_least' | 'at_most'>
export type Cardinality_ = { at_least?: number, at_most?: number }

// -- VALIDATION

// Parse a parameter specification.
const parameter = joi.object({
  name: joi.string().required(),
  type: joi.string().default('string')
})

// Parse a specification of CLI command parameters.
export const schema: joi.ObjectSchema<Parameters> = joi.object({
  type: joi.string().valid('heterogeneous', 'homogeneous').required()
}).when(joi.object({ type: joi.valid('heterogeneous') }).unknown(), {
  then: joi.object ({
    specifications: joi.array().items(parameter).required(),
    required_arguments: joi.number().integer().min(0),
  })
}).when(joi.object({ type: joi.valid('homogeneous') }).unknown(), {
  then: joi.object ({
    specification: parameter.required(),
    cardinality: joi.object({
      at_least: joi.number().integer(),
      at_most: joi.number().integer(),
    })
  })
})

// -- RENDERING

// Create the parameter for homogeneous arguments.
const homogeneous = (target: morph.FunctionDeclaration, structure: HomogeneousParameters) =>
  target.addParameter({
    name: utilities.identifier(pluralize(structure.specification.name)),
    type: structure.specification.type + "[]"
  })

// Create the parameter for heterogeneous arguments.
const heterogeneous = (target: morph.FunctionDeclaration, structure: HeterogeneousParameters) =>
  structure.specifications.forEach((specification, index) => {
    target.addParameter({
      name: utilities.identifier(specification.name),
      type: specification.type,
      hasQuestionToken: index > structure.required_arguments
    })
  })

// Create the parameters for a function declaration using the given parameter
// specification.
export const create = (target: morph.FunctionDeclaration, structure: Parameters, state: commands.State) => {
  switch (structure.type) {
    case "homogeneous":
      homogeneous(target, structure)
      break
    case "heterogeneous":
      heterogeneous(target, structure)
      break
  }

  // The final parameter is always an `Options` objectt containing all the
  // flags that are available for this command.
  const type = pascalize([... state.path, "options"].join('_'))
  target.addParameter({ name: "options", hasQuestionToken: true, type })
}

// Create the parameters for a function that also accepts an inline program.
export const inline = (target: morph.FunctionDeclaration, structure: Parameters, state: commands.State) => {
  target.addParameter({ name: "__program", type: "PulumiFn" })
  create(target, structure, state)
}
