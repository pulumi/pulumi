# Maintaining Pulumi Forks with Claude Code

This guide helps maintainers of Pulumi forks efficiently resolve merge conflicts and upgrade issues using Claude Code AI assistance.

> 📖 **Also see:** [`CLAUDE_CODE_GUIDELINES.md`](./CLAUDE_CODE_GUIDELINES.md) for general Claude Code best practices and advanced prompting techniques.

## Quick Start: Fork Upgrade with Claude Code

> 🎯 **First Time?** Run the complete modification discovery process using [`MODIFICATION_DISCOVERY_PROMPT.md`](./MODIFICATION_DISCOVERY_PROMPT.md) to create your fork's baseline analysis. This creates a `FORK_ANALYSIS_[DATE].md` file that will dramatically improve all future Claude Code interactions.

### 1. Pre-Upgrade Assessment

**Option A: Full Analysis (Recommended for major upgrades)**
Use the complete modification discovery system:
1. Run the prompt from [`MODIFICATION_DISCOVERY_PROMPT.md`](./MODIFICATION_DISCOVERY_PROMPT.md)
2. Save the generated analysis as `FORK_ANALYSIS_[DATE].md`
3. Reference this analysis in all upgrade-related Claude sessions

**Option B: Quick Assessment (For minor upgrades)**
Before upgrading your fork, run these diagnostic commands and provide the output to Claude:

```bash
# Identify your custom changes since last upstream sync
git log --oneline upstream/v3.186.0..HEAD | head -20

# Find files you've modified that might conflict  
git diff --name-only upstream/v3.186.0 HEAD | grep -E "(backend|client|engine|cmd)" | head -10

# Check for custom test modifications
find . -name "*_test.go" -exec git diff --quiet upstream/v3.186.0 HEAD -- {} \; || echo "Custom tests found"

# Identify your most frequently modified files
git log --name-only upstream/v3.186.0..HEAD | grep -v "^$" | grep -v "^commit" | sort | uniq -c | sort -nr | head -10
```

### 2. Upgrade Conflict Resolution

When tests fail after merging upstream changes, use this enhanced prompt template with Claude:

```markdown
**Fork Upgrade Issue - Need Systematic Help**

I'm upgrading my Pulumi fork from v3.X.X to v3.Y.Y and encountering test failures.

**Fork Analysis Reference:**
[If available] See my complete fork analysis: `FORK_ANALYSIS_[DATE].md`
[Key areas from analysis]: [list your high-risk modification areas]

**Diagnostic Information:**
[Paste output from Pre-Upgrade Assessment commands above]

**Failing Tests:**
- TestName1: [error message]
- TestName2: [error message]

**My Custom Modifications (High Level):**
[Brief description of what functionality you've added/modified]

**Specific Conflict Areas:**
[Based on your fork analysis, list the areas most likely to conflict]

**Request:** Please analyze the upstream changes systematically and help me resolve conflicts while preserving my custom functionality. Reference my fork analysis for context on what must be preserved.
```

## Common Fork Conflict Patterns

### Pattern 1: Function Signature Changes
**Symptoms:** Compilation errors, "too many arguments" or "not enough arguments"

**Claude Prompt Template:**
```markdown
My fork has compilation errors after upgrade. Please analyze:

```bash
# Show function signature changes
git diff upstream/v3.186.0..upstream/v3.187.0 -- [failing_file.go] | grep -A5 -B5 "func.*("
```

Help me update my custom code to match new function signatures while preserving my modifications.
```

### Pattern 2: Test Baseline Changes  
**Symptoms:** Tests pass but assertions fail, JSON output mismatches

**Claude Prompt Template:**
```markdown
My tests are failing with assertion mismatches after upgrade:

```bash
# Compare test baselines
find . -path "*/output/*" -name "*.json" -o -name "*.txt" | head -10 | xargs -I {} git diff upstream/v3.186.0..upstream/v3.187.0 -- {}
```

[Paste failing test output]

Should I regenerate baselines or update custom logic? Help me understand what changed.
```

### Pattern 3: New Required Method Calls
**Symptoms:** Runtime panics, unexpected nil pointers, missing functionality

**Claude Prompt Template:**  
```markdown
My fork has runtime issues after upgrade:

[Paste error/panic output]

```bash  
# Check for new method additions in files I've modified
git diff upstream/v3.186.0..upstream/v3.187.0 -- [my_modified_files] | grep -E "^\+.*func|^\+.*\w+\("
```

Help me identify what new calls I need to add to my custom code.
```

