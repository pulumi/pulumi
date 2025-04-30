* Each deployment assumes
    - A single target
    - A single source
    - A single previous snapshot, on which a single dependency graph is built,
      etc.
    - Probably other things

* Started factoring out targets so that targets/"stack references" are tied to
  each event that comes into resmon/stepgen
* In theory then we can plumb a deployment to have one or more sources?
* When we've got that we can see what goes wrong next

# Done

* Hack to have outputs written to state so that `StackReferences` can provide
  dependency tracking across stacks
* Hack to push `StackReference` (a different type, confusing -- basically an
  organisation/project/stack triple) into `Goal`s, `ReadResourceEvent`s etc. so
  that URNs can be generated and so on without having a single target for a
  deployment.
* Copy-paste of the `pulumi up` command, `pulumi up-all` that will be modified
  to take enough args to set up the multi-deployment object (not yet done).
* Basic integration test (`TestNodeMultistack`) that is being extended to try
  and get something end-to-end (will call `up-all`)

```
(SDKS= make build install; cd tests/integration; go test ./... -tags=all -v -count=1 -run 'TestNodeMultistack')
```
