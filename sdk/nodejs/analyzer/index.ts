// Copyright 2016-2018, Pulumi Corporation.
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

import { ID, Output, parseUrn, Resource, URN } from "../resource";
import { deserializeProperties, initResource, resolveProperties } from "../runtime";

const grpc = require("grpc");
const analyzerProto = require("../proto/analyzer_pb.js");
const analyzerRPC = require("../proto/analyzer_grpc_pb.js");
const pluginProto = require("../proto/plugin_pb.js");

/**
 * analyzers is the complete set of active analyzers that will be run in response to engine calls. A single plugin
 * may host one or many of them and they are guaranteed to run in the order in which they are registered with add.
 */
const analyzers: Analyzer[] = [];

/**
 * Analyzer is a pluggable checker that can analyze individual resources, or entire resource graphs, to find
 * problems at various points of the resource management lifecycle (during initial deployment, during updates,
 * after the fact, etc). Because they run arbitrary code, they can query anything needed to answer questions.
 *
 * TODO: wire up parents/childs.
 * TODO: graph view of the resources.
 * TODO: communicate the action being taken, including old vs new state.
 */
export interface Analyzer {
    /**
     * A friendly name for this analyzer check.
     */
    readonly name: string;
    /**
     * An optional description about what this analyzer does.
     */
    readonly description?: string;
    /*
     * analyze inspects a single resource and optionally returns zero to many diagnostics about them
     */
    readonly analyze?: (resource: Resource) => AnalyzerReturn;
    /**
     * analyzeStack inspects the entire set of resources in a stack and returns zero to many diagnostics about them.
     */
    readonly analyzeStack?: (resources: Resource[]) => AnalyzerReturn;
}

/**
 * AnalyzerResult is a diagnostic, array of diagnostics, or nothing.
 */
export type AnalyzerResult = Diagnostic | Diagnostic[] | undefined | void;

/**
 * AnalyzerReturn is the return type of a analyzer method, carrying optional diagnostic information, in either raw form,
 * or wrapped in an Output<T> or Promise<T> type.
 */
export type AnalyzerReturn = AnalyzerResult | Output<AnalyzerResult> | Promise<AnalyzerResult>;

/**
 * Diagnostic represents a potential problem that an analyzer has uncovered. It includes enough information to
 * convey the identity of the diagnostic in addition to metadata about its severity, category, and confidence.
 */
export interface Diagnostic {
    /**
     * An optional ID unique to this analyzer for classification. This can be useful if a particular analyzer
     * might report one of many different error types, and can be used in filtering or further classification.
     */
    id?: string;
    /**
     * A freeform message describing the issue discovered by an analyzer.
     */
    message: string;
    /**
     * The severity of this diagnostic, indicating how seriously it is to be taken.
     */
    severity: DiagnosticSeverity;
    /**
     * A category that classifies this diagnostic into one of several well-defined diagnostic categories.
     */
    category: DiagnosticCategory;
    /**
     * A confidence score from 0.0 (meaning no confidence) to 1.0 (meaning perfect confidence), conveying the
     * likelihood that this diagnostic actually represents an error. This may be used in filtering to weed out
     * errors that may not be actionable or whose likelihood of being an actual problem are low.
     */
    confidence?: number;
}

/**
 * DiagnosticSeverity represents the set of out-of-the-box sevirities indicating how serious a diagnostic is.
 */
export type DiagnosticSeverity = "low" | "medium" | "high" | "critical";

/**
 * DiagnosticCategory represents the set of out-of-the-box categories that classify the kind of diagnostics.
 */
export type DiagnosticCategory =
    "compliance"  |
    "design"      |
    "naming"      |
    "performance" |
    "reliability" |
    "security"
;

/**
 * add registers a new analyzer under the given name. This will run in response to analysis requests that arrive from
 * the engine. In order to listen for those events, make sure to call the listen function before your program exits.
 */
export function add(a: Analyzer): void {
    analyzers.push(a);
}

/**
 * listen spawns an RPC server that allows the engine to connect and issue analysis requests. It will print out the
 * port so that the engine can discover the right endpoint to connect to; no stdout messages may precede this call.
 */
export function listen(): void {
    const server = new grpc.Server();
    server.addService(analyzerRPC.AnalyzerService, {
        analyze,
        analyzeStack,
        getPluginInfo,
    });

    const port = server.bind("0.0.0.0:0", grpc.ServerCredentials.createInsecure());
    server.start();

    console.log(`${port}`);
}

