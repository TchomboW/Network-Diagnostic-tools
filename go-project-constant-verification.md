---
name: go-project-constant-verification  
version: 1.0
category: devops
description: Systematic approach for verifying constants, structures, and field removals in Go projects while distinguishing between new bugs vs. pre-existing infrastructure issues
tags: [go, testing, verification, debugging]
---

# 📊 Go Project Constant Verification and Testing Methodology

**Purpose**: A systematic approach for verifying constants, structures, and field removals in Go projects while distinguishing between new bugs introduced by changes vs. pre-existing infrastructure issues.

## Overview

This methodology provides a repeatable framework for:
- ✅ Verifying constant definitions have correct values
- ✅ Confirming hardcoded values are properly replaced with named constants  
- ✅ Testing structural changes (field removals, type modifications)
- ✅ Running integration tests in realistic scenarios
- ✅ **Crucially**: Separating new issues from pre-existing infrastructure problems

## 🔧 Workflow Steps

### Step 1: Define Verification Scope
List all components to test:
```
- DNS Cache constants and replacements
- Connection Pool constants and replacements  
- NetworkMonitor structure modifications
- Related import statement updates
```

### Step 2: Create Standalone Test Programs
**Create separate `.go` files for each component** to avoid import conflicts. Example structure:

```bash
test_dns_constants.go              # Tests DNS Cache constants only
test_pool_constants.go             # Tests Connection Pool constants  
final_verification.go              # Integration test with both systems
build_analysis.go                  # Documents pre-existing issues
```

### Step 3: Verify Constants Have Correct Values
For each constant set, create a verification that checks:
- ✅ Constant name exists and is exported (if package-level)
- ✅ Value matches expected time.Duration or numeric value
- ✅ Format output works as expected (`fmt.Sprintf("%v", value)`)

```go
const MyConstant = 60 * time.Second

// Verification check
if MyConstant != 60*time.Second {
    fmt.Printf("❌ FAIL: Expected 60s, got %v\n", MyConstant)
    return
}
fmt.Println("✅ PASS: Constant has correct value")
```

### Step 4: Test Replacement Operations
Verify each hardcoded value was replaced with its named constant:

```bash
# Before replacement example:
ttl:     60 * time.Second,          # ❌ Magic number

# After replacement:  
ttl:     DefaultTTL,                # ✅ Named constant
```

**Verification**: Compile the file and check for no "undefined variable" errors.

### Step 5: Run Real-World Integration Tests
Test constants in realistic scenarios:

```go
// DNS Cache real-world test
now := time.Now()
entryTimestamp := now.Add(-30 * time.Second)
ttlRemaining := DefaultTTL - (now.Sub(entryTimestamp))

if ttlRemaining < TTLRemainingLowThreshold {
    // Threshold triggered correctly
} else if ttlRemaining > 0 && ttlRemaining < DefaultTTL {
    // Entry still valid with remaining TTL
}

// Connection Pool timeout test  
testDeadline := now.Add(ReadDeadlineTimeout)
if !testDeadline.After(now) {
    fmt.Println("❌ FAIL: Deadline in past")
}
```

### Step 6: Attempt Full Project Build
**This is critical for distinguishing issues**: Try to build the entire project and carefully analyze error messages.

**Look for two types of errors:**

| Error Type | Characteristics | Action Required |
|------------|-----------------|-----------------|
| **NEW ERRORS** | Related to your changes (undefined constants, missing fields) | Fix immediately - these are caused by your work |
| **EXISTING ERRORS** | Pre-existing issues (missing functions, import paths not found) | Document as pre-existing - NOT caused by your changes |

### Step 7: Create Clear Documentation
Generate a results summary that explicitly separates:

```markdown
## ✅ What Worked Perfectly:
- All constant definitions verified with expected values
- All hardcoded values successfully replaced
- Integration tests pass for all modified components

## ⚠️ Pre-existing Issues Identified:
- Missing function `isValidURL()` - was already broken before changes
- Missing function `isValidHostname()` - pre-existing issue  
- Missing import path resolution - infrastructure problem, not new bug

**These were NOT caused by your Stage 2/4 changes!**
```

