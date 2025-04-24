package main

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"mime"
	"os"
	"path"
	"strings"

	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func filebase64OrPanic(path string) string {
	if fileData, err := os.ReadFile(path); err == nil {
		return base64.StdEncoding.EncodeToString(fileData[:])
	} else {
		panic(err.Error())
	}
}

func filebase64sha256OrPanic(path string) string {
	if fileData, err := os.ReadFile(path); err == nil {
		hashedData := sha256.Sum256([]byte(fileData))
		return base64.StdEncoding.EncodeToString(hashedData[:])
	} else {
		panic(err.Error())
	}
}

func sha1Hash(input string) string {
	hash := sha1.Sum([]byte(input))
	return hex.EncodeToString(hash[:])
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		encoded := base64.StdEncoding.EncodeToString([]byte("haha business"))
		tmpVar0, _ := base64.StdEncoding.DecodeString(encoded)
		decoded := string(tmpVar0)
		_ = strings.Join([]string{
			encoded,
			decoded,
			"2",
		}, "-")
		// tests that we initialize "var, err" with ":=" first, then "=" subsequently (Go specific)
		_, err := aws.GetAvailabilityZones(ctx, &aws.GetAvailabilityZonesArgs{}, nil)
		if err != nil {
			return err
		}
		_, err = aws.GetAvailabilityZones(ctx, &aws.GetAvailabilityZonesArgs{}, nil)
		if err != nil {
			return err
		}
		bucket, err := s3.NewBucket(ctx, "bucket", nil)
		if err != nil {
			return err
		}
		_ = bucket.ID().ApplyT(func(id string) (pulumi.String, error) {
			return pulumi.String(base64.StdEncoding.EncodeToString([]byte(id))), nil
		}).(pulumi.StringOutput)
		_ = bucket.ID().ApplyT(func(id string) (pulumi.String, error) {
			value, _ := base64.StdEncoding.DecodeString(id)
			return pulumi.String(value), nil
		}).(pulumi.StringOutput)
		secretValue := pulumi.ToSecret("hello").(pulumi.StringOutput)
		_ = pulumi.Unsecret(secretValue).(pulumi.StringOutput)
		currentStack := ctx.Stack()
		currentProject := ctx.Project()
		workingDirectory := func(cwd string, err error) string {
			if err != nil {
				panic(err)
			}
			return cwd
		}(os.Getwd())
		fileMimeType := mime.TypeByExtension(path.Ext("./base64.txt"))
		// using the filebase64 function
		_, err = s3.NewBucketObject(ctx, "first", &s3.BucketObjectArgs{
			Bucket:      bucket.ID(),
			Source:      pulumi.NewStringAsset(filebase64OrPanic("./base64.txt")),
			ContentType: pulumi.String(fileMimeType),
			Tags: pulumi.StringMap{
				"stack":   pulumi.String(currentStack),
				"project": pulumi.String(currentProject),
				"cwd":     pulumi.String(workingDirectory),
			},
		})
		if err != nil {
			return err
		}
		// using the filebase64sha256 function
		_, err = s3.NewBucketObject(ctx, "second", &s3.BucketObjectArgs{
			Bucket: bucket.ID(),
			Source: pulumi.NewStringAsset(filebase64sha256OrPanic("./base64.txt")),
		})
		if err != nil {
			return err
		}
		// using the sha1 function
		_, err = s3.NewBucketObject(ctx, "third", &s3.BucketObjectArgs{
			Bucket: bucket.ID(),
			Source: pulumi.NewStringAsset(sha1Hash("content")),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
