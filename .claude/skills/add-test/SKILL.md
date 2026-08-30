---
name: add-test
description: Write a fuku test using table-driven tests with the mocks-once-at-top pattern. Use when adding tests, refactoring tests, or fixing failing tests.
---

# Writing a fuku test

## Defaults

- Prefer **table-driven tests (TDT)**. Use TDT whenever testing multiple scenarios for the same function.
- Never use multiple `t.Run()` blocks in a single test function — if you have multiple cases, use TDT.
- For a single case where TDT doesn't fit, use `Test_<MethodName>_<TestCase>` (e.g. `Test_Load_ExplicitPathNotFound`).
- Use `testify` for assertions.
- Use `go.uber.org/mock` (mockgen) — never `testify/mock`. See `fuku-generate-mock`.

## Pattern: mocks once at top, `before` per case

```go
func Test_Example(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockDep := NewMockDependency(ctrl)
    subject := &Implementation{dep: mockDep}

    tests := []struct {
        name   string
        before func()
        input  string
        expect bool
    }{
        {
            name:  "success case",
            input: "test-input",
            before: func() {
                mockDep.EXPECT().Method("test-input").Return(nil)
            },
            expect: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            tt.before()
            result := subject.TestMethod(tt.input)
            assert.Equal(t, tt.expect, result)
        })
    }
}
```

## Table case format

Always multi-line, one field per line:

```go
// GOOD
{
    name:     "test case",
    input:    "value",
    expected: true,
},

// BAD — never inline
{name: "test case", input: "value", expected: true},
```

## Rules

- Add new tests to the existing `*_test.go` file matching the source file name; do not create a new file when one already exists.
- Tests use the **same package** as the source (`package runner`, not `runner_test`).
- Error assertions come **before** result assertions: `assert.Error(t, err)` before `assert.Nil(t, result)`.
- Prefer deterministic test inputs — never use random generators for input.
- Do not add comments before subtests — `t.Run("description")` already says what the case does.
- Do not use godoc-style comments on test functions.
- Cover both success and error scenarios.
- Mock all external dependencies; tests must be isolated and fast.
- Aim for ≥90% coverage.
- Skip integration tests that hang or require subprocesses with `t.Skip(...)` and a descriptive message.
- For CLI tests, separate exit codes from errors in fields like `expectedExit` and `expectedError`.
- Test all command aliases (e.g. `help`, `--help`, `-h`).
