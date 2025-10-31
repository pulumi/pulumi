package pcl

import pcl "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/pcl"

// Call is the name of the PCL `call` intrinsic, which can be used to invoke methods on resources.
// 
// `call` has the following signature:
// 
// 	call(self, method, args)
// 
// where `self` is the resource to invoke the method on, `method` is the name of the method to invoke, and `args` is an
// object containing the arguments to pass to the method.
const Call = pcl.Call

