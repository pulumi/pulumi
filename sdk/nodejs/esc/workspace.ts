// Copyright 2025, Pulumi Corporation.  All rights reserved.

import path from "path";
import fs from "fs";
import { Account, Credentials, EscCredentials } from "workspace_models";

/*
Pulumi workspace and account logic for python SDK.
This is a partial port of ESC and Pulumi CLI code found in
https://github.com/pulumi/esc/tree/main/cmd/esc/cli/workspace
*/

// Returns the path of the ".pulumi" folder where Pulumi puts its artifacts.
export function getPulumiHomeDir(): string {
  // Allow the folder we use to be overridden by an environment variable
  const dirEnv = process.env.PULUMI_HOME;
  if (dirEnv) {
    return dirEnv;
  }

  // Otherwise, use the current user's home dir + .pulumi
  const homeDir = process.env.HOME || process.env.USERPROFILE || process.env.HOMEPATH;
  if (!homeDir) {
    throw new Error('Unable to determine home directory.');
  }

  return path.join(homeDir, '.pulumi');
}

// Returns the path of the ".esc" folder inside Pulumi home dir.
export function getEscBookkeepingDir(): string {
  const homeDir = getPulumiHomeDir();
  return path.join(homeDir, '.esc');
}

// Returns the path to the esc credentials file on disk.
export function getPathToCredsFile(dir: string): string {
  return path.join(dir, 'credentials.json');
}

// Returns the current account name from the ESC credentials file.
export function getEscCurrentAccountName(): string | null {
  try {
    const credsFile = getPathToCredsFile(getEscBookkeepingDir());
    if (!fs.existsSync(credsFile)) {
        return null
    }
    const parsedData = JSON.parse(fs.readFileSync(credsFile, 'utf-8'));
    return (parsedData as EscCredentials)?.name;
  } catch (error) {
    console.error(`An unexpected error occurred: ${error}`);
    return null;
  }
}

// Reads and parses credentials from the Pulumi credentials file.
export function getStoredCredentials(): Credentials | null {
  try {
    const credsFile = getPathToCredsFile(getPulumiHomeDir());
    if (!fs.existsSync(credsFile)) {
        return null
    }
    const parsedData = JSON.parse(fs.readFileSync(credsFile, 'utf8'));
    return parsedData as Credentials
  } catch (error) {
    console.error(`An unexpected error occurred: ${error}`);
    return null;
  }
}

// Gets current account values from credentials file.
export function getCurrentAccount(): {account?: Account, backendUrl?: string } {
  let backendUrl = getEscCurrentAccountName();
  const pulumiCredentials = getStoredCredentials();
  if (!pulumiCredentials) {
    return {};
  }
  if (!backendUrl) {
    backendUrl = pulumiCredentials.current;
  }
  if (!backendUrl || !(backendUrl in pulumiCredentials.accounts)) {
    return {};
  }
  return {
    account: pulumiCredentials.accounts[backendUrl],
    backendUrl,
  };
}
