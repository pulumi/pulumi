// Copyright 2026, Pulumi Corporation.
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

// Minimal shape of the fetch Response we consume. Declared locally because @types/node is pinned to v14 to stay
// compatible with TypeScript 3.8. This version predates the global `fetch`/`Response` types. Once @types/node is
// updated, this can be removed in favor of the standard `Response` type.
interface FetchResponse {
    readonly ok: boolean;
    readonly status: number;
    readonly statusText: string;
    text(): Promise<string>;
}

/**
 * @internal
 * Downloads the contents of `url` as a string. Follows redirects and aborts the request after 2 minutes.
 */
export async function download(url: string): Promise<string> {
    // Cast `globalThis` to access `fetch` and `AbortSignal.timeout`, which are available on Node.js 20+ (our minimum
    // supported version) but not declared in our pinned @types/node.
    const g = globalThis as unknown as {
        fetch(input: string, init?: { redirect?: "follow"; signal?: unknown }): Promise<FetchResponse>;
        AbortSignal: { timeout(ms: number): unknown };
    };

    let response: FetchResponse;
    try {
        response = await g.fetch(url, {
            redirect: "follow",
            signal: g.AbortSignal.timeout(120_000),
        });
    } catch (err) {
        const e = err as Error;
        if (e.name === "TimeoutError") {
            throw new Error(`Timed out downloading ${url}`);
        }
        throw new Error(`Failed to download ${url}: ${e.message}`);
    }
    if (!response.ok) {
        throw new Error(`Failed to download ${url}: ${response.status} ${response.statusText}`);
    }
    return await response.text();
}
