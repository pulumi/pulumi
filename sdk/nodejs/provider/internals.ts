// Copyright 2016-2021, Pulumi Corporation.
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

// Functionality exported for unit testing.

// The parsing here is approximate for the moment.
//
// When Pulumi CLI decides how to structure command line arguments for
// plugins that will be parsed with this function, it uses the
// following code:
//
// https://github.com/pulumi/pulumi/blob/master/sdk/go/common/resource/plugin/plugin.go#L281
//
// The code can prepend `--logtostderr` and verbosity e.g. `-v=9`
// arguments. We ignore these for the moment.
/**
 * @internal
 */
export function parseArgs(args: string[]): { engineAddress: string } | undefined {
    const cleanArgs = [];

    for (let i = 0; i < args.length; i++) {
        const v = args[i];
        if (v === "--logtostderr") {
            continue;
        }
        if (v.startsWith("-v=")) {
            continue;
        }
        if (v === "--tracing") {
            i += 1;
            continue;
        }
        cleanArgs.push(v);
    }

    if (cleanArgs.length === 0) {
        return undefined;
    }

    return { engineAddress: cleanArgs[0] };
}
