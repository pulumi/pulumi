Stress-testing rewriting and normalizing aliases in the engine state.

Benchmark timings (1-shot, n=100, darwin mbp 2019):

```
pulumi       3.36.0-alpha.1657742945+3cfba73d
destroy      0m48.543s
up           0m43.026s
empty-update 0m32.936s

pulumi       v3.35.3
destroy      0m48.645s
up           0m41.580s
empty-update 0m32.341s
```


Non-quadratic version (1-shot, n=100, darwin mbp 2019):

```
pulumi       3.36.0-alpha.1657742945+3cfba73d
up           0m36.441s
empty-update 0m19.781s

pulumi       v3.35.3
up           0m36.265s
empty-update 0m19.574s
```

Same with n=1000

```
pulumi       3.36.0-alpha.1657742945+3cfba73d
up           6m40.879s
empty-update 4m8.565s

pulumi       v3.35.3
up           6m39.083s
empty-update 4m10.661s
```
