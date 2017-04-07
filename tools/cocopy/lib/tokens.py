# Copyright 2017 Pulumi, Inc. All rights reserved.

token_delim    = ":" # the character used to delimit parts of a token (module, member, etc).
mod_name_delim = "/" # the character used to delimit parts of a module name (i.e., sub-modules).

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

