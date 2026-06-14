# Releasing rewynd

A release is fully automated by pushing a git tag.

## Cut a release

```bash
git tag v0.1.0
git push origin v0.1.0
```

The [Release workflow](.github/workflows/release.yml) then:

1. **goreleaser** builds cross-platform binaries (linux/darwin/windows × amd64/arm64) and
   publishes a **GitHub Release** with archives + `checksums.txt`.
2. `scripts/release/build-npm.mjs` turns those binaries into per-platform npm packages
   (`@rewynd/cli-<platform>-<arch>`) and publishes them, plus the `rewynd` wrapper and
   `@rewynd/shim`, to **npm**.
3. The Python `rewynd` package is built and published to **PyPI**.

## Required secrets (repo -> Settings -> Secrets -> Actions)

| Secret | Used for |
|---|---|
| `NPM_TOKEN` | publishing `rewynd`, `@rewynd/shim`, `@rewynd/cli-*` |
| `PYPI_TOKEN` | publishing `rewynd` on PyPI |
| `GITHUB_TOKEN` | the GitHub Release (provided automatically) |

If a token is absent the corresponding publish step is skipped — the GitHub Release still happens.

## Publishing the packages after adding tokens

If you tagged a release before configuring `NPM_TOKEN` / `PYPI_TOKEN`, you do **not** need to
re-tag. Add the secrets, then go to **Actions → Release → Run workflow** and enter the existing
tag (e.g. `v0.1.0`). The manual run rebuilds the binaries locally (it does not re-publish the
existing GitHub Release) and pushes the npm + PyPI packages.

## Versioning

- The Go binary version is injected at build time via `-ldflags -X main.version={{.Version}}`.
- npm package versions are stamped from the tag by `scripts/release/build-npm.mjs`.
- The PyPI package version is stamped from the tag in the workflow.

## Local dry run (no publishing)

```bash
go install github.com/goreleaser/goreleaser/v2@latest
goreleaser release --snapshot --clean          # builds binaries + archives into ./dist
node scripts/release/build-npm.mjs 0.0.0-snap  # generates the npm platform packages
```
