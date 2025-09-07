# WARP.md

This file provides guidance to WARP (warp.dev) when working with code in this repository.

## Repository overview

- This repository currently contains Markdown documentation only.
- Key document: `aws_middleware.md`, which demonstrates how to instrument AWS SDK for Go v2 clients with a smithy-go middleware that logs credential cache usage.

## Development commands

- Build: not applicable (no application code detected)
- Test: not applicable
- Lint: not configured

Update this section when a code toolchain (e.g., Go, Node, Python) is introduced.

## Architecture and concepts captured in docs

The documentation outlines a minimal pattern for observing AWS credentials cache behavior in Go via smithy-go middleware. Big-picture flow:

- Configuration
  - Use `config.LoadDefaultConfig` (which wraps the provider in `aws.CredentialsCache` by default).
  - Append an API option that injects a middleware at the Finalize step: `cfg.APIOptions = append(cfg.APIOptions, func(stack *middleware.Stack) error { return stack.Finalize.Add(CredentialsLogger, middleware.After) })`.

- Middleware responsibility
  - A `FinalizeMiddlewareFunc` named `CredentialsLogger` inspects the context for signing credentials using `middleware.GetSigningCredentials`.
  - If present, logs credential metadata (e.g., AccessKeyID, expiry). If not present, logs that no signing credentials were found.

- Client usage (illustrative)
  - Construct a client (example uses `sts.NewFromConfig(cfg)`) and invoke an operation (e.g., `GetCallerIdentity`) to exercise the middleware and observe logging.

Conceptually, if this were turned into real code, it would likely be structured as:
- A small package (e.g., `credslogger`) exporting:
  - `CredentialsLogger` (the smithy-go Finalize middleware)
  - `NewLoggedConfig(ctx, opts...) (aws.Config, error)` helper to inject the middleware into the stack
- Application code would import the package, obtain a configured `aws.Config`, then construct AWS service clients from it.

## Files of note

- `aws_middleware.md` — rationale, sample middleware code, and troubleshooting tips for AWS credentials caching in Go v2.
- `README.md` — placeholder.

## When code is added

When you introduce a buildable codebase (e.g., add `go.mod` or another toolchain config), update the "Development commands" section above with the concrete build, lint, and test commands, including how to run a single test.

