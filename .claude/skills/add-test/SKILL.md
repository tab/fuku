---
name: add-test
description: Write a Go test using table-driven tests with the mocks-once-at-top pattern. Use when adding tests, refactoring tests, or fixing failing tests.
---

# Writing a test

## Defaults

- Prefer **table-driven tests (TDT)** for several scenarios of one function.
- Never write several `t.Run()` blocks in one test function. Use TDT.
- A single case that does not fit TDT is `Test_<MethodName>_<TestCase>` (`Test_Load_ExplicitPathNotFound`).
- Use `testify` for assertions.
- Use `go.uber.org/mock` (mockgen) — never `testify/mock`. See `generate-mock`.

## Structure: Arrange, Act, Assert

Every test reads top to bottom: Arrange, Act, Assert. Blank lines separate them. Never comments like `// Arrange`.
Arrange has a fixed order: the controller, the mocks in the order the code uses them, the real values, then plain variables:

```go
func Test_<FuncName>(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockRepo := NewMockRepository(ctrl)

    cfg := DefaultConfig()

    user := User{ID: "42", Name: "alice"}

    result := NewService(mockRepo, cfg).Register(user)

    assert.Equal(t, expected, result)
}
```

- Act and Assert may be the TDT loop; Arrange stays above the table
- never create a variable inside the Act. A case gets what it needs from its `before` func
- no `if`/`else` in Act or Assert. Move a branch into `before`, or give the case its own test
- a value the Act needs is a named variable in Arrange, not an inline expression.
  `log := slog.New(slog.DiscardHandler)` in the real-values group, then `NewService(mockRepo, log)`
- no sugar helpers (`testLogger()`, `newTestService()`). Write the composition inline

## Mocks: one layer down

Read the code under test as layers. The package under test is A. Its direct dependencies are B. What B calls is C.

```text
A  app/updater.Checker     tests run the real checker
B  updater.ReleaseSource   mocked through the checker's own interface (check_mock_test.go)
C  adapters/github.Client  never touched by the checker's tests
```

The same rule holds one level down.
`github.Client`'s tests run the real client against a mocked `HTTPDoer` and a temporary cache directory.
An adapter mocks only true externals: a network client, a clock.
A package that implements another package's interface never imports that package's mock.

- A's tests run A's real code. Never mock the package under test or its own helpers.
- B is mocked through the interface A declares. Expect what A must call: `mockService.EXPECT().Register(user).Return(nil)`.
- C is out of reach. A's test never builds a real B to observe A through B's output. A check that needs B's output belongs in B's tests.
- A stdlib no-op (`slog.New(slog.DiscardHandler)`, `io.Discard`) stands in for a B the test never asserts on.
  Use a generated mock only when the test asserts a call, its count or its arguments.
- Generated mocks are `<file>_mock_test.go` beside the `<file>_test.go` that uses them, in the consumer's package.
  They name only the interfaces a test mocks. See `generate-mock`.
- A bus consumer's tests never build a bus. They call the handler with `contracts.Message` values.
  A publisher's tests assert on a `MockPublisher`.
- A lifecycle participant (`Subscribe`/`Drain`, `Start`/`Stop`, `Run`) is tested through those methods.
  The coordinator is tested once in `bootstrap/lifecycle`.

## Pattern: mocks once at top, `before` per case

```go
func Test_Service_Register(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockRepo := NewMockRepository(ctrl)

    subject := NewService(mockRepo)

    tests := []struct {
        name   string
        before func()
        user   User
        expect error
    }{
        {
            name: "stores a new user",
            before: func() {
                mockRepo.EXPECT().Save(User{ID: "42"}).Return(nil)
            },
            user: User{ID: "42"},
        },
        {
            name: "reports a duplicate",
            before: func() {
                mockRepo.EXPECT().Save(User{ID: "42"}).Return(ErrDuplicate)
            },
            user:   User{ID: "42"},
            expect: ErrDuplicate,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            tt.before()

            err := subject.Register(tt.user)

            require.ErrorIs(t, err, tt.expect)
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

- Add a test to the `*_test.go` that matches the source file. Do not create a second file.
- Tests use the **same package** as the source (`package user`, not `user_test`).
- Assert the error **before** the result: `assert.Error(t, err)` before `assert.Nil(t, result)`.
- Deterministic inputs. No random generators.
- No comment before a subtest. `t.Run("description")` already says it.
- No godoc comment on a test function.
- Cover success and error cases.
- Mock one layer down. Tests are isolated and fast.
- Aim for ≥90% coverage.
- A test that needs a child process, a socket or a terminal lives in `e2e/`, against the built binary.
- Never `time.Sleep` to wait for a goroutine. Use a channel, a `WaitGroup` or a barrier the code closes.
- Never write a `module_test.go`. `bootstrap/modules/base_test.go` validates every composition once.
- CLI tests keep exit codes and errors in separate fields: `expectedExit`, `expectedError`.
- Test every command alias (`help`, `--help`, `-h`).
