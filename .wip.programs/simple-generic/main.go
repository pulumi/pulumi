package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	s3beta "example.com/simple/s3point5"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {

		indexDocument := "index.html"

		var bucket s3beta.Bucket
		ctx.RegisterResource("aws:s3/bucket:Bucket", "my-bucket", pulumi.Map{
			"website": pulumi.Map{
				"indexDocument": pulumi.T(indexDocument),
			},
			"acl": pulumi.T("public-read"),
		}, &bucket)

		bucketNameLength /* Promise[int] */ := pulumi.Apply(bucket.Bucket, func(name string) int {
			//                                                |                        |       ^ checked
			//                                                \ Promise[string]        \ inferred
			return len(name)
		})

		ctx.Export("bucketNameLength2", bucketNameLength /* Promise[int] */)

		// Uses existing helper functions, this still returns a StringOutput for back-compatibility.
		content := pulumi.Sprintf(`<html>
<head>
  <title>Hello, Generics!</title><meta charset="UTF-8">
</head>
<body>
  <h1>Hello, Generics!</h1>
  <p>Made with <a href="https://pulumi.com">Pulumi[💜]</a></p>
  <p>This site knows its own address, it is: <code>http://%s</code></p>
</body>
</html>`, bucket.WebsiteEndpoint)

		s3beta.NewBucketObject(ctx, "my-beta-bucket", s3beta.
			NewBucketArgs(bucket.Bucket). // Required arguments
			With(                         // Optional arguments
				s3beta.OptionalBucketObjectArgs{
					Key:     pulumi.T(indexDocument),      // Construct a Promise[T] from a T
					Content: pulumi.Cast[string](content), // Cast a StringOutput to a Promise[string]
					Metadata: pulumi.T(map[string]pulumi.Promise[string]{
						// Fully dynamic maps using pulumi.T to use string keys and inputty values:
						"generics-are": pulumi.T("💜"),
					}),
					ContentType:  pulumi.T("text/html"),
					Acl:          pulumi.T("public-read"),
					StorageClass: pulumi.T("STANDARD"),
				},
			))

		ctx.Export("websiteUrl", bucket.WebsiteEndpoint)

		return nil
	})
}
