# Mu Cross-Cloud Targeting

The Mu metadata format and the primitive constructs in the `mu` namespace, like `mu/container`, are intentionally cloud-
agnostic and have been designed to support [many cloud targets](targets.md).  It is easy, however, to introduce cloud
dependencies by relying on certain stacks.  For example, mounting an `aws/ebs/volume` for a database volume pins it to
the AWS IaaS provider; in fact, *any* such service in the transitive closure of dependencies pins the whole stack to 
AWS.  Facilities do exist for conditional metadata inclusion, which can intelligently choose dependencies based on the
cloud target, however using these conditionals aggressively can quickly turn into an `#ifdef` nightmare.

To address this challenge and help developers build cloud-agnostic services, applications, and even whole clusters, Mu
offers the `mu/x` namespace.  The services offered in this namespace have already been conditionalized internally and
are guaranteed to run on all clouds, including locally for development and testing.  The differences between them have
been abstracted and unified so that you can configure them declaratively, using a single logical set of options, and
rely on the service internally mapping to the cloud provider's specific configuration settings.

For example, `mu/x/fs/volume` implements the `mu/volume` abstract interface, and maps to an AWS Elastic Block Store
(EBS), Azure Data Disk (DD), or GCP Persistent Disk (PD) volume, depending on the IaaS target.  Although the details for
each of these differs, a standard set of options -- like capacity, filesystem type, reclaimation policy, storage class,
and so on -- and the Mu framework handles mapping these standard options to the specific underlying ones.

Depending on the `mu/x` namespace makes building higher level cloud-agnostic services and applications easier.  For
instance, imagine a standard database service with a persistent volume argument.  It might want to choose a default for
ease of use.  But picking one that keeps that database service cloud-neutral is difficult.  So the common approach would
be to forego the default, complicating the end user experience.  By using `mu/x/fs/volume`, however, we get the best
outcome: the service is easy to use, and remains deployable to any cloud.

TODO(joe): a full list of these services.

