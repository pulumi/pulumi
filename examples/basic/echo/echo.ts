// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import * as lumix from "lumix";

// Echo is a simple service that wraps an API gateway and exposes a single Say function.
class Echo extends lumix.Service {
    constructor(
        // port is an optional port number for the API service.
        port: number = 80
    ) {
        new lumix.APIGateway({
            api: this
            port: port
        });
    }

    // Say simply echoes back the given string.
    @lumix.api()
    public Say(s: string): string {
        return s;
    }
}