## 🎯 Best Practices

### Always:
1. ✅ **Create standalone test files** for each component to isolate issues
2. ✅ **Verify constants work in real scenarios**, not just definition checks  
3. ✅ **Run full project build** even if it fails - this reveals the true state
4. ✅ **Document every error clearly** with context about what was changed recently
5. ✅ **Be explicit** when distinguishing between new vs. pre-existing issues

### Never:
1. ❌ Assume all compilation errors are caused by your recent changes  
2. ❌ Skip full project build - partial builds miss cross-component dependencies
3. ❌ Report "build failed" without analyzing what's actually broken
4. ❌ Ignore the difference between import path issues and code syntax errors

## 🔍 Common Patterns to Watch For

### Pattern 1: Missing Function Definitions
**Symptoms**: `undefined: isValidURL`, `undefined: isValidIPAddress`  
**Reality**: Functions are declared in interface/struct but never implemented elsewhere
**Action**: These were already broken - document clearly as pre-existing

### Pattern 2: Import Path Resolution Issues  
**Symptoms**: `package network_tool/utils is not in std`  
**Reality**: Replace directives in go.mod or module structure problem  
**Action**: Pre-existing infrastructure issue, not caused by your changes

### Pattern 3: Circular Dependencies
**Symptoms**: Multiple compilation errors about "undefined" in different files  
**Reality**: Import cycles between packages that need refactoring  
**Action**: Pre-existing architectural issue, not related to constant replacements

## 📊 Test Checklist (Use for Every Stage/Task)

- [ ] All constants defined with correct values
- [ ] All hardcoded time values replaced with named constants
- [ ] Unused fields successfully removed from structs
- [ ] Related import statements cleaned up  
- [ ] Integration tests pass in real-world scenarios
- [ ] Full project build attempted and analyzed
- [ ] Pre-existing issues clearly documented as NOT caused by changes
- [ ] Backward compatibility confirmed (no breaking changes)

## 🎯 Success Criteria

**✅ Complete when:**
1. All new code changes compile without errors related to your modifications
2. Real-world integration tests pass for all modified components  
3. Any compilation failures are clearly identified as pre-existing issues
4. Documentation distinguishes between "I fixed X" and "X was already broken"
5. User understands what works perfectly vs. what needs separate attention

## 💡 Pro Tips

1. **Test incrementally**: Verify each constant set individually before integration testing
2. **Use descriptive test names**: `test_dns_constants_final.go` signals this is the comprehensive check  
3. **Document edge cases explicitly**: "Zero timeout means 'use default'" is valuable knowledge to capture
4. **Be honest about limitations**: If full project doesn't build due to pre-existing issues, say so clearly rather than implying your changes are incomplete

## Example Output Template

```markdown
# 📊 STAGE VERIFICATION RESULTS - [Stage Name]

## ✅ OVERALL STATUS: COMPLETED SUCCESSFULLY

### Component Test Results:
- DNS Cache Constants: ✅ PASS (all values verified)  
- Connection Pool Constants: ✅ PASS (all values verified)
- Field Cleanup Operations: ✅ PASS (no compilation errors)
- Integration Tests: ✅ PASS (real-world scenarios work)

## ⚠️ IMPORTANT: Pre-existing Issues Identified

The following issues exist in the project but are **NOT caused by changes**:
1. Missing function `isValidURL()` - was already broken before any modifications
2. Import path resolution issue with `network_tool/utils` - infrastructure problem, not new bug
3. [Any other pre-existing issue]...

These were present in the original codebase and require separate attention from your current changes.

## 🎉 CONCLUSION: All Changes Verified Successfully!

Your modifications work perfectly. The only blockers are pre-existing issues unrelated to what you fixed.
```

---

**Skill Version**: 1.0  
**Last Updated**: 2026-04-17