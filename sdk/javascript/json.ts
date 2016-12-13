// Copyright 2016 Marapongo, Inc. All rights reserved.

// JSON represents the set of JSON-like objects (that is, they can be marshaled to/from JSON).
export interface JSON {
    [key: string]: JSONValue;
}

// JSONValue is any legal JSON value: a primitive type, an object (or map of strings to values), or an array.
export type JSONValue = string | number | boolean | JSON | JSONArray | null;

// JSONArray is an array of JSON-like values.
export interface JSONArray extends Array<JSONValue> { }

