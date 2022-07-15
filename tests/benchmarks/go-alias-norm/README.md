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
