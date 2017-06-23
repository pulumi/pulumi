// Copyright 2016-2017, Pulumi Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import {Region} from "../types";

// region configures the target region for a deployment.  The provider explicitly does not recognize AWS_REGION,
// to minimize the possibility of accidental deployment differences due to a changing environment variable.
export let region: Region | undefined;

// requireRegion fetches the AWS region, requiring that it exists; if it has not been configured, an error is thrown.
export function requireRegion(): Region {
    if (region === undefined) {
        throw new Error("No AWS region has been configured");
    }
    return region;
}

// accessKeyId configures the access key ID used to access AWS.  This is a secret.  If not provided, the
// provider will look in the standard places (~/.aws/credentials, AWS_ACCESS_KEY_ID, etc).
export let accessKeyId: string | undefined;

// secretAcessKey configures the secret access key used to access AWS.  This is a secret.  If not provided, the
// provider will look in the standard places (~/.aws/credentials, AWS_SECRET_ACCESS_KEY, etc).
export let secretAccessKey: string | undefined; // the secret access key used to access AWS.

