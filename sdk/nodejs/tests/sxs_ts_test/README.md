This test validates that changes we're making in @pulumi/pulumi will be side-by-side compatible with the 'latest' version of `@pulumi/pulumi` that has already shipped.

If a change is made that is not compatible, then the process should be:

1. Ensure that the change is absolutely what we want to make.
2. Disable running this test.
3. Commit the change and update the minor version of `@pulumi/pulumi` (i.e. from 0.17.x to 0.18.0).
4. Flow this change downstream, rev'ing the minor version of all downstream packages.
5. Re-enable the test.  Because there is now a new 'latest' `@pulumi/pulumi`, this test should pass.

Step '3' indicates that we've made a breaking change, and that if 0.18 is pulled in from any package, that it must be pulled in from all packages.

Step '4' is necessary so that people can pick a set of packages that all agree on using this new `@pulumi/pulumi` version.  While not necessary to rev the minor version of these packages, we still do so to make it clear that there is a significant change here, and that one should not move to it as readily as they would a patch update.
