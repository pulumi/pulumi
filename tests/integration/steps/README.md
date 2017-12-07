This test attempts to exhaustively test all interesting combinations of resource steps.  This includes:

* Same
* Create
* Update
* Delete
* CreateReplacement
* DeleteReplaced

In addition to the ability to recover from failures.  For example, there is a "pending deletion" capability
that will remember resources that were meant to be deleted, but couldn't be, due to a failure partway through.

The test is broken into a series of steps that will be executed in order.  Because the steps create different
resources, we will end up with a specific sequence of CRUD operations that we will validate.

# Step 1

Populate the world:

* Create 4 resources, a1, b1, c1, d1.  c1 depends on a1 via an ID property.

Checkpoint: a1, b1, c1, d1

# Step 2

Same, Update, Same, Delete, Create:

* Create 1 resource, a2, equivalent to the a1 in Step 1 (Same(a1, a2)).

* Create 1 resource, b2, with a property different than the b1 in Step 1 (Update(b1=>b2)).

* Create 1 resource, c2, equivalent to the c1 in Step 1 (Same(c1, c2)).

* Elide d (Delete(d1)).

* Create 1 resource, e2, not present in Step 1 (Create(e2)).

Checkpoint: a2, b2, c2, e2

# Step 3

Replace a resource:

* Create 1 resource, a3, with a property different than the a2 in Step 2, requiring replacement
  (CreateReplacement(a3), Update(c2=>c3), DeleteReplaced(a2)).

* Elide b (Delete(b2)).

* Create 2 resources, c3 and e3, equivalent to Step 2 (Same(c2, c3), Same(e2, e3)).

Checkpoint: a3, c3, e3

# Step 4

Fail during an update:

* Create 1 resource, a4, with a property different than the a3 in Step 3, requiring replacement
  (CreateReplacement(a4), Update(c3=>c4), DeleteReplaced(a3)).

* Inject a fault into the Update(c3=>c4), such that we never delete a3 (and it goes onto the checkpoint list).

Checkpoint: a4, c3, e3; pending delete: a3

# Step 5

Delete everything:

* Elide a (Delete(a4)).

* Elide c (Delete(c)).

* Elide e (Delete(e)).

* Pending delete (Delete(a3)).

Checkpoint: empty
