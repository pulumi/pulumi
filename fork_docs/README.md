# Complete Pulumi Fork Management System

**For maintaining Pulumi forks and resolving upgrade conflicts with Claude Code assistance**

## 🎯 Quick Start: What This System Does

This documentation system eliminates the frustrating back-and-forth when using Claude Code to resolve fork upgrade conflicts. Instead of multiple rounds of questions and partial context, you get **immediate, targeted solutions** with complete understanding of your modifications.

## ⚡ Prerequisites

Before using this system, ensure you have:
- **Git repository access** with upstream remote configured
- **Claude Code access** (claude.ai/code) 
- **Basic Git knowledge** for running diagnostic commands
- **30-60 minutes** for initial setup (one-time investment)

```bash
# Verify prerequisites
git remote -v | grep upstream  # Should show upstream remote
git log --oneline -5           # Should show recent commits
claude --version 2>/dev/null || echo "Use claude.ai/code web interface"
```

## 📁 Documentation Files Included

### Core System Files
- **`MODIFICATION_DISCOVERY_PROMPT.md`** (224 lines) - Comprehensive prompt for analyzing your fork's modifications
- **`FORK_ANALYSIS_TEMPLATE.md`** (216 lines) - Structured template for documenting your discoveries  
- **`FORK_MAINTENANCE.md`** (240 lines) - Step-by-step guide for resolving upgrade conflicts
- **`CLAUDE_CODE_GUIDELINES.md`** (290 lines) - Best practices for effective Claude Code interactions
- **`CLAUDE.md`** (76 lines) - This directory's Claude Code guidance

### Generated Files (You Create These)
- **`FORK_ANALYSIS_[DATE].md`** - Your fork's "DNA profile" (generated from template)
- **Additional analyses** - Updated versions as your fork evolves

## 🚀 How to Use This System

### Initial Setup (One-time per fork)

#### Step 1: Run the Discovery Analysis
1. Open Claude Code (claude.ai/code) in your fork directory
2. Copy the **complete prompt** from `MODIFICATION_DISCOVERY_PROMPT.md` 
3. Paste it into Claude Code session
4. Wait for Claude to systematically analyze your fork across 6 key areas:
   - Repository context and divergence analysis
   - Engine, backend, CLI, and SDK modifications  
   - Architectural impact assessment
   - Risk categorization for upgrade conflicts
   - Structured inventory generation
   - Comprehensive documentation creation

#### Step 2: Save Your Fork "DNA Profile"
1. Claude generates analysis using the `FORK_ANALYSIS_TEMPLATE.md` structure
2. Save the complete output as `FORK_ANALYSIS_[YYYY-MM-DD].md`
3. Commit this file to your repository for team access
4. Update after major modifications or every 3-6 months

### Daily Usage Scenarios

- **🔄 For Upgrades:** Reference your analysis in conflict resolution prompts
- **🔧 For Development:** Share relevant analysis sections for context  
- **🐛 For Debugging:** Use risk categorizations to predict problem areas
- **📈 For Maintenance:** Update analysis after significant modifications
- **👥 For Team Onboarding:** Share analysis to quickly explain fork modifications

## 💡 The Difference This Makes

### Before Using This System
```
You: "My tests are failing after upgrade"
Claude: "Can you show me what changed?"
You: [shares some git output]
Claude: "Can you also show me your custom modifications?"
You: [shares more context]  
Claude: "What about function signatures that changed?"
You: [more investigation...]
Claude: "Let me look at your specific test failures..."
[... 6-8 rounds of back-and-forth taking 45+ minutes ...]
```

### After Using This System
```
You: "My tests are failing after upgrade. See my fork analysis: FORK_ANALYSIS_2025-01-15.md. 
High risk areas: backend.go, client.go. Custom features: template downloads, schema validation. 
Failing tests: TestBackendUpdate, TestClientAuth"

Claude: [Has complete context immediately]
"I can see from your analysis that you've modified the deployment schema handling in backend.go 
and added custom authentication in client.go. The v3.187.0 changes likely conflict with your 
custom validation logic. Let me check the specific signatures that changed..."
[Provides targeted solution in first response - 5 minutes total]
```

## 🔧 System Architecture Benefits

