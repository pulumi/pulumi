# Writing documentation

## Contributing

The sources of all the pages on this website are maintained in the
[docs](https://github.com/pulumi/pulumi/tree/master/docs) folder of the
[pulumi](https://github.com/pulumi/pulumi) GitHub repository. Pulumi welcomes contributions to the developer
documentation via Pull Requests.

## Conventions

* Write documentation in Markdown (`.md` files) where possible. While Sphinx
  supports reStructuredText (rST, `.rst` files), Markdown is generally more
  ubiquitous, and easier to read and write for contributors. Most advanced
  reStructuredText features are made available by the
  [MyST parser](https://myst-parser.readthedocs.io) we use, so you should be
  able to do everything you need to do in Markdown. If you can't, consider
  opening an issue so that we can make it possible!

* The entry point for documentation on a component, service, library, etc.
  should be named `README.md`.

* Use headers correctly. All documents *must* have a top-level ("h1") header --
  this is `#` in Markdown. There *must* not be more than one top-level header in
  a given file.

* If you are documenting a single thing (a component, service, library etc.) you
  should in almost all cases use the name of that thing (or an appropriate
  human-readable counterpart) as the top-level header (e.g.
  `# Code generation` in `pkg/codegen/README.md`).

* If you are documenting a group of things (e.g. a package or bounded context
  consisting of multiple libraries, services, maybe some words on how
  development works, etc.) you should aggregate the documentation for those
  things using an appropriate table of contents. Use the `toctree` directive
  (available using triple-colon fences in MyST) to achieve this. You can make
  use of globs to make this more "free" if you want:

  ```markdown
  :::{toctree}
  :glob:
  :maxdepth: 1
  :titlesonly:

  /path/to/package/**/README
  :::
  ```

## MyST-compatible Markdown style guide/cheatsheet

The following is a quick reference for writing MyST-compatible Markdown. For
more comprehensive documentation, see the [MyST
documentation](https://myst-parser.readthedocs.io).

:::{note}
The `# H1: Document title` in the example snippet below is actually written as
an H2 (`##`) in the source document so as not to violate the principle of only
having one top-level header per document (and in turn not break any tables of
contents).
:::

````markdown
# H1: Document title

H1s are denoted with `#`; H2s with `##`; H3s with `###`; and so on. Leave a
blank line before any heading. A document should have exactly one H1.

Lines should be no longer than 120 characters. Except where noted, favour
`lower-kebab-case` for identifiers/reference names etc.

## H2: Getting into it

Many "reStructuredText-only" features are made available in MyST Markdown using
triple-colon fences. For example, admonitions (hints, notes, etc.) look as
follows:

:::{note}
An example one-line note.
:::

Code blocks work as they do in normal Markdown, using backticks for inline code
and triple backticks for blocks. Specifying a supported language will enable
syntax highlighting:

```bash
# Execute a bash script.
$ /some/interesting/bash/script.sh
```

Use *single asterisks for emphasis* (italics), **double asterisks for heavy
emphasis** (boldface) and `backticks for monospace`. Links can be
take a number of forms -- [here's an external HTTP example](https://example.com)
and [here's a link to a part of the document](#sec-some-part-of-the-document).
External links will open in a new tab by default.[^side-note-1]

[^side-note-1]: Check out this neat side note too!

(sec-some-part-of-the-document)=

### H3: Cross-referencing

You can define a label for a block as above using the `(label)=` syntax and
refer to it using an `#id` as shown above. As above, follow a convention of
prefixing sections with `sec-` and adhere to kebab-case.

### H3: Custom link types

MyST supports custom URL schemes and we (ab)use these for convenient linking to
GitHub issues and files:

* <gh-issue:pulumi#1234> will link to issue 1234 in the `pulumi` repository.
* <gh-file:pulumi#README.md> will link to the `README.md` file in the `pulumi`
  repository.
````

## H1: Document title

H1s are denoted with `#`; H2s with `##`; H3s with `###`; and so on. Leave a
blank line before any heading. A document should have exactly one H1.

Lines should be no longer than 120 characters. Except where noted, favour
`lower-kebab-case` for identifiers/reference names etc.

## H2: Getting into it

Many "reStructuredText-only" features are made available in MyST Markdown using
triple-colon fences. For example, admonitions (hints, notes, etc.) look as
follows:

:::{note}
An example one-line note.
:::

Code blocks work as they do in normal Markdown, using backticks for inline code
and triple backticks for blocks. Specifying a supported language will enable
syntax highlighting:

```bash
# Execute a bash script.
$ /some/interesting/bash/script.sh
```

Use *single asterisks for emphasis* (italics), **double asterisks for heavy
emphasis** (boldface) and `backticks for monospace`. Links can
take a number of forms -- [here's an external HTTP example](https://example.com)
and [here's a link to a part of the document](#sec-some-part-of-the-document).
External links will open in a new tab by default.[^side-note-1]

[^side-note-1]: Check out this neat side note too!

(sec-some-part-of-the-document)=

### H3: Cross-referencing

You can define a label for a block as above using the `(label)=` syntax and
refer to it using an `#id` as shown above. As above, follow a convention of
prefixing sections with `sec-` and adhere to kebab-case.

### H3: Custom link types

MyST supports custom URL schemes and we (ab)use these for convenient linking to
GitHub issues and files:

* <gh-issue:pulumi#1234> will link to issue 1234 in the `pulumi` repository.
* <gh-file:pulumi#README.md> will link to the `README.md` file in the `pulumi`
  repository.
