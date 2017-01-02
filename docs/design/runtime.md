## System Services

TODO(joe): describe package manager, artifact repository, CI/CD, the relationship between them, etc.

## Runtime Services

TODO(joe): describe logging, Mu correlation IDs, etc.

TODO(joe): describe what out of the box runtime features are there.  It would be nice if we can do "auto-retry" for
    service connections...somehow.  E.g., see https://convox.com/guide/databases/.

## Extensibility

There are three general areas of extensibility:

* *Container/runtime*: A service with a runtime footprint may be backed by a Docker container, VM, cloud-hosted
  abstraction (such as an AWS Lambda), or something else.  It is possible to use your favorite technologies and
  languages for implementing your service, in a 1st class way, without Mull dictating a preference.

* *RPC*: Any service with an RPC interface associated with it may be bound to anything that speaks a "JSON-like" wire
  protocol.  This might be your RPC framework of choice, such as gRPC or Thrift, or even hand-rolled REST over HTTP.

* *Events*: Any service that exposes events that can be subscribed to can be bound to any runtime that similarly deals
  with "JSON-like" payloads.  This might be your favorite PubSub framework, such as Kafka or AWS SNS.

Mull doesn't have a strong opinion on your choice, and permits mixing and matching, although in each case Mu also
provides a simple runtime choice that can be used by default for a seamless and simple experience.  Either way,
out-of-the-box providers exist to both bind to your favorite frameworks, and also generate code for them so that, for
instance, you needn't manually keep your RPC framework's interfaces in synch with Mull service interfaces.

