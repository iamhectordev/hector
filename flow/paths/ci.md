# CI

Run repository validation in GitHub Actions without third-party actions.

## Principles
- CI runs on pull requests to `main` and pushes to `main`.
- Workflow permissions stay read-only unless a later release slice requires writes.
- Do not use third-party GitHub Actions for checkout or tool setup.
- Pin external tools to concrete versions and verify downloaded binaries with SHA256.
- Execute repo validation through `mise` tasks so local and CI behavior match.

## Outline
```yaml
on:
  pull_request:
    branches: [main]
  push:
    branches: [main]

permissions:
  contents: read

jobs:
  ci:
    runs-on: ubuntu-24.04
    steps:
      - run: git fetch the push SHA or pull request merge ref
      - run: download pinned mise release and verify SHA256
      - run: mise install
      - run: mise run lint
      - run: mise run test
      - run: mise run build
```

## Example
See `.github/workflows/ci.yml`.
