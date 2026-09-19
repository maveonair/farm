# AGENTS.md

- When writing something intended for human consumption, including comments, commit messages, or replies, use as few words as possible. Pick every word carefully. Less is more.

- Avoid superlatives and praise. Do not tell me I am absolutely right. Give me the cold hard truth.

- Avoid magic numbers and strings. Extract recurring or meaningful values into descriptive constants or named types. Keep obvious one-off values inline. If a value comes from a specification, use a named constant.

- Reduce indentation. Avoid the Arrow Anti-Pattern. Prefer early returns and `continue`.

- Keep function names short. Prefer less than 30 characters.

- Avoid ambiguous boolean parameters. Use a named type or options when `Foo(true)` does not make the call site's intent obvious.

- Let code breathe. Add empty lines between logical blocks.

- Add short comments explaining what a non-obvious block does and why. Use examples when useful. Propose ASCII diagrams for complex systems.

- Treat visibility changes as API design changes. Keep fields and functions private or unexported unless external access is required. Ask for explicit approval before increasing visibility unless explicitly requested.

- Program to levels of abstraction. Encapsulate low-level mechanics behind dedicated abstractions. Calling code should work with domain concepts, not implementation details.

- Do not touch unrelated code. Minimize changed lines. Do not reformat, rename, comment, or refactor code unrelated to the requested change.

- Strictly preserve layer boundaries. A layer may only communicate with its immediate dependency layer. Do not bypass intermediate abstractions.

## Go

- Write idiomatic, boring Go. Prefer clarity over clever abstractions. Follow standard library conventions.

- Format changed Go files with `gofmt`. Run relevant tests and `go vet`. Run the race detector for concurrency changes.

- Keep APIs small. Default to unexported identifiers and export only what other packages require.

- Treat changing an identifier from unexported to exported as an API design change. Ask before doing so unless explicitly requested.

- Use short, descriptive Go names. Avoid redundant names such as `user.UserService`; prefer `user.Service`.

- Keep package names short, lowercase, and focused.

- Avoid generic packages such as `util`, `helpers`, `common`, or `misc`.

- Avoid package-level mutable state. Pass dependencies explicitly.

- Prefer useful zero values. Add constructors only when initialization, validation, or dependencies require them.

- Accept interfaces where abstraction is needed and return concrete types by default.

- Define interfaces where they are consumed. Keep them small.

- Do not introduce interfaces, generics, reflection, or other abstractions without a concrete need.

- Prefer values. Use pointers when mutation, identity, synchronization, large copies, or meaningful nil semantics require them.

- Model domain states with named types and constants rather than raw strings, integers, or ambiguous booleans.

- Handle errors explicitly. Never silently discard an error unless doing so is intentionally safe.

- Add useful context when propagating errors. Use `%w` when callers may inspect the cause.

- Use `errors.Is` and `errors.As` instead of matching error strings.

- Do not both log and return the same error unless the layer intentionally owns both responsibilities.

- Do not use `panic` for expected failures.

- Error strings should normally start lowercase and not end with punctuation.

- Pass `context.Context` as the first parameter.

- Do not store `context.Context` in structs except where an existing API requires it.

- Propagate cancellation through blocking and long-running work.

- Every goroutine must have a clear lifetime and termination path.

- Avoid goroutines that can leak.

- Make channel ownership clear. Only the owner closes a channel.

- Prefer simpler synchronization when channels add no value.

- Keep synchronization primitives next to the state they protect.

- Never copy synchronization primitives after use.

- Use `defer` for cleanup when it keeps resource ownership clear.

- Release every acquired resource on all paths.

- Do not expose mutable internal slices, maps, or pointers unless callers are intentionally allowed to modify them.

- Prefer the standard library when it solves the problem cleanly.

- Add dependencies when they provide clear value over maintaining equivalent code locally.

- Avoid import cycles by fixing package boundaries, not by moving unrelated code into a shared package.

- Do not generate getters and setters mechanically. Expose behavior rather than representation.

- Use table-driven tests when multiple cases exercise the same behavior.

- Test observable behavior rather than implementation details.

- Keep tests deterministic. Avoid sleeps, execution-order assumptions, uncontrolled randomness, and external network dependencies.

- Use `t.Helper()` in test helpers.

- Use `t.Cleanup()` for test-owned cleanup.

- For time-dependent behavior, inject or abstract time when direct `time.Now` calls would make tests unreliable.

- Do not manually edit generated files. Change their source and regenerate them.

- Do not suppress compiler, vet, or linter findings without a specific reason.

- Treat compiler and linter findings as design feedback. Fix the cause instead of suppressing the symptom where practical.

## Bug fixes

- If the prompt indicates that a bug is being fixed, do not write the fix first.

- First write the smallest test that reproduces the bug.

- Run it and confirm it fails for the expected reason.

- Implement the fix.

- Run the test again and confirm it passes.

- Do not weaken, delete, or rewrite an existing test merely to make a new implementation pass unless the required behavior changed.

## Commit messages

Follow these rules:

1. Separate the subject from the body with one blank line.

2. Limit the subject to 50 characters. 72 characters is the absolute maximum.

3. Capitalize the first letter of the subject.

4. Do not end the subject with a period.

5. Use the imperative mood. The subject must complete:

   `If applied, this commit will <subject>`

6. Wrap the body manually at 72 characters.

7. Explain what and why, not how. Assume the code explains the implementation.
