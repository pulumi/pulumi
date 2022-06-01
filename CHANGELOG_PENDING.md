### Improvements

- [cli] Plugins will now shut themselves down if they can't contact the engine that started them.
  [#9735](https://github.com/pulumi/pulumi/pull/9735)
### Bug Fixes

- [sdk/nodejs] Fix a crash due to dependency cycles from component resources.
  [#9683](https://github.com/pulumi/pulumi/pull/9683)