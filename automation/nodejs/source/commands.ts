import * as morph from 'ts-morph'
import joi from 'joi'
import pascalize from 'pascalize'

import * as execution from './execution'
import * as options from './options'
import * as parameters from './parameters'
import * as utilities from './utilities'

// -- TYPES

// CLI commands are either command menus or leaf commands.
export type Structure = Menu | Command

// A menu is a command that groups other commands. Specifically, if you want to
// execute anything against a menu, you need to run one of its subcommands.
export type Menu = {
  type: "menu",
  // The subcommands available in this menu.
  commands: { [subcommand: string]: Structure },
  // The flags made available at this level of the menu.
  // TODO: this should probably be some kind of set to figure out which of
  // these are overrides of the ones in parent scopes, but we'll worry about
  // that later.
  flags?: { [key: string]: options.Option },
}

// A leaf command is something that can be executed. It takes a set of
// arguments and has a set of available flags.
export type Command = {
  type: "command",
  // Any optional documentation about what this command does.
  documentation?: string,
  // The positional arguments of this particular command.
  // If not provided, defaults to an empty heterogeneous structure.
  arguments?: parameters.Parameters,
  // The flags made available for this command.
  // TODO: this should also probably be some kind of set.
  flags?: { [key: string]: options.Option },
  // Arguments/flags that we would like to add as arguments to every request
  // regardless.
  preset_arguments: string[],
  // TODO: we could generalise this to something more like a plugin interface
  // and then this library would not be tied to Pulumi at all.
  runs_pulumi_program: boolean
}

// -- VALIDATION

// TODO: should we be able to document command menus? Where should the
// documentation go?
export const schema: joi.ObjectSchema<Structure> = joi.object({
  type: joi.string().valid('menu', 'command').required()
}).when(joi.object({ type: joi.valid('menu') }).unknown(), {
  then: joi.object({
    // Commands are a recursive structure: every subcommand should adhere to
    // the same schema as the parent command.
    commands: joi.object().pattern(joi.any(), joi.link('...')).required(),
    flags: joi.object().pattern(joi.string(), options.schema)
  })
}).when(joi.object({ type: joi.valid('command') }).unknown(), {
  then: joi.object({
    documentation: joi.string(),
    arguments: parameters.schema,
    flags: joi.object().pattern(joi.string(), options.schema),
    preset_arguments: joi.array().items(joi.string()).default([]),
    // TODO: see type declaration
    runs_pulumi_program: joi.boolean().default(false),
  })
})

// -- RENDERING

// As we walk the hierarchy, we need to keep track of both the path to our
// current position /and/ the currently available flags.
//
// TODO: I know there are CLIs like `git` that have position-sensitive flags -
// for example, `git -p add -p` has two `p` flags that mean different things,
// and that's because of where they're applied in relation to subcommands.
// Really, we ought to apply these flags as we go, but we'd probably need to
// get into continuation-passing or more complex state objects, and neither
// seem worth the effort in a hackathon.
export type State = {
  // The name/path of the executable binary so we can carry it down to the
  // leaves.
  executable: string,
  // A "path" through the command menu tree. For example, the command
  // `pulumi state move` is considered to be the `move` command in the `state`
  // menu of the `pulumi` executable.
  path: string[],
  // TODO: this probably shouldn't exist as we should already have applied all
  // the flags from other command menus at this point. Perhaps what we actually
  // need is some kind of continuation or difflist for building the args list.
  // A job for later.
  flags: { [key: string]: options.Option }
}

// Create a CLI command in the API based on a command specification structure.
export const create = (structure: Structure, file: morph.SourceFile, state: State) => {
  options.create(structure, file, state)

  switch (structure.type) {
    case 'menu': return menu(structure, file, state)
    case 'command': return command(structure, file, state)
  }
}

// Recursively walk through a command menu and augment the state for the
// subcommands. At the moment, a command menu doesn't actually generate any
// "output", but instead just modifies the output of its subcommands.
//
// TODO: is this right? We might want to check whether some command menus are
// "executable" in some way other than for showing help text, but we'll get to
// that later.
const menu = (structure: Menu, file: morph.SourceFile, { executable, path, flags }: State) => {
  for (const [ name, specification ] of Object.entries(structure.commands)) {
    const state_ = { executable, path: [... path, name], flags: { ...flags, ...structure.available_flags } }

    create(specification, file, state_)
  }
}

// Generate some TypeScript for the actual command. The name is based on the
// subcommand menu path, which allows us to "avoid" namespace problems.
//
// TODO: this doesn't actually help us to avoid namespace problems in practice
// as subcommands can have underscores in their name.
const command = (structure: Command, file: morph.SourceFile, state: State) => {
  const name = utilities.identifier(state.path.join('_'))
  const docs = structure.documentation ? [ structure.documentation ] : []

  // The function object that we're generating for this command.
  const target = file.addFunction({ name, docs, returnType: 'Promise<Output>', isExported: true })
  const state_ = { ... state, flags: { ...state.flags, ...structure.available_flags } }

  // Next, we need to generate the parameters for this function. This will
  // either be a homogeneous `args` or a heterogeneous set of positional
  // arguments, followed by an options object for the flags.
  // If no arguments are specified, default to an empty heterogeneous structure.
  const args = structure.arguments || {
    type: "heterogeneous" as const,
    specifications: [],
    required_arguments: 0
  }

  parameters.create(target, args, state_)
  execution.create(target, args, structure.preset_arguments, state_)

  // At this point, we need to check if this is a function someone might want
  // to run with an inline program. In that case, we also want to create a
  // secondary version of the function that allows for inline programs.
  // TODO: generalise this out of Pulumi
  if (structure.runs_pulumi_program) {
    const target = file.addFunction({ name: name + "Inline", docs, returnType: 'Promise<Output>', isExported: true })

    parameters.inline(target, args, state_)
    execution.inline(target, args, structure.preset_arguments, state_)
  }
}
