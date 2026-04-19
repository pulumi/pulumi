# TODO: Address PR #22584 Review Comments

## Approved Plan Steps:
1. [ ] Create TODO.md (done)
2. [ ] Edit sdk/python/lib/pulumi/runtime/settings.py: Add public async get_package_ref/set_package_ref, fold monitor_supports_parameterization check, remove _sync_monitor_supports_parameterization if unused.
3. [ ] Update tests if needed (sdk/python/lib/test/runtime/test_package_ref_cache.py).
4. [ ] Run make lint && make test_fast && make tidy_fix && make format_fix
5. [ ] git add . && git commit -m "fix(python): public async package_ref APIs per review (#22584)" && git push origin fix-python-package-ref-context-cache
6. [ ] gh pr ready 22584 --repo pulumi/pulumi (user: ensure gh CLI PATH fixed)

## Progress:
- Step 1 complete.
