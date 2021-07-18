package yaml

// Package yaml provides an alternative method for authoring Pulumi schemas. Rather than hewing closely to the design of
// OpenAPI, this package chooses a YAML-based approach that allows for a much more concise description of a Pulumi
// schema. Although this package is intended to cover the majority of use cases, it is not intended to have feature
// parity with the JSON-based schema language, and omits some advanced features for the sake of simplicity.
//
// The two most striking differences are the representations of package members and property types.
//
// Rather than using separate, flat namespaces for types, resources, and functions, a YAML schema uses a single,
// hierarchical namespace. The type of a member is indicated using a YAML tag; members may be object types, enum types,
// resources, components, functions, or modules.
//
// YAML schemas also make use of tags and the module tree to simplify type references. A type reference may be a
// primitive type, constructed type, or a reference to an object type, enum type, or resource. Primitive types are
// booleans, strings, integers, and numbers; constructued types are inputs, arrays, maps, and unions. At the
// outermost level, a property's type may also be optional, in which case the property does not require a value.
//
// An example schema that showcases the differences and its JSON equivalent are given below.
//
// YAML:
//
//     version: "0.0.1"
//     name: example
//     imports:
//       aws: /aws/v3.14.0/schema.json
//     members:
//       # A simple object type.
//       MyObject: !Object
//         foo: !Input [MyResource]
//         bar: !Optional [!Input [string]]
//         baz: !Optional [!Input [!Array [!Input [!Array [!Input [string]]]]]] # List of lists of strings
//         qux: !Optional [!Input [!Map [!Input [!Array [!Input [number]]]]]] # Mapping from string to list of numbers
//       MyResource: !Resource
//         inouts:
//           foo: !Optional [!Input [MyObject]]
//           bar: !Optional [!Input [string]]
//           bar: !Optional [!Input [string]]
//           baz: !Input [string]
//           qux: !Array [!Input [number]]
//       MyComponent: !Component
//         inouts:
//           resource: !Optional [!Input [MyResource]]
//           bucket: /aws/resources/s3/bucket/Bucket
//       myFunction: !Function
//         parameters:
//           arg1: !Optional [MyResource]
//         returns:
//           result: !Optional [MyResource]
//
// JSON:
//
//     {
//       "name": "example",
//       "version": "0.0.1",
//       "config": {},
//       "types": {
//         "example::MyObject": {
//           "description": "A simple object type.\n",
//           "properties": {
//             "bar": {
//               "type": "string"
//             },
//             "baz": {
//               "type": "array",
//               "items": {
//                 "type": "array",
//                 "items": {
//                   "type": "string"
//                 }
//               },
//               "description": "List of lists of strings\n"
//             },
//             "foo": {
//               "$ref": "#/resources/example::MyResource"
//             },
//             "qux": {
//               "type": "object",
//               "additionalProperties": {
//                 "type": "array",
//                 "items": {
//                   "type": "number"
//                 }
//               },
//               "description": "Mapping from string to list of numbers\n"
//             }
//           },
//           "type": "object",
//           "required": [
//             "foo"
//           ]
//         }
//       },
//       "provider": {},
//       "resources": {
//         "example::MyComponent": {
//           "properties": {
//             "bucket": {
//               "$ref": "/aws/v3.14.0/schema.json#/resources/aws:s3/bucket:Bucket"
//             },
//             "resource": {
//               "$ref": "#/resources/example::MyResource"
//             }
//           },
//           "required": [
//             "bucket"
//           ],
//           "inputProperties": {
//             "bucket": {
//               "$ref": "/aws/v3.14.0/schema.json#/resources/aws:s3/bucket:Bucket"
//             },
//             "resource": {
//               "$ref": "#/resources/example::MyResource"
//             }
//           },
//           "requiredInputs": [
//             "bucket"
//           ],
//           "isComponent": true
//         },
//         "example::MyResource": {
//           "properties": {
//             "bar": {
//               "type": "string"
//             },
//             "baz": {
//               "type": "string"
//             },
//             "foo": {
//               "$ref": "#/types/example::MyObject"
//             },
//             "qux": {
//               "type": "array",
//               "items": {
//                 "type": "number"
//               }
//             }
//           },
//           "required": [
//             "baz",
//             "qux"
//           ],
//           "inputProperties": {
//             "bar": {
//               "type": "string"
//             },
//             "baz": {
//               "type": "string"
//             },
//             "foo": {
//               "$ref": "#/types/example::MyObject"
//             },
//             "qux": {
//               "type": "array",
//               "items": {
//                 "type": "number"
//               },
//               "plain": true
//             }
//           },
//           "requiredInputs": [
//             "baz",
//             "qux"
//           ]
//         }
//       },
//       "functions": {
//         "example::myFunction": {
//           "inputs": {
//             "properties": {
//               "arg1": {
//                 "$ref": "#/resources/example::MyResource",
//                 "plain": true
//               }
//             },
//             "type": "object"
//           },
//           "outputs": {
//             "properties": {
//               "result": {
//                 "$ref": "#/resources/example::MyResource"
//               }
//             },
//             "type": "object"
//           }
//         }
//       }
//     }
