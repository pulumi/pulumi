// Copyright 2024-2024, Pulumi Corporation.
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

declare module "@npmcli/arborist" {
    interface _Node {
        name: string;
        parent: Node | null;
        children: Map<string, Node | Link>;
        package: any;
        path: string;
        realpath: string;
        location: string;
        isLink: boolean;
        isRoot: boolean;
        isProjectRoot: boolean;
        isTop: boolean;
        top: Node;
        root: Node;
        errors: Error[];
        edgesOut: Map<string, Edge>;
        workspaces: Map<string, string> | null;
    }

    interface Node extends _Node {
        isLink: false;
    }

    interface Link extends _Node {
        isLink: true;
        target: Node;
    }

    interface Edge {
        from: Node;
        type: "prod" | "dev" | "optional" | "peer";
        name: string;
        spec: string;
        to: Node;
        value: boolean;
        error: "DETACTHED" | "MISSING" | "PEER LOCAL" | "INVALID" | null;
    }

    interface Options {
        path: string;
    }

    class Arborist {
        constructor(opts: Options);
        loadActual(): Promise<Node>;
    }
}
