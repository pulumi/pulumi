# Security

Mu's security concepts are inspired by [UNIX's security model](https://en.wikipedia.org/wiki/Unix_security) in addition
to [AWS's IAM system](http://docs.aws.amazon.com/IAM/latest/UserGuide/id.html).

## Personas

Mu's architecture is flexible enough to describe a range of scenarios -- from single- to multi-tenant -- and team sizes
-- from small developer teams doing DevOps to large Enterprise organizations with dedicated IT operations departments.
Although the details for different points on this spectrum vary greatly, Mu attempts to choose smart defaults that
encourage best practices like [defense in depth](
https://en.wikipedia.org/wiki/Defense_in_depth_(computing)) and the [principle of least privilege](
https://en.wikipedia.org/wiki/Principle_of_least_privilege).

That said, we see the following three personas emerging in modern cloud management:

* Application/service developer: A person who writes and deploys code into an existing cluster.
* Application/service ops: A person who performs operational activities against an existing cluster, including
    provisioning or managing new or existing applications or services.
* IT ops: A person who performs operational activities on the cluster and underlying infrastructure itself.

Note that a single person may assume multiple personas.  For instance, an ISV might choose to lump all developers into
the application/service developer and ops personas, but then have dedicated ops specialists who manage the
infrastructure.  A large Enterprise, on the other hand, might want specialized members in each distinct persona.

## Identity (Users, Roles, Groups)

There are three primary constructs:

* **User**: An account that may be logged into by a human.
* **Roles**: An account that is assumed by services and cannot be logged into directly.
* **Groups**: A collection of Users and/or Roles that are granted a set of permissions in bulk, based on membership.

To illustrate two ends of the spectrum, let's look at two cases.

First, a small team.  This case, the default if no extra settings are chosen, pre-populates the following:

* An `administrators` Group with full permissions to everything in the Cluster.
* An `administrator` User assigned to the `administrators` Group.
* A `developers` Group with full permissions to the entire set of the Stack's Services.
* A `developer` User assigned to the `developers` Group.
* One `service` Role per Stack, with read-only permissions to each of the Stack's Services.

In the small team case, any Cluster-wide operations must be performed by a User in the `administrators` Group.  One such
User, `administrator`, is available out-of-the-box to do just that.  Any operations that are specific to a Stack, on the
other hand, can be performed by someone in the `developers` Group, which has far fewer permissions.  One such User,
`developer`, is available out-of-the-box for this purpose.  Lastly, by having a `service` Role per Stack, we ensure that
code running in the live service cannot perform privileges operations against the Cluster or even the Stacks within it.

TODO(joe): having one Role per Stack sounds good in theory, but I suspect it will be difficult in practice due to
    shared resources.  We do understand dependencies, and prohibit cycles, however, thanks to capabilities.  So it's
    worth giving this a shot... (this is left as an exercise in the translation doc.)

Second, an Enterprise-wide, shared cluster.  In this case, we expect a operations engineer to configure the identities
with precision, defense-in-depth, and the principle of least privilege.  It is unlikely that `administrator` versus
`developer` will be fine-grained enough.  For instance, we may want to segment groups of developers differently, such
that they can only modify certain Stacks, or perform only certain operations on Stacks.

## Access Controls (ACLs)

TODO(joe): we still need to figure out the ACL "language" to use.  Perhaps just RWX for each Stack.  It's unclear how
    this muddies up the IAM mappings, etc. however.

## Secrets

TODO(joe): configuration/secrets; see https://github.com/docker/docker/issues/13490.

