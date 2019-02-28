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

import { Resource } from "./resource";
import * as utils from "./utils";

/**
 * RunError can be used for terminating a program abruptly, but resulting in a clean exit rather
 * than the usual verbose unhandled error logic which emits the source program text and complete
 * stack trace.  This type should be rarely used.  Ideally ResourceError should always be used so
 * that as many errors as possible can be associated with a Resource.
 */
export class RunError extends Error {
    /**
     * A private field to help with RTTI that works in SxS scenarios.
     */
    // tslint:disable-next-line:variable-name
    /* @internal */ public readonly __pulumiRunError: boolean = true;

    /**
     * Returns true if the given object is an instance of a RunError.  This is designed to work even when
     * multiple copies of the Pulumi SDK have been loaded into the same process.
     */
    public static isInstance(obj: any): obj is RunError {
        return utils.isInstance<RunError>(obj, "__pulumiRunError");
    }

    constructor(message: string) {
        super(message);
    }
}

/**
 * ResourceError can be used for terminating a program abruptly, specifically associating the
 * problem with a Resource.  Depending on the nature of the problem, clients can choose whether or
 * not a call stack should be returned as well.  This should be very rare, and would only indicate
 * no usefulness of presenting that stack to the user.
 */
export class ResourceError extends Error {
    /**
     * A private field to help with RTTI that works in SxS scenarios.
     */
    // tslint:disable-next-line:variable-name
    /* @internal */ public readonly __pulumResourceError: boolean = true;

    /**
     * Returns true if the given object is an instance of a ResourceError.  This is designed to work even when
     * multiple copies of the Pulumi SDK have been loaded into the same process.
     */
    public static isInstance(obj: any): obj is ResourceError {
        return utils.isInstance<ResourceError>(obj, "__pulumResourceError");
    }

    constructor(message: string, public resource: Resource | undefined, public hideStack?: boolean) {
        super(message);
    }
}
