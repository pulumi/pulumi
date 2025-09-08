# Pulumi Fork Modification Discovery System

This prompt systematically discovers and documents all modification points in your Pulumi fork, creating a comprehensive reference for both fork maintenance and Claude Code assistance.

## Complete Fork Analysis Prompt

Copy and paste this entire prompt to Claude Code to generate a comprehensive modification analysis:

---

```markdown
# Comprehensive Pulumi Fork Modification Analysis

I need you to systematically analyze my Pulumi fork to create a complete inventory of modifications, conflict points, and architectural changes. This analysis will be saved for future reference during upgrades and development.

## Phase 1: Repository Context Discovery

Please run these commands and analyze the output:

```bash
# Repository structure and status
pwd && git remote -v && git branch -a | head -10

# Identify upstream reference point
git log --oneline --merges | head -5
git describe --tags --abbrev=0 2>/dev/null || echo "No tags found"

# Fork divergence analysis  
git log --oneline upstream/main..HEAD | wc -l
git log --oneline HEAD..upstream/main | wc -l

# Modified files overview
git diff --name-only upstream/main HEAD | wc -l
git diff --stat upstream/main HEAD | tail -10
```

## Phase 2: Modification Pattern Analysis

Please analyze each category systematically:

### A. Core Engine Modifications
```bash
# Engine and resource management changes
git diff --name-only upstream/main HEAD | grep -E "(engine|resource|deploy)" | head -10
git log --oneline upstream/main..HEAD -- pkg/engine/ pkg/resource/ | head -15

# Find custom engine functionality
grep -r "TODO\|CUSTOM\|FORK\|INTERNAL" pkg/engine/ --include="*.go" | head -10
```

### B. Backend and Client Modifications  
```bash
# Backend system changes (most critical for conflicts)
git diff --name-only upstream/main HEAD | grep -E "(backend|client)" | head -10
git diff upstream/main HEAD -- pkg/backend/httpstate/backend.go pkg/backend/httpstate/client/client.go | grep "^[+-]" | head -20

# Custom backend functionality
grep -r "func.*" pkg/backend/ --include="*.go" | grep -v "upstream" | head -10
```

### C. CLI and Operations Modifications
```bash
# Command-line interface changes  
git diff --name-only upstream/main HEAD | grep -E "(cmd|operations)" | head -10
git log --oneline upstream/main..HEAD -- pkg/cmd/ | head -10

# Custom CLI commands or flags
grep -r "cobra\|flag\|Command" pkg/cmd/ --include="*.go" | grep -E "(Add|New)" | head -10
```

### D. SDK and Language Host Modifications
```bash
# SDK changes across languages
git diff --name-only upstream/main HEAD | grep -E "sdk/(go|nodejs|python)" | head -15
git log --oneline upstream/main..HEAD -- sdk/ | head -15

# Custom SDK functionality
find sdk/ -name "*.go" -o -name "*.ts" -o -name "*.py" | xargs grep -l "TODO\|CUSTOM\|FORK" | head -10
```

### E. Test Infrastructure Modifications
```bash
# Test changes (critical for upgrade conflicts)
git diff --name-only upstream/main HEAD | grep "_test\.go" | head -15
git diff --stat upstream/main HEAD -- "*_test.go" | head -10

# Custom test patterns
find . -name "*_test.go" -exec grep -l "Test.*Custom\|Test.*Fork\|Test.*Internal" {} \; | head -10
```

## Phase 3: Architectural Impact Analysis

For each modification category, please analyze:

### Function Signature Changes
```bash
# Find function modifications in core files
git diff upstream/main HEAD -- pkg/backend/httpstate/backend.go pkg/backend/httpstate/client/client.go pkg/resource/deploy/deployment_executor.go | grep -E "^[-+].*func.*\(" | head -20

# Identify interface changes
git diff upstream/main HEAD | grep -E "^[-+].*interface.*\{" -A5 | head -20
```

### New Dependencies and Imports  
```bash
# Custom imports analysis
git diff upstream/main HEAD | grep -E "^[+-]import" | head -15
git diff upstream/main HEAD -- go.mod go.sum | head -20

# Custom package usage
git diff upstream/main HEAD | grep -E "^[+].*github\.com|^[+].*gopkg" | head -10
```

### Configuration and Build Changes
```bash
# Build system modifications
git diff --name-only upstream/main HEAD | grep -E "(Makefile|\.yml|\.yaml|\.json)" | head -10
git diff upstream/main HEAD -- Makefile | head -20

# Configuration changes
git diff upstream/main HEAD -- *.yaml *.yml *.json | head -20
```

## Phase 4: Risk Assessment Analysis

Please categorize each modification by conflict risk:

### High Risk (Likely to conflict in upgrades)
- Core engine functionality changes
- Backend operation modifications  
- Function signature alterations
- Test infrastructure changes

### Medium Risk (May conflict)
- SDK modifications
- CLI command additions
- New utility functions
- Configuration changes

### Low Risk (Unlikely to conflict)
- Documentation changes
- Comment additions
- Logging improvements
- Minor formatting changes

## Phase 5: Generate Modification Inventory

Please create a structured inventory in this format:

```yaml
fork_analysis:
  repository:
    upstream_divergence: [commits_behind, commits_ahead]
    total_modified_files: [count]
    analysis_date: [date]
  
  modifications:
    high_risk:
      - file: [path]
        type: [function_signature|new_method|behavioral_change]
        description: [brief description]
        functions_affected: [list]
        
    medium_risk:
      - file: [path] 
        type: [addition|enhancement|new_feature]
        description: [brief description]
        
    low_risk:
      - file: [path]
        type: [documentation|logging|formatting]
        description: [brief description]
  
  critical_conflict_points:
    - area: [backend|engine|cli|sdk]
      files: [list of files]
      upgrade_impact: [description]
      
  custom_functionality:
    - name: [feature name]
      files: [list of files]
      description: [what it does]
      dependencies: [what it depends on]
      
  testing_changes:
    - type: [new_tests|modified_tests|test_data]
      files: [list]
      purpose: [why changed]
```

## Phase 6: Save Analysis Results

Please save the complete analysis to a file called `FORK_ANALYSIS_[DATE].md` with:

1. **Executive Summary**: High-level overview of fork modifications
2. **Detailed Inventory**: Complete catalog from Phase 5  
3. **Upgrade Risk Assessment**: Specific guidance for future upgrades
4. **Conflict Prediction**: Areas most likely to have merge conflicts
5. **Custom Functionality Map**: Documentation of custom features
6. **Maintenance Recommendations**: Ongoing fork maintenance strategy

This analysis will serve as the foundation for all future upgrade planning and Claude Code assistance sessions.

## Expected Deliverables

After running this analysis, I should have:
- Complete inventory of all fork modifications
- Risk assessment for each change
- Upgrade conflict prediction
- Custom functionality documentation  
- Maintenance strategy recommendations

Please proceed systematically through each phase and provide detailed analysis at each step.
```

---

## Usage Instructions

1. **Save this prompt** as a reference for comprehensive fork analysis
2. **Run periodically** (suggested: before each major version upgrade) 
3. **Update the generated analysis file** after making significant fork modifications
4. **Reference the analysis** when using either FORK_MAINTENANCE.md or CLAUDE_CODE_GUIDELINES.md

The generated `FORK_ANALYSIS_[DATE].md` file becomes your fork's "modification DNA" that can be quickly shared with Claude Code for any development task.