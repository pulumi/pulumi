package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	s3beta "example.com/simple/s3point5"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {

		indexDocument := "index.html"

		bucket, err := s3beta.NewBucket(ctx, "my-bucket",
			s3beta.NewBucketArgs().With(s3beta.OptionalBucketArgs{
				Website: s3beta.BucketWebsite{
					IndexDocument: pulumi.T(indexDocument),
				},
				Acl: pulumi.T("public-read"),
			}))
		if err != nil {
			return err
		}

		// N.B.: unmarshal outputs hasn't been taught how to handle promises yet, it still expects the
		// previous codegen's Input-shaped-structs. This prevents us from using
		// `bucket.Website.IndexDocument`.
		//
		// End state should look like "bucket.Website" being a plain struct, but each of its fields is a
		// promise.
		//
		// This will require updating:
		//  * context.go here: sdk/go/pulumi/context.go:1087 to deeply recurse into output shapes and
		//    create the promises at the leaves
		//  * and some tweak to unmarshalOutputs most likely here: sdk/go/pulumi/rpc.go:665
		//
		// With this end state, lifting is done by the RPC layer. Only lists, maps, and non-object
		// values are promises. Only *dynamic* structure and values are a promise (maps, arrays). Static
		// structure is not.
		//
		// On that note, related to the pointer comment below - in Go fashion, we can just populate all
		// empty values with their defaults. If applied over, users can check they're a default, like
		// so:

		ctx.Export("is-policy-set", pulumi.Apply(bucket.Policy, func(policy string) string {
			if policy == "" {
				return "nope"
			} else {
				return "yep"
			}
		}))

		// This allows us to share the same types for inputs & outputs, along with the change noted
		// below the optional values.

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
			NewBucketObjectArgs(bucket.Bucket). // Required arguments
			With(                               // Optional arguments
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
					// Values here don't need to be pointers - we can peek at the value and infer that an
					// optional wasn't set because the output is a default value, not one that was
					// constructed. We also tweak marshalInputs to not emit empty maps (recursively) for a nice
					// diff, free of "output<string>" noise.
				},
			))

		ctx.Export("websiteUrl", bucket.WebsiteEndpoint)

		return nil
	})
}
