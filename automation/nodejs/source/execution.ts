import * as morph from 'ts-morph'
import pluralize from 'pluralize'

import * as commands from './commands'
import * as options from './options'
import * as parameters from './parameters'
import * as utilities from './utilities'

// Generate the argument handling for a set of homogeneous arguments.
const homogeneous = (writer: morph.CodeBlockWriter, structure: parameters.HomogeneousParameters) => {
  const name = pluralize(utilities.identifier(structure.specification.name))
  writer.writeLine(`__args.push(... ${ name })`)
}

// Generate the argument handling for a set of heterogeneous arguments.
const heterogeneous = (writer: morph.CodeBlockWriter, structure: parameters.HeterogeneousParameters) =>
  structure.specifications.forEach((spec, index) => {
    const name = utilities.identifier(spec.name)

    if (index < structure.required_arguments) {
      writer.writeLine(`__args.push(${ name })`)
    } else {
      writer.writeLine(`if (${ name } != null) {`)
      writer.indent(() => writer.writeLine(`__args.push(${ name })`))
      writer.writeLine('}')
    }
  })

// Create the function body for a given command wrapper. This has to take care
// of reading the options object, the explicit arguments, and so on.
export const create = (
  target: morph.FunctionDeclaration,
  structure: parameters.Parameters,
  presets: string[],
  state: commands.State
) => {
  // First, we have to apply all the arguments to the list of arguments being
  // passed to the CLI process. For homogeneous functions, this is very obvious
  // and straightforward. For heterogeneous arguments, howveer, we'll need to
  // generate the right code-level names.
  const args: string[] = []

  target.addStatements(writer => {
    writer.writeLine(`const __args = []`)

    // Add the subcommand path to the command.
    state.path.forEach(command => writer.writeLine(`__args.push('${ command }')`))

    // Add any preset arguments.
    presets.forEach(preset => writer.writeLine(`__args.push('${ preset }')`))

    switch (structure.type) {
      case "homogeneous":
        homogeneous(writer, structure)
        args.push('args')

        break

      case "heterogeneous":
        heterogeneous(writer, structure)

        const identify = (x: { name: string }) => utilities.identifier(x.name)
        args.push(... structure.specifications.map(identify))

        break
    }

    writer.writeLine('if (options != null) {')
    writer.indent(() => {
      Object.values(state.flags).forEach((o: options.Option) => {
        const flag = o.longName ? '--' + o.longName : '-' + o.shortName
        const name = utilities.identifier(o.longName || o.shortName || '')

        // When we encounter a boolean flag, we don't need a parameter - the
        // presence of the flag implies that it is `true`, and its absence
        // implies `false`.
        if (o.type == "boolean") {
          writer.writeLine(`if (options.${ name }) {`)
          writer.indent(() => writer.writeLine(`__args.push('${ flag }')`))
          writer.writeLine('}')
        } else {
          writer.writeLine(`if (options.${ name } != null) {`)
          writer.indent(() => writer.writeLine(`__args.push('${ flag }', options.${ name })`))
          writer.writeLine('}')
        }
      })
    })

    writer.writeLine('}')
    // Finally, we apply the list of arguments to the CLI command.
    writer.writeLine(`return execute('${ state.executable }', __args)`)
  })
}

// Like `execute`, but with an inline function argument.
//
// TODO: merge this function and `execute` with some sort of switch to
// determine the final line of the function.
export const inline = (
  target: morph.FunctionDeclaration,
  structure: parameters.Parameters,
  presets: string[],
  state: commands.State
) => {
  // First, we have to apply all the arguments to the list of arguments being
  // passed to the CLI process. For homogeneous functions, this is very obvious
  // and straightforward. For heterogeneous arguments, howveer, we'll need to
  // generate the right code-level names.
  const args: string[] = []

  target.addStatements(writer => {
    writer.writeLine(`const __args = []`)
    state.path.forEach(command => writer.writeLine(`__args.push('${ command }')`))
    presets.forEach(preset => writer.writeLine(`__args.push('${ preset }')`))

    switch (structure.type) {
      case "homogeneous":
        homogeneous(writer, structure)
        args.push('args')

        break

      case "heterogeneous":
        heterogeneous(writer, structure)

        const identify = (x: parameters.Parameter) => utilities.identifier(x.name)
        args.push(... structure.specifications.map(identify))

        break
    }

    writer.writeLine('if (options != null) {')
    writer.indent(() => {
      Object.values(state.flags).forEach((o: options.Option) => {
        if (o.repeatable) {
          const flag = o.longName ? '--' + o.longName : '-' + o.shortName
          const name = utilities.identifier(o.longName || o.shortName || '')

          // When we encounter a boolean flag, we don't need a parameter - the
          // presence of the flag implies that it is `true`, and its absence
          // implies `false`.
          if (o.type == "boolean") {
            writer.writeLine(`if (options.${ name }) {`)
            writer.indent(() => {
              writer.writeLine(`for (let value in options.${ name }) {`)
              writer.indent(() => writer.writeLine(`__args.push('${ flag }')`))
              writer.writeLine('}')
            })
            writer.writeLine('}')
          } else {
            writer.writeLine(`if (options.${ name } != null) {`)
            writer.indent(() => {
              writer.writeLine(`for (let __value in options.${ name }) {`)
              writer.indent(() => writer.writeLine(`__args.push('${ flag }', __value)`))
              writer.writeLine('}')
            })
            writer.writeLine('}')
          }
        } else {
          const flag = o.longName ? '--' + o.longName : '-' + o.shortName
          const name = utilities.identifier(o.longName || o.shortName || '')

          // When we encounter a boolean flag, we don't need a parameter - the
          // presence of the flag implies that it is `true`, and its absence
          // implies `false`.
          if (o.type == "boolean") {
            writer.writeLine(`if (options.${ name }) {`)
            writer.indent(() => writer.writeLine(`__args.push('${ flag }')`))
            writer.writeLine('}')
          } else {
            writer.writeLine(`if (options.${ name } != null) {`)
            writer.indent(() => writer.writeLine(`__args.push('${ flag }', options.${ name })`))
            writer.writeLine('}')
          }
        }
      })
    })

    writer.writeLine('}')

    // Finally, we apply the list of arguments to the CLI command.
    writer.writeLine(`return inline(__program, '${ state.executable }', __args)`)
  })
}
