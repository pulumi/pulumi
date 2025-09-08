# Claude Code Guidelines for Pulumi Development

This guide optimizes interactions with Claude Code for Pulumi development tasks, reducing iteration cycles and improving resolution quality.

> 🔧 **For Fork Maintainers:** See [`FORK_MAINTENANCE.md`](./FORK_MAINTENANCE.md) for specialized guidance on resolving merge conflicts and upgrading divergent forks.

## Essential Setup for Claude Code Sessions

> 🔍 **For Fork Maintainers:** Before any Claude Code session, ensure you have a current fork analysis using [`MODIFICATION_DISCOVERY_PROMPT.md`](./MODIFICATION_DISCOVERY_PROMPT.md). This provides Claude with comprehensive context about your modifications and prevents repeated discovery cycles.

### 1. Initialize Context Commands
Always run these commands first and provide output to Claude Code:

```bash
# Repository context
git status --porcelain | head -10
git log --oneline -5
pwd

# Build status  
make --version && go version && node --version 2>/dev/null || echo "Node not available"

# Recent changes context (if working with version comparison)
git diff --name-only HEAD~3..HEAD | head -10
```

### 2. Problem-Specific Context

**For Test Failures:**
```bash
# Test context
go test -v ./[failing_package] 2>&1 | tail -20

# Find related test files
find . -name "*_test.go" -exec grep -l "[TestName]" {} \;

# Check test data dependencies
find . -path "*/testdata/*" -o -path "*/output/*" | grep [TestName] | head -5
```

**For Build Issues:**
```bash
# Compilation context
go build ./... 2>&1 | head -15

# Module dependencies
go list -m all | grep pulumi | head -10

# Import analysis for specific file
go list -f '{{.Deps}}' ./pkg/[specific_package]
```

**For Fork Conflicts:**
```bash
# Enhanced fork context (if you have analysis)
echo "=== FORK ANALYSIS REFERENCE ==="
ls -la FORK_ANALYSIS_*.md | tail -1
echo "High Risk Areas: [from your analysis]"

# Conflict analysis  
git diff --name-only HEAD upstream/main | head -10
git log --oneline HEAD..upstream/main | head -10
git diff --stat HEAD upstream/main
```

## Optimal Prompting Patterns

### Pattern 1: Systematic Problem Solving
```markdown
**Context:** [Brief description of what you're trying to achieve]

**Fork Analysis Reference (if applicable):**
[Reference to your FORK_ANALYSIS_[DATE].md file and relevant sections]

**Current State:** 
[Output from Essential Setup commands]

**Issue:** 
[Specific error messages or unexpected behavior]

**Request:** 
Please analyze systematically and provide step-by-step resolution. Use the TodoWrite tool to track progress.
```

### Pattern 2: Code Understanding  
```markdown
**Architecture Question:**
I need to understand how [specific functionality] works in Pulumi.

**Files I'm looking at:**
- [file1.go]: [brief description of relevance]  
- [file2.go]: [brief description of relevance]

**Specific Question:**
[Detailed question about architecture, data flow, or implementation]

**Context Commands:**
```bash
# [Include relevant grep/find commands Claude should run]
```

**Request:** 
Please analyze the codebase and explain the architecture. Feel free to read multiple files and use search tools.
```

### Pattern 3: Implementation Guidance
```markdown
**Implementation Task:** 
[What you want to implement]

**Current Understanding:**
[What you know about the area you're modifying]

**Constraints:**
- Must maintain backward compatibility
- Must follow existing patterns in [specific area]
- Must include appropriate tests

**Guidance Needed:**
1. Identify the right files/packages to modify
2. Understand existing patterns to follow  
3. Implement with proper error handling
4. Add appropriate tests
5. Validate with existing test suite

**Request:**
Please guide me through this implementation step-by-step using the TodoWrite tool.
```

## Claude Code Tool Usage Patterns

### Search and Analysis Tools
```markdown
# Use these patterns when asking Claude to investigate:

"Please use Grep to find all occurrences of [pattern] in [area]"
"Please use Glob to find all files matching [pattern] in [directory]" 
"Please read [specific files] and analyze [specific aspect]"
"Please use the Task tool to research [complex topic] across the codebase"
```

### Code Modification Tools  
```markdown
# Use these patterns when asking Claude to modify code:

"Please use MultiEdit to make these changes to [file]: [list of changes]"
"Please use Edit to update [specific function] in [file] to [description]"
"Please use Write to create [new file] with [functionality]"
```

### Testing and Validation
```markdown
# Use these patterns for testing with Claude:

"Please use Bash to run [specific test] and analyze the results"
"Please use Bash to run the build and fix any issues that arise" 
"Please use Bash to run linting and address any violations"
```

## Common Pulumi Development Scenarios

### Scenario 1: Understanding Test Failures
**Optimal Approach:**
1. Provide failing test name and error output
2. Share relevant file context using diagnostic commands
3. Ask Claude to analyze systematically
4. Request both immediate fix and explanation of root cause

