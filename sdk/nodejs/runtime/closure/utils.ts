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

import * as ts from "typescript";

const legalNameRegex = /^[a-zA-Z_][0-9a-zA-Z_]*$/;
export function isLegalMemberName(n: string) {
    return legalNameRegex.test(n);
}

export function isLegalFunctionName(n: string) {
    if (!isLegalMemberName(n)) {
        return false;
    }

    const scanner = ts.createScanner(
        ts.ScriptTarget.Latest, /*skipTrivia:*/false, ts.LanguageVariant.Standard, n);
    const tokenKind = scanner.scan();
    if (tokenKind !== ts.SyntaxKind.Identifier &&
        tokenKind !== ts.SyntaxKind.ConstructorKeyword) {
        return false;
    }

    return true;
}
