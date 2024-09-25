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

import * as grpc from "@grpc/grpc-js";

import { Resource } from "./resource";
import * as utils from "./utils";

/**
 * {@link RunError} can be used for terminating a program abruptly, but
 * resulting in a clean exit rather than the usual verbose unhandled error logic
 * which emits the source program text and complete stack trace. This type
 * should be rarely used. Ideally {@link ResourceError} should always be used so
 * that as many errors as possible can be associated with a resource.
 */
export class RunError extends Error {
    /**
     * A private field to help with RTTI that works in SxS scenarios.
     *
     * @internal
     */
    // eslint-disable-next-line @typescript-eslint/naming-convention,no-underscore-dangle,id-blacklist,id-match
    public readonly __pulumiRunError: boolean = true;

    /**
     * Returns true if the given object is a {@link RunError}. This is designed
     * to work even when multiple copies of the Pulumi SDK have been loaded into
     * the same process.
     */
    public static isInstance(obj: any): obj is RunError {
        return utils.isInstance<RunError>(obj, "__pulumiRunError");
    }
}

/**
 * {@link ResourceError} can be used for terminating a program abruptly,
 * specifically associating the problem with a {@link Resource}. Depending on
 * the nature of the problem, clients can choose whether or not the call stack
 * should be hidden as well. This should be very rare, and would only indicate
 * that presenting the stack to the user would not be useful/be detrimental.
 */
export class ResourceError extends Error {
    /**
     * A private field to help with RTTI that works in SxS scenarios.
     *
     * @internal
     */
    // eslint-disable-next-line @typescript-eslint/naming-convention, no-underscore-dangle, id-blacklist, id-match
    public readonly __pulumResourceError: boolean = true;

    /**
     * Returns true if the given object is a {@link ResourceError}. This is
     * designed to work even when multiple copies of the Pulumi SDK have been
     * loaded into the same process.
     */
    public static isInstance(obj: any): obj is ResourceError {
        return utils.isInstance<ResourceError>(obj, "__pulumResourceError");
    }

    constructor(
        message: string,
        public resource: Resource | undefined,
        public hideStack?: boolean,
    ) {
        super(message);
    }
}

export function isGrpcError(err: Error): boolean {
    const code = (<any>err).code;
    return code === grpc.status.UNAVAILABLE || code === grpc.status.CANCELLED;
}

export class InvalidInputDetails {
    public propertyPath: string;
    public reason: string;

    constructor(propertyPath: string, reason: string) {
        this.propertyPath = propertyPath;
        this.reason = reason;
    }
}

export class InvalidInputPropertiesError extends Error {
    public readonly __pulumiInvalidInputPropertiesError: boolean = true;

    constructor(
        message: string,
        public invalidProperties?: Array<InvalidInputDetails>,
    ) {
        super(message);
    }

    public static isInstance(obj: any): obj is InvalidInputPropertiesError {
        return utils.isInstance<InvalidInputPropertiesError>(obj, "__pulumiInvalidInputPropertiesError");
    }
}
