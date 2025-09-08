# Fork Analysis Template

This template provides the structure for documenting fork modifications discovered using the MODIFICATION_DISCOVERY_PROMPT.md. Copy this template and fill it with your analysis results.

## Executive Summary

**Fork Repository:** [your-org/pulumi]  
**Analysis Date:** [YYYY-MM-DD]  
**Upstream Base:** [version/commit]  
**Divergence:** [X commits ahead, Y commits behind]  
**Total Modified Files:** [count]  
**Risk Level:** [High/Medium/Low]

**Key Modifications:**
- [Brief description of major changes]
- [Custom functionality highlights] 
- [Architectural modifications]

## Detailed Modification Inventory

### High Risk Modifications (Likely Upgrade Conflicts)

#### Backend System Changes
- **File:** `pkg/backend/httpstate/backend.go`
  - **Type:** [function_signature|new_method|behavioral_change]  
  - **Functions:** [list of modified functions]
  - **Description:** [what was changed and why]
  - **Upgrade Impact:** [how future upgrades might conflict]

- **File:** `pkg/backend/httpstate/client/client.go`
  - **Type:** [function_signature|new_method|behavioral_change]
  - **Functions:** [list of modified functions] 
  - **Description:** [what was changed and why]
  - **Upgrade Impact:** [how future upgrades might conflict]

#### Engine Core Changes  
- **File:** `pkg/engine/[specific_file].go`
  - **Type:** [modification type]
  - **Functions:** [list]
  - **Description:** [changes made]
  - **Upgrade Impact:** [conflict prediction]

#### Resource Management Changes
- **File:** `pkg/resource/deploy/[specific_file].go`
  - **Type:** [modification type]
  - **Functions:** [list]
  - **Description:** [changes made]
  - **Upgrade Impact:** [conflict prediction]

### Medium Risk Modifications (May Conflict)

#### CLI Command Changes
- **Files:** `pkg/cmd/pulumi/[commands].go`
  - **Type:** [new_command|flag_addition|behavior_change]
  - **Description:** [what was added/modified]
  - **Dependencies:** [what it relies on]

#### SDK Modifications
- **Go SDK:** `sdk/go/[areas]`
- **Node.js SDK:** `sdk/nodejs/[areas]`  
- **Python SDK:** `sdk/python/[areas]`
- **Description:** [cross-language changes made]

### Low Risk Modifications (Unlikely to Conflict)

#### Documentation & Comments
- **Files:** [list of files with doc changes]
- **Type:** [documentation|comments|formatting]

#### Logging & Debugging
- **Files:** [list of files with logging changes]
- **Type:** [logging_enhancement|debug_features]

## Critical Conflict Points

### Area 1: [Backend Operations]
**Files:**
- [list of specific files]

**Conflict Likelihood:** [High/Medium/Low]  
**Upgrade Impact:** [description of how upgrades will affect this area]  
**Mitigation Strategy:** [how to handle conflicts]

### Area 2: [Engine Lifecycle]
**Files:**
- [list of specific files]

**Conflict Likelihood:** [High/Medium/Low]
**Upgrade Impact:** [description]
**Mitigation Strategy:** [how to handle conflicts]

## Custom Functionality Map

### Feature 1: [Custom Feature Name]
**Purpose:** [what this feature does]  
**Files Modified:**
- [list of files and their roles]

**Dependencies:**
- [upstream components it relies on]
- [potential conflict points]

**Testing:**
- [custom tests created]  
- [integration test coverage]

### Feature 2: [Another Custom Feature]
**Purpose:** [what this feature does]
**Files Modified:**
- [list of files and their roles]

**Dependencies:** 
- [upstream dependencies]

**Testing:**
- [test coverage]

## Test Infrastructure Changes

### New Tests Added
- **Test:** `Test[CustomFeature]`
  - **File:** [test file location]
  - **Purpose:** [what it validates] 
  - **Dependencies:** [what it requires]

### Modified Existing Tests
- **Test:** `Test[ExistingTest]`
  - **File:** [location]
  - **Changes:** [what was modified]
  - **Reason:** [why it was changed]

### Test Data & Fixtures
- **Custom Test Data:** [list of custom test files]
- **Modified Baselines:** [existing baselines that were changed]

## Upgrade Risk Assessment

### Next Minor Version (v3.X.X → v3.Y.Y)
**Risk Level:** [High/Medium/Low]  
**Predicted Conflicts:**
- [area 1]: [why it will conflict]
- [area 2]: [why it will conflict]

**Preparation Steps:**
- [what to do before upgrading]
- [tests to run]
- [backup strategies]

### Next Major Version (v3.X.X → v4.X.X)
**Risk Level:** [High/Medium/Low]
**Predicted Conflicts:**
- [major architectural changes expected]
- [breaking changes that will affect your fork]

**Migration Strategy:**
- [approach for handling major version upgrade]

## Maintenance Recommendations

### Regular Maintenance (Monthly)
- [ ] Review upstream changes: `git fetch upstream && git log HEAD..upstream/main --oneline`
- [ ] Check for security updates in modified areas
- [ ] Validate custom functionality still works
- [ ] Update this analysis if new modifications are added

### Pre-Upgrade Checklist  
- [ ] Create backup branch: `git branch fork-backup-$(date +%Y%m%d)`
- [ ] Review CHANGELOG for breaking changes in your modified areas
- [ ] Run full test suite on current version
- [ ] Identify potential conflicts using this analysis
- [ ] Prepare rollback plan

### Post-Upgrade Validation
- [ ] All existing tests pass
- [ ] Custom functionality works as expected  
- [ ] Performance benchmarks maintained
- [ ] Security posture preserved
- [ ] Update this analysis document

## Claude Code Integration Points

### Quick Context for Claude Code Sessions
```bash
# Share this with Claude for immediate context:
echo "Fork Analysis: $(date)"
echo "Modified Files: [count from analysis]" 
echo "High Risk Areas: [list key areas]"
echo "Custom Features: [list main features]"
```

### Reference for Conflict Resolution
When upgrade conflicts occur:
1. Share relevant section from this analysis
2. Identify which "Critical Conflict Points" are involved  
3. Reference the "Mitigation Strategy" for that area
4. Use the custom functionality map to explain what must be preserved

### Development Context
For new development or debugging:
1. Check if new work affects any "High Risk" areas
2. Reference custom functionality dependencies  
3. Ensure changes don't break existing custom features
4. Update this analysis if significant modifications are made

---

## Analysis Maintenance

**Last Updated:** [date]  
**Updated By:** [person]  
**Next Review Due:** [date + 3 months]  
**Upstream Version Tracked:** [version]

**Change Log:**
- [date]: Initial analysis
- [date]: Updated after [specific changes]
- [date]: Major upgrade from [version] to [version]