**Example Prompt:**
```markdown
Test `TestCreateStackDeploymentSchemaVersion` is failing after upgrading from v3.186.0 to v3.187.0.

**Error Output:**
[paste error]

**Context:**
```bash
git diff v3.186.0..v3.187.0 -- pkg/backend/httpstate/backend_test.go | head -50
find . -name "*backend*test*" -exec grep -l "DeploymentSchemaVersion" {} \;
```

**Request:** Please analyze what changed in the deployment schema handling and help me understand why my test is failing. Use systematic analysis and provide both fix and explanation.
```

### Scenario 2: Function Signature Investigation  
**Optimal Approach:**
1. Identify the changed function signature
2. Find all usage locations
3. Understand the impact on existing code
4. Get guidance on updating calls

**Example Prompt:**
```markdown
I'm seeing compilation errors about `PatchUpdateCheckpoint` function. 

**Investigation Commands:**
```bash
git diff v3.186.0..v3.187.0 -- pkg/backend/httpstate/client/client.go | grep -A10 -B5 "PatchUpdateCheckpoint"
grep -r "PatchUpdateCheckpoint" . --include="*.go" | head -10
```

**Request:** Please analyze what changed in this function signature, find all places it's called, and help me update the calling code appropriately.
```

### Scenario 3: Architecture Understanding
**Optimal Approach:**  
1. Specify the architectural area you're exploring
2. Provide context about your goals
3. Ask for systematic exploration
4. Request documentation of findings

**Example Prompt:**
```markdown
I need to understand how Pulumi's checkpoint versioning system works, specifically the new V4 support.

**Context:** I'm working on custom backend modifications and need to understand:
- How version downgrading works
- When and why it's applied  
- How it affects custom backend implementations

**Request:** Please explore the codebase systematically using available tools to map out the checkpoint versioning architecture. Use the Task tool if needed for comprehensive research.
```

## Maximizing Claude Code Effectiveness

### Do's:
✅ **Provide Complete Context:** Always include diagnostic command output  
✅ **Be Specific:** Name exact test failures, files, functions  
✅ **Request Systematic Approach:** Ask Claude to use TodoWrite for tracking  
✅ **Leverage Tool Capabilities:** Encourage use of search, analysis, and modification tools
✅ **Ask for Explanations:** Request both fixes and understanding of why something broke

### Don'ts:
❌ **Vague Requests:** "My tests are broken" without specifics  
❌ **Missing Context:** Not providing git status, recent changes, or error output
❌ **Single-Shot Fixes:** Not asking for systematic analysis of root causes
❌ **Ignoring Tools:** Asking Claude to guess instead of researching with available tools
❌ **Batch All Problems:** Mixing multiple unrelated issues in one prompt

## Session Optimization Strategies

### Long Development Sessions
```markdown
# Start each new topic with context refresh:
**Previous Context:** [Brief summary of what was accomplished]
**Current Focus:** [New area of investigation]  
**Connection:** [How this relates to previous work, if any]
```

### Complex Problem Solving
```markdown
# Use progressive disclosure:
1. Start with high-level problem description
2. Let Claude investigate and ask clarifying questions
3. Provide requested diagnostics  
4. Ask for systematic breakdown using TodoWrite
5. Work through solution step-by-step
```

### Fork Maintenance Sessions
```markdown
# Always establish fork context first:
**Fork Analysis:** [Reference to FORK_ANALYSIS_[DATE].md]
**Fork Purpose:** [Why you maintain a fork]
**Custom Modifications:** [Summary from analysis - high-risk areas]
**Upgrade Goal:** [What version you're moving to]
**Risk Tolerance:** [How much breakage is acceptable]
**Critical Areas:** [From fork analysis - areas that must be preserved]
```

## Troubleshooting Claude Code Interactions

### If Claude Seems Confused:
1. **Reset Context:** Summarize current state and objective clearly
2. **Provide Fresh Diagnostics:** Re-run context commands and share output  
3. **Be More Specific:** Break down broad requests into focused questions
4. **Use Tools Explicitly:** Ask Claude to use specific tools for investigation

### If Solutions Don't Work:
1. **Share Results:** Always share what happened when you tried Claude's suggestions
2. **Provide Error Output:** Include complete error messages and stack traces
3. **Ask for Analysis:** Request investigation of why the solution didn't work
4. **Iterate Systematically:** Work through alternatives methodically

### If Performance Is Slow:
1. **Focus Scope:** Limit investigation to specific files or areas
2. **Use Targeted Tools:** Prefer Grep over Task tool for simple searches  
3. **Batch Related Questions:** Group related questions in one prompt
4. **Provide Specific Paths:** Give exact file paths instead of broad searches

Remember: Claude Code is most effective when given clear objectives, complete context, and systematic approach requests. The investment in providing thorough context up-front pays dividends in solution quality and reduced iteration cycles.