# Assets

Lumi is intentionally limited in two dimensions that can interfere with the expression of certain desired operations.
First, there is no ad-hoc I/O available, in order to encourage determinism.  Second, the type system is limited to a
"JSON-like" subset, in order to facilitate cross-language interoperability.  As a result, if you wanted to author a
resource that consumed a file -- something we call an "asset" in this document -- your options are limited.  Moreover,
even if a resource provider did come up with its own scheme for supporting such things, there would be no consistency.

Enter assets.  The Lumi framework providers a common base class, `lumi.Asset`, with derived implementations for
many kinds of file sources, including in-memory blobs and strings, files, and URI-addressed files.  There is a standard
way for resource providers to interact with assets, ensuring consistency across providers.

