## v0.10.0

### Added

### Changed

 - For local stacks, Pulumi now uses a seperate encryption key for each stack instead of one shared for all stacks, to
   encrypt secrets. You are now able to use a different passphrase between two stacks. In addition, the top level
   `encryptionsalt` member of the `Pulumi.yaml` is removed and salts are stored per stack in `Pulumi.yaml`.  Pulumi will
   automatically re-use the existing key for any local stacks in the Pulumi.yaml file which have encrypted, but future
   stacks will have new keys generated. There is no impact to stacks deployed using the Pulumi Cloud.

