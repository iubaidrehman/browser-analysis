# CEF (Scenario G) — Not Implemented

The spec (sections 7G, 40) defines CEF as an optional scenario that must only
be implemented after every other architecture works, as a separate
experimental branch, without compromising the primary benchmark.

## Why it is not implemented

- CEF (Chromium Embedded Framework) requires a C++/cgo build toolchain and
  platform-specific binaries. This project is a pure-Go controller driving
  Playwright Chromium; introducing CEF would add a fragile, heavy dependency
  for an optional scenario.
- The research question CEF would answer — off-screen rendering cost vs
  headless Chromium — is partially addressed by the existing headless
  scenario (B), which already measures rendering-isolated cost.

## Status

Documented as **not implemented** per the spec's explicit allowance. If the
research later requires it, it should be added as a separate experimental
branch (`scenario: cef` in a standalone YAML, a distinct worker type) without
touching the primary benchmark paths.
