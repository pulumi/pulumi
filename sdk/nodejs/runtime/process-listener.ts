// Copyright 2016-2026, Pulumi Corporation.
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

type ProcessEventListener = (...args: any[]) => void;

export function addProcessListener(event: string, listener: ProcessEventListener): void {
    process.setMaxListeners(process.getMaxListeners() + 1);
    process.on(event, listener);
}

export function removeProcessListener(event: string, listener: ProcessEventListener): void {
    const hadListener = (process as NodeJS.EventEmitter).listeners(event).includes(listener);
    process.off(event, listener);
    if (hadListener) {
        process.setMaxListeners(Math.max(0, process.getMaxListeners() - 1));
    }
}
