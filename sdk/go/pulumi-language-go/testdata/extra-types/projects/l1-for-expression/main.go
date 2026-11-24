package main

import (
	"fmt"
	"sort"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		names := []string{
			"alpha",
			"beta",
			"gamma",
		}
		tags := map[string]string{
			"Environment": "production",
			"Team":        "infra",
		}
		var forResult0 []string
		for _, n := range names {
			forResult0 = append(forResult0, fmt.Sprintf("prefix-%v", n))
		}
		ctx.Export("prefixed", pulumi.ToStringArray(forResult0))
		var forResult1 []string
		for _, n := range names {
			if n != "beta" {
				forResult1 = append(forResult1, n)
			}
		}
		ctx.Export("filtered", pulumi.ToStringArray(forResult1))
		var forResult2 []string
		for i, n := range names {
			forResult2 = append(forResult2, fmt.Sprintf("%v:%v", i, n))
		}
		ctx.Export("indexed", pulumi.ToStringArray(forResult2))
		var forResult3 []string
		forRange3 := tags
		forKeys3 := make([]string, 0, len(forRange3))
		for forKey3 := range forRange3 {
			forKeys3 = append(forKeys3, forKey3)
		}
		sort.Strings(forKeys3)
		for _, k := range forKeys3 {
			v := forRange3[k]
			forResult3 = append(forResult3, fmt.Sprintf("%v=%v", k, v))
		}
		ctx.Export("tagList", pulumi.ToStringArray(forResult3))
		forResult4 := map[string]string{}
		for _, n := range names {
			forResult4[n] = fmt.Sprintf("prefix-%v", n)
		}
		ctx.Export("prefixedMap", pulumi.ToStringMap(forResult4))
		forResult5 := map[string]string{}
		forRange5 := tags
		forKeys5 := make([]string, 0, len(forRange5))
		for forKey5 := range forRange5 {
			forKeys5 = append(forKeys5, forKey5)
		}
		sort.Strings(forKeys5)
		for _, k := range forKeys5 {
			v := forRange5[k]
			if k != "Team" {
				forResult5[k] = v
			}
		}
		ctx.Export("filteredTags", pulumi.ToStringMap(forResult5))
		return nil
	})
}
