# System operational pages

## Scope

This slice replaces the code-scan, API-assets, and system-settings placeholders
with real system-scoped pages. Three independent TDD work streams implemented the
pages in parallel; the root integration then connected the routes and removed the
old placeholder exports.

## Code scan

- Loads real scan records through `listScans` with loading, error, empty, and
  ready states.
- Displays source ref, commit, language, framework, status, creation time, and
  backend error text.
- Owners and maintainers can create a scan record and run it against an explicit
  server repository path. Other roles do not receive write controls; the backend
  remains the authoritative role boundary.
- Failed mutations preserve the already loaded list and show the actual API
  error.

## API assets

- Loads the real system-scoped API operation list.
- Displays method, path, summary, tags, verification status, lifecycle status,
  and update time; absent fields render as unknown rather than fabricated text.
- Provides local method/status/path filters and reports filtered and total counts.
- Does not expose fake import or write actions because no such mutation contract
  exists on this page.

## Environment settings

- Adds typed API client operations for environments and external secret
  references.
- Lists non-sensitive environment variables and active/disabled state.
- Owners and maintainers can create/update environments and save only supported
  external references (`vault://`, `secret://`, `aws-secrets://`,
  `gcp-secret://`). Sensitive-looking variable keys are rejected in the browser
  and again by the API.
- Reference loading is isolated per environment. Viewer responses that omit the
  reference location are rendered as hidden, never guessed or replaced with a
  fake value.

## Verification

- Page-specific work completed Red → Green independently.
- Integrated Web suite: 16 files and 71 tests passed.
- TypeScript and Vite production build passed with `/bizdevops/` base path and no
  development user ID.
- The public HTTP trial remains on its existing release because the combined Web
  tree contains the TLS-gated formal login flow.
