from sphinx.ext.autodoc import between
import os
import sys

# Configuration file for the Sphinx documentation builder.
#
# This file only contains a selection of the most common options. For a full
# list see the documentation:
# https://www.sphinx-doc.org/en/master/usage/configuration.html


# -- Path setup --------------------------------------------------------------

# If extensions (or modules to document with autodoc) are in another directory,
# add these directories to sys.path here. If the directory is relative to the
# documentation root, use os.path.abspath to make it absolute, like shown here.
sys.path.append(os.path.abspath("./_ext"))


# -- Project information -----------------------------------------------------

project = "Pulumi"
copyright = "Pulumi 2024"
author = "Pulumi engineering"


# -- General configuration ---------------------------------------------------

# The master document, where the root table of contents ("toctree") lives. The
# extension is not needed here, so e.g. "README" corresponds to README.rst in
# the top-level directory. Our top-level README is designed to introduce Pulumi
# itself as a product, so we use the README in the "docs" directory as the
# master document (which is a Markdown file).
master_doc = "docs/README"

# Add any Sphinx extension module names here, as strings. They can be extensions
# coming with Sphinx (named "sphinx.ext.*") or custom ones (which will likely
# come from the path extended above i.e. _ext).
extensions = [
    # Community extensions
    "myst_parser",
    "sphinx.ext.autodoc",
    "sphinx.ext.autosectionlabel",
    "sphinx.ext.intersphinx",
    "sphinx_tabs.tabs",
    "sphinxcontrib.mermaid",
    "sphinxcontrib.programoutput",
]

# Extensions for MyST, which we use to support Markdown as an alternative to
# reStructuredText (Sphinx's default format).
myst_enable_extensions = [
    "attrs_inline",
    "colon_fence",
    "deflist",
    "fieldlist",
]

myst_links_external_new_tab = True

# Languages listed here will be interpreted as directives when encountered after
# a code fence. For instance, if MyST sees the string "```mermaid", and mermaid
# is configured here, it will interpret it as if it had been written as a
# directive, e.g. "```{mermaid}".
#
# This is useful for interoperability with tools that have special support for
# certain languages. GitHub, for instance, will render Mermaid diagrams if it
# encounters them, so it's beneficial for us to be able to write them as
# "normal" code blocks rather than as directives.
myst_fence_as_directive = ["mermaid"]

# Configure some custom MyST URL schemes for easy linking to GitHub files and
# issues.
myst_url_schemes = {
    "http": None,
    "https": None,
    # Usage: <gh-file:repository-under-pulumi-org#path/to/file>
    "gh-file": {
        "url": "https://github.com/pulumi/{{path}}/blob/master/{{fragment}}",
        "title": "pulumi/{{path}}:{{fragment}}",
        "classes": ["github"],
    },
    # Usage: <gh-file:repository-under-pulumi-org#issue-or-pr-number>
    "gh-issue": {
        "url": "https://github.com/pulumi/{{path}}/issues/{{fragment}}",
        "title": "pulumi/{{path}} #{{fragment}}",
        "classes": ["github"],
    },
}

# Intersphinx enables linking to references and the like in other Sphinx
# documentation sites. We configure it here so that we can link to the various
# Pulumi projects that are all hosted under the root Pulumi site.
intersphinx_mapping = {
    # Terrafom Bridge developer documentation
    # https://github.com/pulumi/pulumi-terraform-bridge
    "tfbridge": ("https://pulumi-developer-docs.readthedocs.io/projects/pulumi-terraform-bridge/en/latest/", None),
}

# Sphinx defaults to trying to automatically resolve *unresolved* labels using
# Intersphinx mappings. This can have unintended side effects, such as local
# references suddenly resolving to external locations. As a result we disable
# this behaviour here. See
# https://www.sphinx-doc.org/en/master/usage/extensions/intersphinx.html#confval-intersphinx_disabled_reftypes
# for more information.
intersphinx_disabled_reftypes = ["*"]

# The types of source files and suffixes that Sphinx should recognise and parse.
source_suffix = {
    ".md": "markdown",
    ".rst": "restructuredtext",
}

# Configuration for the "autosectionlabel" extension, which generates references
# to section headers automatically (and is thus super user for Markdown files
# where explicitly writing such references can sometimes be tedious).
autosectionlabel_prefix_document = True

# Add any paths that contain templates here, relative to this directory.
templates_path = ["_templates"]

# A list of of patterns, relative to the source directory, that match files and
# directories to ignore when looking for source files. These patterns also
# affect html_static_path and html_extra_path.
exclude_patterns = [
    # General noise
    "**/*.rst.inc",
    "**/node_modules",
    "**/site-packages",
    ".direnv",
    ".git",
    "node_modules",
    # Pulumi-specific cases
    ## Standalone documentation files/noise from vendored libraries (CHANGELOGs,
    ## LICENSEs, etc.)
    "CHANGELOG.md",
    "CODE_OF_CONDUCT.md",
    "CONTRIBUTING.md",
    "README.md",
    ## Test data
    "**/testdata",
]


# -- Options for HTML output -------------------------------------------------

# The theme to use for HTML and HTML Help pages. We are using the "book" theme
# from the sphinx_book_theme package. See
# https://sphinx-book-theme.readthedocs.io/ for more information on the theme
# and its options.
#
html_theme = "sphinx_book_theme"

html_title = "Pulumi developer documentation"
html_logo = "https://www.pulumi.com/images/logo/logo-on-white-box.svg"

html_theme_options = {
    # Expand all headings up to level 2 by default in the left sidebar (global)
    # table of contents.
    "show_navbar_depth": 2,
    # Show all headings up to level 2 in the right sidebar (in-page) table of
    # contents.
    "show_toc_level": 2,
    # Expand all headings up to level 2 by default in the sidebar table of
    # contents.
    # Enable margin notes
    # (https://sphinx-book-theme.readthedocs.io/en/stable/content/content-blocks.html#sidenotes-and-marginnotes).
    "use_sidenotes": True,
    # Enable linking to GitHub page sources and editing files directly in the
    # browser.
    "repository_provider": "github",
    "repository_url": "https://github.com/pulumi/pulumi",
    "repository_branch": "master",
    "use_source_button": True,
    "use_edit_page_button": True,
    "use_issues_button": True,
}

# Add any paths that contain custom static files (such as style sheets) here,
# relative to this directory. They are copied after the builtin static files, so
# a file named "default.css" will overwrite the builtin "default.css". Note
# that, annoyingly, making changes to these files requires a full rebuild of the
# documentation (that is, the default incremental build won't pick up changes to
# these files).
html_static_path = ["_static"]

html_css_files = ["custom.css"]

# -- Application-specific setup ----------------------------------------------


def setup(app):
    # Connect a listener which ignores any text between ".. AUTODOC-IGNORE"
    # markers when autodoc is processing docstrings. This means that, for
    # example, custom roles and directives can include documentation that talks
    # both about the role API as well as how it should be consumed.
    app.connect(
        "autodoc-process-docstring", between("^.*\.\. AUTODOC-IGNORE.*$", exclude=True)
    )
    return app
