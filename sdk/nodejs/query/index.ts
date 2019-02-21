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

import * as rxjs from "rxjs";

export interface WatchContext<TArgs> {
    list: (args: TArgs) => rxjs.Observable<any>;
}

export interface ListTypeArgs {
    type: string;
}

export interface ListArgs extends ListTypeArgs {
    stack?: string;
}

export interface ListContext {
    // TODO: replace Observable with synchronous query model.

    list: (args: ListArgs) => rxjs.Observable<any>;
}
