package nodejs

import nodejs "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/nodejs"

// NodePackageInfo contains NodeJS-specific information for a package.
type NodePackageInfo = nodejs.NodePackageInfo

// NodeObjectInfo contains NodeJS-specific information for an object.
type NodeObjectInfo = nodejs.NodeObjectInfo

// Importer implements schema.Language for NodeJS.
var Importer = nodejs.Importer

