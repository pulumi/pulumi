# coconut/lib

This directory contains the various libraries (Nuts) that Coconut programs may depend upon.

The Coconut standard library underneath `coconut/` is special in that every program will ultimately use it directly or
indirectly to create resources.

Note that these are written in the Coconut subsets of the languages and therefore cannot perform I/O, etc.

Eventually these packages will be published like any other NPM package.  For now, they are consumed only in a
development capacity, and so there are some manual steps required to prepare a development workspace.

For each library `<lib>` you wish to use, in dependency order:

* `cd <lib>`
* `yarn install`
* For each dependency `<dep>`:
    - `yarn link dep`
* `yarn build`
* `yarn link`

And then from within each Nut's directory, `<lib>`, that will consume said libraries:

* `yarn link <lib>`

For example, let's say we want to use the standard library and the AWS library from another Nut, `/dev/mypackage`:

* First, `cd $GOPATH/src/github.com/pulumi/coconut/lib/coconut`:
    * `yarn install`
    * `yarn build`
    * `yarn link`
* Next, `cd $GOPATH/src/github.com/pulumi/coconut/lib`:
    * `yarn install`
    * `yarn link @coconut/coconut`
    * `yarn build`
    * `yarn link`
* Finally, `cd /dev/mypackage`:
    * `yarn link @coconut/coconut`
    * `yarn link @coconut/aws`
* Now we are ready to go working on `mypackage`, and the two Nut references will be resolved correctly.

