import { exec } from "node:child_process";
import { promisify } from "node:util";

export type Output = {
  stdout: string;
  stderr: string;
  exitCode: number;
};

// Execute the given command and return the process output.
async function __run(command: string): Promise<Output> {
  try {
    const result = await promisify(exec)(command);
    return { exitCode: 0, ... result };

  } catch ({ stdout, stderr, exitCode }: any) {
    return { stdout, stderr, exitCode };
  }
}