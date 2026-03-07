// Copyright 2026, Pulumi Corporation.
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

/** A single flag on a command or menu. */
export interface Flag {
    /** The canonical flag name (for example, "stack"). */
    name: string;

    /** True if this flag is required. */
    required?: boolean;

    /** A primitive logical type: "string", "boolean", "int", etc. */
    type: string;

    /** The user-facing description of the flag. */
    description?: string;

    /** True if the flag may appear multiple times (for example, string arrays). */
    repeatable?: boolean;
}

/** A positional argument to a command. */
export interface Argument {
    /** The human-readable name for the argument. */
    name: string;

    /** The argument type, defaulting to "string" when omitted. */
    type?: string;

    /**
     * Optional override for how the argument appears in the usage string.
     * Mirrors the `Usage` field in the Go struct.
     */
    usage?: string;
}

/** The full positional argument specification for a command. */
export interface Arguments {
    /** All positional arguments (in order). */
    arguments: Argument[];

    /** The number of required leading arguments. */
    requiredArguments?: number;

    /** True if the last argument is variadic. */
    variadic?: boolean;
}

/** Base shape shared by menus and commands. */
interface NodeBase {
    /**
     * The node type discriminator.
     * This is either "menu" or "command".
     */
    type: string;

    /**
     * Flags available at this level of the hierarchy, keyed by their canonical
     * flag name.
     */
    flags?: Record<string, Flag>;
}

/** A menu is a command that groups other commands. */
export interface Menu extends NodeBase {
    type: "menu";

    /** True if this menu can also be executed directly as a command. */
    executable?: boolean;

    /** Subcommands in this menu. */
    commands?: Record<string, Structure>;
}

/** A leaf command that can be executed. */
export interface Command extends NodeBase {
    type: "command";

    /** Positional arguments for this command (if any). */
    arguments?: Arguments;

    /** Free-form documentation about what the command does. */
    description?: string;
}

/** A node in the CLI tree. */
export type Structure = Menu | Command;

// ---------------------------------------------------------------------------
// Automation overrides (preset and omitted flags)
// ---------------------------------------------------------------------------

/** Preset value for a flag when invoking the CLI. */
export type PresetValue = boolean | string | number | string[];

/** Per-flag rule: omit from options and/or use a preset value when invoking. */
export interface FlagRule {
    /** If true, exclude this flag from the generated options type. */
    omit?: boolean;

    /** If set, always add this flag with this value when invoking. */
    preset?: PresetValue;
}

/** A scope applies to a command path; path is a list of subcommand names. */
export interface AutomationOverridesScope {
    /** Command path (e.g. [] for root, ["stack"] for pulumi stack). */
    path: string[];

    /** If true, apply to this command and all subcommands; if false, this command only. */
    propagate: boolean;

    /** Flag name (CLI spec) -> rule. */
    flags: Record<string, FlagRule>;
}

/** Auxiliary spec for preset flags and option exclusions. */
export interface AutomationOverrides {
    version: number;
    scopes: AutomationOverridesScope[];
}
