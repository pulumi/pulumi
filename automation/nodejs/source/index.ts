import * as commander from 'commander'
import * as json from 'load-json-file'
import * as morph from 'ts-morph'
import cpy from 'cpy'

import { create, schema } from './specification'

// Options that control the running of the SDK generator.
type Options = {
  // An output path for the SDK.
  output?: string
}

// Parse the given specification file, and generate an SDK for interacting with
// the given command-line interface.
const handle = async (path: string, options: Options) => {
  // The output directory.
  const output = options.output || './output'

  // Create the TypeScript program and the output directory.
  const project = new morph.Project()
  const root = project.createDirectory(output)

  // First, we need to copy across all the boilerplate files. These are just
  // utilities and helper functions for generating more regular code.
  await cpy('boilerplate/*', output, { overwrite: true })

  // With everything else set up, we can create the file for the API output,
  // and start with any imports for the boilerplate code.
  const index = root.createSourceFile(output + '/index.ts', '', { overwrite: true })

  // Currently, the only boilerplate file is the utilities file, which we
  // always want imported into the CLI.
  index.addImportDeclaration({
    moduleSpecifier: './utilities',
    namedImports: ['Output', 'PulumiFn', 'execute', 'inline']
  })

  // Try to parse the input and validate it as a CLI specification.
  const file = await json.loadJsonFile(path)
  const spec = await schema.validateAsync(file)

  create(spec, index)
  project.save()
}

// In case we ever want to make this program more complicated, we have a basic
// `commander` CLI setup.
commander
  .program
  .argument('<path>', 'The path to a CLI specification')
  .option('-o, --output <path>', 'An output path for the SDK.', './output')
  .action(handle)
  .parse()