// analyze is the RPC call that will analyze an individual resource, one at a time (i.e., check).
async function analyze(call: any, callback: any): Promise<void> {
    // Prep to perform the analysis.
    const req = call.request;
    const res = new AnalyzableResource(req.getUrn(), req.getId(), req.getProperties());

    // Run the analysis for every analyzer in the global list, tracking any diagnostics.
    const ds: Diagnostic[] = [];
    try {
        for (const a of analyzers) {
            if (a && a.analyze) {
                const d = await getDiagnosticsFromReturn(a.analyze(res));
                if (d && d.length) {
                    ds.push(...d);
                }
            }
        }
    }
    catch (err) {
        callback(err, undefined);
        return;
    }

    // Now marshal the results into a resulting diagnostics list, and invoke the callback to finish.
    callback(undefined, createAnalyzeResponse(ds));
}

// analyzeStack is the RPC call that will analyze a whole stack.
async function analyzeStack(call: any, callback: any): Promise<void> {
    // Prep to perform the analysis.
    const req = call.request;
    const resources: AnalyzableResource[] = [];
    for (const res of req.getResources()) {
        resources.push(new AnalyzableResource(req.getUrn(), req.getId(), req.getProperties()));
    }

    // Run the analysis for every analyzer in the global list, tracking any diagnostics.
    const ds: Diagnostic[] = [];
    try {
        for (const a of analyzers) {
            if (a && a.analyzeStack) {
                const d = await getDiagnosticsFromReturn(a.analyzeStack(resources));
                if (d && d.length) {
                    ds.push(...d);
                }
            }
        }
    }
    catch (err) {
        callback(err, undefined);
        return;
    }

    // Now marshal the results into a resulting diagnostics list, and invoke the callback to finish.
    callback(undefined, createAnalyzeResponse(ds));
}

// getPluginInfo returns the version information about this particular analyzer.
async function getPluginInfo(call: any, callback: any): Promise<void> {
    const pinfo = new pluginProto.PluginInfo();
    pinfo.setVersion("1.0.0"); // TODO: fetch this from somewhere (package.json?)
    callback(undefined, pinfo);
}

// getDiagnosticsFromReturn simply turns a return into a promise of diagnostics, possibly empty. This handles the myriad
// cases that an analyzer might use when producing the diagnostics (promises, outputs, etc).
async function getDiagnosticsFromReturn(d: AnalyzerReturn): Promise<Diagnostic[]> {
    if (d) {
        // First unpack the result from the output, promise, or raw value.
        let v: AnalyzerResult;
        if (d instanceof Promise) {
            v = await d;
        } else if ((d as Output<AnalyzerResult>).__pulumiOutput) {
            v = await (d as Output<AnalyzerResult>).promise();
        } else {
            v = (d as Diagnostic | Diagnostic[]);
        }

        // Now turn that into an array.
        if (v) {
            if (Array.isArray(v)) {
                return v;
            }
            return [ v ];
        }
    }

    // If the result is empty, just return an empty array.
    return [];
}

// createAnalyzeResponse creates a protobuf encoding the given list of diagnostics.
function createAnalyzeResponse(ds: Diagnostic[] | undefined) {
    const resp = new analyzerProto.AnalyzeResponse();
    const dplist = [];
    if (ds) {
        for (const d of ds) {
            const dp = new analyzerProto.AnalyzerDiagnostic();
            dp.setId(d.id);
            dp.setMessage(d.message);
            dp.setSeverity(d.severity);
            dp.setCategory(d.category);
            dp.setConfidence(d.confidence);
            dplist.push(dp);
        }
    }
    resp.setDiagnosticsList(dplist);
    return resp;
}

/**
 * AnalyzableResource provides a simple object that looks like a resource. The difference is that it is weakly
 * typed and is not truly under the management of the engine for purposes of the current program.
 */
class AnalyzableResource extends Resource {
    /**
     * The URN auto-assigned to this resource by the engine.
     */
    public readonly urn: Output<URN>;
    /**
     * The provider-assigned unique identifier for this resource (for custom resources only).
     */
    public readonly id?: Output<ID>;
    /**
     * The full set of resource properties, some of which may be unknown during planning.
     */
    public readonly properties: Record<string, Output<any>>;
    /**
     * A promise used internally to rendezvous with the availability of this resource's state.
     */
    // tslint:disable-next-line:variable-name
    /* @internal */ public readonly __done: Promise<void>;

    constructor(urn: URN, id: any, props: any) {
        const [ _, __, t, ___, name ] = parseUrn(urn);
        super(t, name, false, {}, { skipRegistration: true });

        // For everything but the URN, we need to deserialize the properties, and possibly generate Output<T>
        // objects (since they could be unresolved during planning phases).
        const rawProps = deserializeProperties(props);

        // Initialize this resource object asynchronously and prepare to resolve the properties.
        const [resolveURN, resolveID, resolvers] = initResource(
            `AnalyzableResource(${urn}, ...)`, this, true, rawProps);

        // Finally, actually perform the resolutions for all properties.
        resolveURN(urn);
        resolveID!(id, id !== undefined);
        resolveProperties(this, resolvers, t, name, rawProps);
    }
}
