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

/**
 * Support for `pulumi policy scan`, which runs a program with mocks to check its code against policy packs. No
 * provider runs during a scan, so values that only a provider or another stack can give are unknown, and the
 * program keeps running where it would otherwise fail.
 *
 * Everything here is inert unless `PULUMI_POLICY_SCAN` is set; the scan driver sets it.
 *
 * @internal
 */

/**
 * The marker a value from {@link scanUnknown} answers to. The serializer turns such values into unknowns.
 *
 * @internal
 */
export const scanUnknownSymbol = Symbol.for("pulumi.policyScan.unknown");

/**
 * What the scan driver gives the SDK, and what the SDK reports back to it.
 *
 * @internal
 */
export interface PolicyScanState {
    /** Data source results by full token or function name; fields left out are unknown. */
    invokes: Record<string, any>;
    /** Config keys the program required but nobody set. */
    missingConfig: Set<string>;
    /** Fixtures the program used. */
    usedFixtures: Set<string>;
    /** Data sources the program called that have no fixture. */
    unknownInvokes: Set<string>;
    /** `apply` callbacks on unknown values that ran with probes. */
    probedApplies?: number;
    /** Probed callbacks that threw, so their result stayed unknown. */
    failedApplies?: number;
}

/**
 * True when the program runs under `pulumi policy scan`.
 *
 * @internal
 */
export function isPolicyScan(): boolean {
    return !!process.env.PULUMI_POLICY_SCAN;
}

/**
 * The scan's shared state. It lives on `globalThis` so the driver and every copy of the SDK it loads see the same
 * object.
 *
 * @internal
 */
export function policyScanState(): PolicyScanState {
    const g = globalThis as any;
    if (!g.__pulumiPolicyScan) {
        g.__pulumiPolicyScan = {};
    }
    const s = g.__pulumiPolicyScan;
    s.invokes = s.invokes || {};
    s.missingConfig = s.missingConfig || new Set<string>();
    s.usedFixtures = s.usedFixtures || new Set<string>();
    s.unknownInvokes = s.unknownInvokes || new Set<string>();
    return s;
}

/**
 * A value that supports whatever the program does with it (property access, calls, iteration, string conversion)
 * and serializes as an unknown. Invoke results are these when there's no fixture, since no provider runs.
 *
 * @internal
 */
export function scanUnknown(): any {
    const target = function () {
        return undefined;
    };
    return new Proxy(target, {
        get(_t, p) {
            if (p === scanUnknownSymbol) {
                return true;
            }
            if (p === "then" || p === "toJSON" || p === "constructor") {
                return undefined;
            }
            if (p === "length") {
                return 0;
            }
            if (p === Symbol.iterator) {
                return function* () {
                    // No elements.
                };
            }
            if (p === Symbol.toPrimitive || p === "toString" || p === "valueOf") {
                return () => "[unknown]";
            }
            if (typeof p === "symbol") {
                return undefined;
            }
            return scanUnknown();
        },
        apply() {
            return scanUnknown();
        },
        construct() {
            return scanUnknown();
        },
    });
}

/**
 * True for a value from {@link scanUnknown}, or any other stand-in that answers to {@link scanUnknownSymbol}.
 *
 * @internal
 */
export function isScanUnknown(v: any): boolean {
    return (typeof v === "function" || (typeof v === "object" && v !== null)) && v[scanUnknownSymbol] === true;
}

/**
 * A fixture object whose missing fields read as unknown.
 *
 * @internal
 */
export function scanPartial(value: any): any {
    if (value === null || typeof value !== "object" || Array.isArray(value)) {
        return value;
    }
    return new Proxy(value, {
        get(t, p) {
            if (typeof p === "symbol" || p in t || p === "then" || p === "toJSON" || p === "constructor") {
                const v = (t as any)[p];
                return v !== null && typeof v === "object" && !Array.isArray(v) ? scanPartial(v) : v;
            }
            return scanUnknown();
        },
    });
}

/**
 * The result of an invoke during a scan: its fixture, by full token or function name
 * (`aws:index/getCallerIdentity:getCallerIdentity` or `getCallerIdentity`), or else an unknown.
 *
 * @internal
 */
export function scanInvokeResult(tok: string): { result: any; isKnown: boolean } {
    const state = policyScanState();
    for (const key of [tok, tok.split(":").pop()!]) {
        if (Object.prototype.hasOwnProperty.call(state.invokes, key)) {
            state.usedFixtures.add(`invoke ${key}`);
            return { result: scanPartial(state.invokes[key]), isKnown: true };
        }
    }
    state.unknownInvokes.add(tok);
    return { result: scanUnknown(), isKnown: false };
}

/**
 * The value for required config nobody set during a scan. The program keeps running with a placeholder, and the key
 * is reported.
 *
 * @internal
 */
export function scanMissingConfig<T>(fullKey: string, placeholder: T): T {
    policyScanState().missingConfig.add(fullKey);
    return placeholder;
}
