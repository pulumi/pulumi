# Copyright 2016-2017, Pulumi Corporation
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

delim      = ":" # the character used to delimit parts of a token (module, member, etc).
name_delim = "/" # the character used to delimit parts of a name (e.g., namespaces, sub-modules).

# special module tokens:
mod_default = ".default" # the default module in a package.

# special variable tokens:
var_this  = ".this"  # the current object (for class methods).
var_super = ".super" # the parent class object (for class methods).

# special function tokens:
func_entrypoint = ".main" # the special package entrypoint function.
func_init       = ".init" # the special module/class initializer function.
func_ctor       = ".ctor" # the special class instance constructor function.

# special type tokens:
type_object  = "object"
type_string  = "string"
type_number  = "number"
type_bool    = "bool"
type_dynamic = "dynamic"

# type token modifiers:
typemod_array_prefix   = "[]"   # the prefix for array type tokens.
typemod_map_prefix     = "map[" # the prefix for map type tokens.
typemod_map_sep        = "]"    # the separator between key/value elements of map type tokens.
typemod_func_prefix    = "("    # the prefix for function type tokens.
typemod_func_param_sep = ","    # the separator between parameters of function type tokens.
typemod_func_sep       = ")"    # the separator between parameters and return of function type tokens.

# accessibility modifiers:
acc_public    = "public"
acc_private   = "private"
acc_protected = "protected"
accs = set([ acc_public, acc_private, acc_protected ])

