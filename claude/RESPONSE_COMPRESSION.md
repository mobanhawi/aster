# Response Compression Protocil
**Purpose**: Optimize context usage when token budget is constrained while preserving critical information.
  
## Quick Reference

| Context Remaining | Action |
|-------------------|--------|
| >40% | Full responses (default) |
| <40% | **Auto-compress** |
| User flag `--compressed` | Force compression |
| User flag `--full` | Force full responses |

## Activation

### Automatic Trigger

- **When**: Context remaining < 40% (watch for system warnings)
- **Action**: Switch to compressed format automatically
- **Priority**: Critical information > explanatory prose
  
### User Flags
Commands support explicit format control:
- `--full`: Detailed prose (default when context allows)
- `--compressed`: Minimal, action-oriented responses
- `--auto`: Explicit automatic mode (default behavior)

**Example**: `/implement-issue 123 --compressed`

## Compression Techniques

**1. Bullets over prose**: Replace paragraphs with concise bullet points
**2. Tables over text**: Use tables for structured information (priorities, options, comparisons)
**3. References over code**: Use `file:line` instead of code blocks where possible

## Compressed Response Template

```markdown
# [Task/Finding]

## Key Findings
- Critical observation 1
- Critical observation 2
- Critical observation 3
  
## Recommendations
| Priority | Action | Rationale |
|----------|--------|-----------|
| High | [Action] | [Brief reason] |
| Medium | [Action] | [Brief reason] |

## Next Steps
- Action item 1
- Action item 2
```

## What to Keep vs Omit

**Keep**:
- Critical findings, actionable recommendations
- File paths and line numbers (code references)
- Error messages (when debugging)
- Command syntax (exact commands)


**Compress**:
- Background explanations → Brief rationale only
- Examples → Maximum 1-2
- Transitions and prose → Bullets

**Omit**:
- Historical context
- Alternative approaches not recommended
- Extended code examples → Use `file:line` references
- Verbose explanations

## When NOT to Compress  

Never compress:
- Security-sensitive information requiring full context
- Complex architectural decisions
- Initial onboarding/teaching scenarios
- User explicitly requests `--full`

  