## Critical Files for Fork Maintainers

### High-Risk Conflict Areas
Monitor these files closely - they change frequently and affect core functionality:

```bash
# Backend operations (state management, deployments)
pkg/backend/httpstate/backend.go
pkg/backend/httpstate/backend_test.go
pkg/backend/httpstate/client/client.go

# Engine operations (resource lifecycle)  
pkg/engine/lifecycletest/*.go
pkg/resource/deploy/*.go

# CLI operations
pkg/cmd/pulumi/operations/*.go
```

### Claude Analysis Command
Run this and share output when asking Claude for help:

**Enhanced Analysis (with fork analysis reference):**
```bash
# Share your fork analysis first
echo "=== FORK ANALYSIS REFERENCE ==="
echo "Analysis File: FORK_ANALYSIS_[DATE].md"
echo "High Risk Areas: [list from your analysis]"
echo "Custom Features: [list from your analysis]"

# Generate comprehensive conflict analysis
echo -e "\n=== MODIFIED FILES ANALYSIS ===" 
git diff --name-only upstream/v3.186.0 HEAD | head -15

echo -e "\n=== RECENT UPSTREAM CHANGES ==="
git log --oneline upstream/v3.186.0..upstream/v3.187.0 | head -10  

echo -e "\n=== FUNCTION SIGNATURE CHANGES ==="
git diff upstream/v3.186.0..upstream/v3.187.0 -- $(git diff --name-only upstream/v3.186.0 HEAD | grep "\.go$" | head -5) | grep -E "^[-+].*func.*\(" | head -10

echo -e "\n=== TEST IMPACT ==="
git diff --stat upstream/v3.186.0..upstream/v3.187.0 | grep test | head -5
```

**Quick Analysis (without fork analysis):**
```bash
# Use this if you haven't created a fork analysis yet
[Previous command content as fallback]
```

## Fork-Specific Upgrade Strategies

### Strategy 1: Preserve-and-Adapt
Best for: Small custom modifications, additional logging, extra validation

**Approach:** Adapt your changes to new interfaces while keeping functionality intact
**Claude Guidance:** "Help me adapt my custom [X] functionality to the new [Y] interface"

### Strategy 2: Override-and-Extend  
Best for: Significant behavioral changes, custom backends, alternative implementations

**Approach:** Maintain your custom implementation as primary, selectively adopt upstream improvements
**Claude Guidance:** "I've implemented custom [X]. What upstream improvements in [Y] should I adopt?"

### Strategy 3: Hybrid Integration
Best for: Complex forks with both behavioral and interface changes

**Approach:** Systematic analysis of each conflict to determine preserve vs. adopt
**Claude Guidance:** "Analyze each conflict in [file] and recommend preserve vs. adopt for my use case: [description]"

## Testing Your Fork After Upgrade

### Validation Commands
```bash
# Quick smoke test
make test_fast

# Test your custom functionality specifically  
go test -run "Test.*Custom.*" ./...

# Test critical paths your fork modifies
go test -run "Test.*(Backend|Deploy|Import)" ./pkg/backend/httpstate/...

# Generate new baselines if needed (use carefully!)
PULUMI_ACCEPT=1 make test_all
```

### Claude Validation Prompt
```markdown  
I've resolved conflicts in my fork. Please help me validate:

**Changes Made:**
[Summarize your resolutions]

**Test Results:**
[Paste any remaining test failures]

**Validation Request:** Review my changes for correctness and suggest additional tests to verify my custom functionality still works.
```

## Emergency Recovery

If your upgrade breaks critical functionality:

```bash  
# Create recovery point
git tag fork-backup-$(date +%Y%m%d)

# Quick rollback
git reset --hard upstream/v3.186.0
git cherry-pick [your-custom-commits]

# Ask Claude for help
# Provide: git log --oneline fork-backup-YYYYMMDD..HEAD
```

## Success Indicators

Your fork upgrade is successful when:
- [ ] All existing tests pass  
- [ ] Your custom functionality works as expected
- [ ] Performance characteristics are maintained
- [ ] No new warnings/errors in logs
- [ ] Integration tests with your custom features pass

Use Claude to verify each checkpoint by sharing test results and asking for validation.