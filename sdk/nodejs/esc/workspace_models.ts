// Copyright 2025, Pulumi Corporation.  All rights reserved.

/*
Models for Pulumi workspace and account logic for python SDK.
This is a partial port of ESC and Pulumi CLI code found in
https://github.com/pulumi/esc/tree/main/cmd/esc/cli/workspace
*/

export interface Account {
    accessToken: string;
    username: string;
    organizations: string;
}
  
export interface Credentials {
  current: string;
  accessTokens: { [key: string]: string };
  accounts: { [key: string]: Account };
}

export interface EscCredentials {
    name: string;
}
