module example.com/pulumi-glog-regression

go 1.25

require (
	github.com/golang/glog v1.2.5
	github.com/pulumi/pulumi/sdk/v3 v3.0.0
)

replace github.com/pulumi/pulumi/sdk/v3 => ../../../../..
