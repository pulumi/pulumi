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

import { z } from "mockpackage";

export const description = "Respects package.json exports";

type LambdaInput = {
    message: string,
}

// @ts-ignore
const getSchemaValidator = (): z.ZodSchema<LambdaInput> => z.object({
    message: z.string(),
});

async function reproHandler(input: any) {
    const payload = getSchemaValidator().parse(input);
    console.log(payload.message);
    return {
    }
}

export const func = reproHandler;
