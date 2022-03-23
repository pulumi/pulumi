import * as pulumi from "@pulumi/pulumi";

// If this is run onder "module": "esnext", it will fail. Neither the import nor the export are
// valid for "esnext".

// Export the name of the bucket
export const bucketName = "name";