- **📊 Systematic Context Gathering:** 6-phase analysis covers all modification vectors
- **⚠️ Risk-Based Categorization:** High/Medium/Low risk predictions for upgrade planning
- **♻️ Reusable Reference:** One analysis serves multiple Claude sessions for months
- **🔗 Integrated Documentation:** All guides work together using the same foundation
- **📋 Template-Driven Consistency:** Structured format ensures complete coverage
- **👥 Team Knowledge Sharing:** Analysis documents serve as fork knowledge base

## 📋 Typical Workflow Example

When you need to upgrade your Pulumi fork from v3.180.0 to v3.190.0:

1. **📋 Reference Analysis** → Load your existing `FORK_ANALYSIS_2025-01-15.md` 
2. **🔍 Quick Assessment** → Run quick diagnostic commands from `FORK_MAINTENANCE.md`
3. **⚠️ Identify Risk Areas** → Focus on High-risk modifications (backend.go, client.go)
4. **🤖 Enhanced Claude Session** → Provide complete context in first message
5. **🎯 Targeted Resolution** → Claude immediately understands conflicts and provides specific fixes
6. **✅ Validation** → Use main Pulumi CLAUDE.md commands for testing
7. **📝 Update Documentation** → Refresh analysis if significant changes occurred

**Result:** Transform "bunch of revisions and back-and-forth" into "immediate targeted resolution with complete context."

## 🎯 Value Proposition

This system creates a **"DNA profile"** for your fork that can be instantly shared with Claude Code for any development scenario, dramatically reducing iteration cycles and improving solution quality.

**📊 Metrics:**
- **Time Investment:** 30-60 minutes initial setup
- **Time Savings:** 2-4 hours saved on every upgrade and major development session  
- **Quality Improvement:** Targeted solutions instead of generic troubleshooting
- **Team Efficiency:** New developers understand fork in 15 minutes vs. 2-3 days

## ✅ Success Validation

You'll know the system is working when:
- **First Claude response** addresses your specific modifications
- **Context questions eliminated** - no "can you show me more" requests
- **Targeted solutions** reference your exact files and functions
- **Team velocity increased** for fork-related development

## 🔧 Maintenance and Updates

### When to Update Your Analysis
- **Major upstream merges** (new Pulumi releases)
- **Significant custom feature additions** 
- **Architecture changes** in your modifications
- **Quarterly reviews** (recommended)

### File Organization
```
your-fork/
├── FORK_ANALYSIS_2025-01-15.md    # Current analysis
├── FORK_ANALYSIS_2024-10-20.md    # Historical reference
├── fork_docs/                      # This documentation system
└── [your Pulumi code...]
```

## 🆘 Troubleshooting

### Common Issues

**❌ Analysis incomplete or vague**
- Ensure you copied the COMPLETE prompt from `MODIFICATION_DISCOVERY_PROMPT.md`
- Run all diagnostic commands before asking Claude to analyze
- Provide more specific context about your modifications

**❌ Claude still asks for context**
- Check that you're referencing the correct analysis file
- Ensure your analysis covers the specific area you're working on
- Consider generating an updated analysis

**❌ Lost or outdated analysis**
- Re-run `MODIFICATION_DISCOVERY_PROMPT.md` process
- Use git history to identify major changes since last analysis
- Document lessons learned for future maintenance

### Recovery Scenarios
If your analysis files are lost, you can rebuild using:
```bash
# Quick context recovery
git log --oneline upstream/main..HEAD | head -20
git diff --name-only --stat upstream/main HEAD
git log --name-only --pretty=format: upstream/main..HEAD | sort | uniq -c | sort -nr | head -20
```

## 🔄 Integration with Development Workflow  

### CI/CD Integration
- Store analysis files in version control
- Update analysis as part of major release processes  
- Reference in pull request templates for context

### Team Workflows
- Include analysis reference in bug reports
- Share analysis during code reviews involving modified areas
- Update analysis as part of onboarding new team members

---

## 🚀 Ready to Start?

1. **📖 Read** this README completely
2. **🔍 Open** `MODIFICATION_DISCOVERY_PROMPT.md`
3. **🤖 Launch** Claude Code in your fork directory  
4. **📋 Copy/paste** the complete discovery prompt
5. **💾 Save** your generated analysis as `FORK_ANALYSIS_[DATE].md`
6. **🎯 Use** the analysis in your next Claude Code session

**Questions?** Check `CLAUDE_CODE_GUIDELINES.md` for advanced usage patterns and `FORK_MAINTENANCE.md` for specific upgrade workflows.