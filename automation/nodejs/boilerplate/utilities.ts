// @ts-ignore
import { LanguageServer, maxRPCMessageSize } from '@pulumi/pulumi/automation/server'
import * as langrpc from '@pulumi/pulumi/proto/language_grpc_pb'

import * as grpc from "@grpc/grpc-js";
import child_process from 'child_process'
import { promisify } from 'util'

// For each command we run, we expect to get back both the standard output and
// standard error.
export type Output = { stdout: string, stderr: string }

// The type of a Pulumi program as an inline function.
export type PulumiFn = () => Promise<Record<string, any> | void>

// A Promise-based executor for child process commands.
export const execute = (command: string, args: string[]): Promise<Output> =>
  promisify(child_process.exec)([command, ... args].join(' '))

// Like `execute`, but we'll run the given program as a separate host, and
// then tell Pulumi to use that host instead of starting up its own. This
// lets us run inline programs within the automation API.
//
// TODO: this needs to be cleverer when we start wanting `execute` to log the
  // correct `exec-kind`.
export const inline = async (program: PulumiFn, command: string, args: string[]): Promise<Output> => {
  const server = new grpc.Server({ "grpc.max_receive_message_length": maxRPCMessageSize })
  const languageServer = new LanguageServer(program)

  server.addService(langrpc.LanguageRuntimeService, languageServer)
  const port: number = await new Promise<number>((resolve, reject) =>
    server.bindAsync(
      '127.0.0.1:0',
      grpc.ServerCredentials.createInsecure(),
      (err, p) => err ? reject(err) : resolve(p)))

  args.unshift('--exec-kind', 'auto.inline')
  args.unshift(`--client=127.0.0.1:${port}`)

  const result = await execute(command, args)
  server.forceShutdown()

  // TODO: actually handle an error.
  languageServer.onPulumiExit(false)
  return result
}
