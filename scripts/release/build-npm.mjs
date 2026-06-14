#!/usr/bin/env node
// Turn goreleaser's built binaries (dist/artifacts.json) into per-platform npm packages
// (@rewynd/cli-<platform>-<arch>) and stamp the version on the `rewynd` + `@rewynd/shim` packages.
// Run after `goreleaser build`/`release`. Usage: node scripts/release/build-npm.mjs <version>

import fs from "node:fs";
import path from "node:path";

const raw = process.argv[2] ?? process.env.VERSION;
if (!raw) {
  console.error("usage: build-npm.mjs <version>");
  process.exit(1);
}
const version = raw.replace(/^v/, "");

const root = path.resolve(import.meta.dirname, "..", "..");
const artifacts = JSON.parse(fs.readFileSync(path.join(root, "dist", "artifacts.json"), "utf8"));

const OS = { linux: "linux", darwin: "darwin", windows: "win32" };
const ARCH = { amd64: "x64", arm64: "arm64" };

const optional = {};
for (const a of artifacts) {
  if (a.type !== "Binary") continue;
  const nos = OS[a.goos];
  const narch = ARCH[a.goarch];
  if (!nos || !narch) continue;

  const name = `@rewynd/cli-${nos}-${narch}`;
  const outDir = path.join(root, "packages", "cli-platforms", `${nos}-${narch}`);
  const exe = a.goos === "windows" ? "rewynd.exe" : "rewynd";
  fs.mkdirSync(path.join(outDir, "bin"), { recursive: true });
  fs.copyFileSync(path.join(root, a.path), path.join(outDir, "bin", exe));
  fs.chmodSync(path.join(outDir, "bin", exe), 0o755);
  fs.writeFileSync(
    path.join(outDir, "package.json"),
    JSON.stringify(
      {
        name,
        version,
        description: `rewynd CLI binary for ${nos}-${narch}.`,
        license: "MIT",
        repository: { type: "git", url: "git+https://github.com/SrinjoyDev/rewynd.git" },
        os: [nos],
        cpu: [narch],
        files: ["bin/"],
      },
      null,
      2,
    ) + "\n",
  );
  optional[name] = version;
}

function stamp(pkgRel, mutate) {
  const p = path.join(root, pkgRel);
  const json = JSON.parse(fs.readFileSync(p, "utf8"));
  json.version = version;
  mutate?.(json);
  fs.writeFileSync(p, JSON.stringify(json, null, 2) + "\n");
}

stamp("packages/cli/package.json", (j) => {
  j.optionalDependencies = optional;
  // Convert the workspace: protocol to a real version so `npm publish` accepts it.
  if (j.dependencies?.["@rewynd/shim"]) j.dependencies["@rewynd/shim"] = version;
});
stamp("packages/shim-node/package.json");

console.log(`generated ${Object.keys(optional).length} platform packages @ ${version}`);
