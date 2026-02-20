---

name: testing-strategy
description: |
    Testing patterns, conventions, and decision frameworks for fabric-accounts. Covers test type selection (unit/API/blackbox/integration), GWT naming, test structures, and coverage guidelines.

    ACTIVATE THIS SKILL when:
    - Writing, creating, implementing, or adding tests.
    - Planning, designing, or strategizing test approaches for features
    - Questions about "how to name tests" (GWT convention: GivenX_WhenY_ThenZ)
    - Working *_test.go files
    - Test coverage discussions, 
    - Questions about test data setup
    - Reviewing tests for naming conventions, structure, or completeness
    - Questions about "how many tests do I need", "what scenarios to test"
---  

# Testing Strategy Patterns

Testing knowledge: conventions, patterns, and decision frameworks.

## Test Type Decision Tree
  

```
Need to test new feature?

└─ Testing business functionality end-to-end? → Blackbox tests (PREFERRED)


Need to test internal logic?

└─ Pure function/package logic? → Unit tests

```
## Test Type Purposes

| Test Type | Purpose | Location | When to Use |
|-----------|---------|----------|-------------|
| **Blackbox** | End-to-end business functionality | `test/blackbox/` | **PREFERRED for new features** |
| **Unit** | Function/package logic | `*_test.go` | Validators, helpers, pure functions |

## Naming Conventions

### Test Functions
```go
// ✅ CORRECT
func TestGetAccount(t *testing.T) {}
func TestValidateRequest(t *testing.T) {}
  
// ❌ WRONG
func Test_GetAccount(t *testing.T) {} // No underscore in function name
func test_get_account(t *testing.T) {} // Wrong case
```

### Test Cases (GWT Paradigm)
```go
// ✅ CORRECT - Exactly 2 underscores, no spaces
"GivenValidAccount_WhenListAccounts_ThenReturnsAccount"
"GivenInvalidID_WhenGetAccount_ThenReturnsError"
"GivenMissingPermission_WhenCreateBucket_ThenPermissionDenied"

// ❌ WRONG
"Given Valid Account When List Accounts Then Returns Account" // Spaces
"Given_Valid_Account_When_List_Accounts_Then_Returns_Account" // Too many underscores
"TestListAccountsSuccess" // Not GWT format
```

**GWT Pattern:** `Given<Context>_When<Action>_Then<Outcome>`
  
## Unit Test Patterns

### Table-Driven Tests

```go
func TestAdd(t *testing.T) {
	// Define a slice of structs, where each struct represents a test case.
	// Each test case includes input values (a, b) and the expected output (want).
	testCases := []struct {
		name string
		a    int
		b    int
		want int
	}{
		{
			name: "Positive numbers",
			a:    2,
			b:    3,
			want: 5,
		},
		{
			name: "Negative numbers",
			a:    -1,
			b:    -5,
			want: -6,
		},
		{
			name: "Mixed numbers",
			a:    10,
			b:    -4,
			want: 6,
		},
		{
			name: "Zero values",
			a:    0,
			b:    0,
			want: 0,
		},
	}

	// Iterate through each test case in the slice.
	for _, tc := range testCases {
		// Use t.Run to create a subtest for each test case, named by tc.name.
		// This allows for individual reporting and execution of each test case.
		t.Run(tc.name, func(t *testing.T) {
			// Call the function under test with the current test case's inputs.
			got := Add(tc.a, tc.b)

			// Assert that the actual result matches the expected result.
			if got != tc.want {
				t.Errorf("Add(%d, %d) got %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

```

### Best Practices
- Use `guid.UUID` type instead of `string` for UUIDs
- No magic constants - define named constants
- Use testify assertions: `assert.NoError(t, err)`
- Coverage threshold: See `.github/COVERAGE_THRESHOLD`

## Test Coverage Strategy

**Unit Tests (Required):**
- Validator logic (all validation rules)
- Business logic functions
- Helper/utility functions
  
## Test Execution Commands

```bash
    go test -v -race ./...
```

## Common Test Scenarios

### New Function

1. Unit tests (table-driven) for:
    - Valid inputs
    - Invalid inputs (each validation rule)
    - Edge cases (null, empty, boundaries)

### Bug Fix

1. Add regression test that fails with bug
2. Fix bug
3. Verify test passes
4. Consider additional edge cases

## Anti-Patterns
  
❌ Test naming: `Test_GetAccount` or too many underscores
✅ `TestGetAccount` with GWT case names

❌ Magic constants in tests
✅ Named constants for reusable values

❌ Skipping error scenarios
✅ Test happy path + edge cases + errors