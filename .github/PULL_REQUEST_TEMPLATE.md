<!--- 
Thanks so much for your contribution! If this is your first time contributing, please ensure that you have read the [CONTRIBUTING](https://github.com/pulumi/pulumi/blob/master/CONTRIBUTING.md) documentation.
-->

# Description

<!--- Please include a summary of the change and which issue is fixed. Please also include relevant motivation and context. -->

Fixes # (issue)

## Checklist

<!--- Please provide details if the checkbox below is to be left unchecked. -->
- [ ] I have added tests that prove my fix is effective or that my feature works
<!--- 
User-facing changes require a CHANGELOG entry.
-->
- [ ] I have updated the [CHANGELOG-PENDING](https://github.com/pulumi/pulumi/blob/master/CHANGELOG_PENDING.md) file with my change
<!--
If the change(s) in this PR is a modification of an existing call to the Pulumi Service,
then the service should honor older versions of the CLI where this change would not exist.
You must then bump the API version in /pkg/backend/httpstate/client/api.go, as well as add
it to the service.
-->
- [ ] Yes, there are changes in this PR that warrants bumping the Pulumi Service API version
  <!-- @Pulumi employees: If yes, you must submit corresponding changes in the service repo. -->
