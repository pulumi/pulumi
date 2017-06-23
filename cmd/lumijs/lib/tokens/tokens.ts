// Copyright 2016-2017, Pulumi Corporation
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

// Tokens.
export type Token = string;            // a valid symbol token.
export type PackageToken = Token;      // a symbol token that resolves to a package.
export type ModuleToken = Token;       // a symbol token that resolves to a module.
export type ModuleMemberToken = Token; // a symbol token that resolves to a module member.
export type ClassMemberToken = Token;  // a symbol token that resolves to a class member.
export type TypeToken = Token;         // a symbol token that resolves to a type.
export type VariableToken = Token;     // a symbol token that resolves to a variable.
export type FunctionToken = Token;     // a symbol token that resolves to a function.

export const tokenDelimiter = ":";     // the character delimiting modules/members/etc.

// Names.
export type Name = string;             // a valid identifier name.

