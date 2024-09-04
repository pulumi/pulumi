(docsgen)=
# Documentation

This package supports generating documentation for Pulumi packages from their
[schema](schema).

## Crash course on templates

The templates use Go's built-in `html/template` package to process templates with data. The driver for this doc generator (e.g. tfbridge for TF-based providers) then persists each file from memory onto the disk as `.md` files.

Although we are using the `html/template` package, it has the same exact interface as the [`text/template`](https://golang.org/pkg/text/template) package, except for some HTML specific things. Therefore, all of the functions available in the `text/template` package are also available with the `html/template` package.

* Data can be injected using `{{.PropertyName}}`.
* Nested properties can be accessed using the dot notation, i.e. `{{.Property1.Property2}}`.
* Templates can inject other templates using the `{{template "template_name"}}` directive.
  * For this to work, you will need to first define the named template using `{{define "template_name"}}`.
* You can pass data to nested templates by simply passing an argument after the template's name.
* To remove whitespace from injected values, use the `-` in the template tags.
  * For example, `{{if .SomeBool}} some text {{- else}} some other text {{- end}}`. Note the use of `-` to eliminate whitespace from the enclosing text.
  * Read more [here](https://golang.org/pkg/text/template/#hdr-Text_and_spaces).
* To render un-encoded content use the custom global function `htmlSafe`.
  * **Note**: This should only be used if you know for sure you are not injecting any user-generated content, as it by-passes the HTML encoding.
* To render strings to Markdown, use the custom global function `markdownify`.
* To print regular strings, that share the same syntax as the Go templating engine, use the built-in global function `print` [function](https://golang.org/pkg/text/template/#hdr-Functions).

Learn more from here: https://curtisvermeeren.github.io/2017/09/14/Golang-Templates-Cheatsheet

## Modifying templates and updating tests

We run tests that validate our template-rendering output. If you need to make change that produces a set of Markdown files that differs from the set that we use in our tests (see `codegen/testing/test/testdata/**/*.md`), your pull-request checks will fail, and to get them to pass, you'll need to modify the test data to match the output produced by your change.

For minor diffs, you can just update the test files manually and include those updates with your PR. But for large diffs, you may want to regenerate the full set. To do that, from the root of the repo, run:

```
cd pkg/codegen/docs && PULUMI_ACCEPT=true go test . && cd -
```
