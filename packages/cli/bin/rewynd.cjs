#!/usr/bin/env node
"use strict";

// Resolve the prebuilt Go binary for this platform (shipped via per-platform optionalDependencies,
// the esbuild/biome model) and hand off to it. No binary in the package itself, no postinstall.
const path = require("node:path");
const { spawnSync } = require("node:child_process");

const pkg = `@rewynd/cli-${process.platform}-${process.arch}`;
const exe = process.platform === "win32" ? "rewynd.exe" : "rewynd";

let bin;
try {
  // Resolve the package.json (a real module path), then locate the binary beside it.
  bin = path.join(path.dirname(require.resolve(`${pkg}/package.json`)), "bin", exe);
} catch {
  console.error(
    `rewynd: no prebuilt binary for ${process.platform}-${process.arch}.\n` +
      "Download a release from https://github.com/SrinjoyDev/rewynd/releases, or run\n" +
      "`go install github.com/SrinjoyDev/rewynd/core/cmd/rewynd@latest`.",
  );
  process.exit(1);
}

const res = spawnSync(bin, process.argv.slice(2), { stdio: "inherit" });
if (res.error) {
  console.error(res.error.message);
  process.exit(1);
}
process.exit(res.status === null ? 1 : res.status);
