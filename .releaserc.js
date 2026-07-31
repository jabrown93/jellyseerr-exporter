// semantic-release configuration.
//
// Versioning is automated from Conventional Commits:
//   * push to `main` -> stable release (feat -> minor, fix/perf -> patch, ! -> major)
//
// Explicit plugin list on purpose: the default set includes
// @semantic-release/npm, which hard-fails (ENOPKG) in this package.json-less
// Go repo. Nothing in-repo carries the version (the image tag does), so there
// is also no changelog/git plugin and no version-bump commit.
//
// Routine dependency bumps (fix(deps), from Renovate via the shared preset)
// do NOT cut a release per merged PR. The weekly scheduled run in
// .github/workflows/release.yml sets RELEASE_DEPS=true, which promotes the
// accumulated bumps into one patch release. Vulnerability fixes are typed
// fix(security), so they still release immediately. See jabrown93/.github's
// README, "Weekly dependency releases".

const releaseDeps = process.env.RELEASE_DEPS === "true";

const depReleaseRules = [
  // Required: commit-analyzer evaluates every matching custom rule and keeps
  // the highest release type, so without this a breaking fix(deps)! would
  // match ONLY the suppression rule below and never release. Listed first so
  // the analyzer short-circuits on major.
  { type: "fix", scope: "deps", breaking: true, release: "major" },
  releaseDeps
    ? { type: "fix", scope: "deps", release: "patch" }
    : { type: "fix", scope: "deps", release: false },
];

module.exports = {
  branches: ["main"],
  tagFormat: "v${version}",
  plugins: [
    ["@semantic-release/commit-analyzer", { releaseRules: depReleaseRules }],
    "@semantic-release/release-notes-generator",
    "@semantic-release/github",
  ],
};
