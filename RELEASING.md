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

## Publishing credentials

| Target | Credential |
|---|---|
| **npm** (`@rewynd/cli`, `@rewynd/shim`, `@rewynd/cli-*`) | repo secret `NPM_TOKEN` |
| **PyPI** (`rewynd`) | **Trusted Publishing** (OIDC) — no token; configure at <https://pypi.org/manage/account/publishing/> with owner `SrinjoyDev`, repo `rewynd`, workflow `release.yml` |
| **GitHub Release** | `GITHUB_TOKEN` (provided automatically) |

If `NPM_TOKEN` is absent the npm step is skipped; if the PyPI trusted publisher isn't configured
the PyPI step fails — the GitHub Release still happens.

## Re-publishing for an existing tag

If you tagged a release before the credentials were in place, you do **not** need to re-tag.
Configure them, then go to **Actions → Release → Run workflow** and enter the existing tag
(e.g. `v0.2.1`). The manual run rebuilds the binaries locally (it does not re-publish the
existing GitHub Release) and pushes the npm + PyPI packages (the npm step skips versions already
on the registry).

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
