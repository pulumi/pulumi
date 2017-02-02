# mu/lib

This directory contains the various MuPackage libraries that Mu programs may depend upon.  The Mu standard library
(under `mu/`) is special in that every Mu program will ultimately use it directly or indirectly to create resources.

Eventually these packages will be published like any other NPM MuPackage.  For now, they are consumed only in a
development capacity, and so there are some manual steps required to prepare a development workspace.

For each library `<lib>` you wish to use, in dependency order:

* `cd <lib>`
* `yarn install`
* For each dependency `<dep>`:
    - `yarn link dep`
* `yarn build`
* `yarn link`

And then from within each MuPackage's directory that will consume said libraries, for each such library `<lib>`:

* `yarn link <lib>`

For example, let's say we want to use the Mu standard library and the AWS library from a MuPackage `/dev/mypackage`:

* First, `cd $GOPATH/src/github.com/marapongo/mu/lib/mu`:
    * `yarn install`
    * `yarn build`
    * `yarn link`
* Next, `cd $GOPATH/src/github.com/marapongo/mu/lib`:
    * `yarn install`
    * `yarn link mu`
    * `yarn build`
    * `yarn link`
* Finally, `cd /dev/mypackage`:
    * `yarn link mu`
    * `yarn link mu/@aws`
* Now we are ready to go working on `mypackage`; references to `mu` and `@mu/aws` will be resolved correctly.

