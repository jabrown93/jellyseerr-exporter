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
// Runtime dependency bumps (chore(deps), from Renovate via the shared
// preset) do NOT cut a release per merged PR. Dev/test/CI dependency bumps
// (chore(dev-deps)) never release at all. The weekly scheduled run in
// .github/workflows/release.yml sets RELEASE_DEPS=true, which promotes the
// accumulated chore(deps) bumps into one patch release. fix releases
// immediately, as does fix(security) for vulnerability fixes. See
// jabrown93/.github's README, "Weekly dependency releases".

const releaseDeps = process.env.RELEASE_DEPS === "true";

const depReleaseRules = [
  // Required: commit-analyzer evaluates every matching custom rule and keeps
  // the highest release type, so without this a breaking chore(deps)! would
  // match ONLY the suppression rule below and never release. Listed first so
  // the analyzer short-circuits on major.
  { type: "chore", scope: "deps", breaking: true, release: "major" },
  { type: "chore", scope: "deps", release: releaseDeps ? "patch" : false },
];

module.exports = {
  branches: ["main"],
  tagFormat: "v${version}",
  plugins: [
    ["@semantic-release/commit-analyzer", { releaseRules: depReleaseRules }],
    "@semantic-release/release-notes-generator",
    [
      "@semantic-release/github",
      {
        // No release/failure comments or labels: forked history references
        // upstream PR/issue numbers that do not exist in this repo, and the
        // plugin's success step hard-fails (404) trying to comment on them --
        // after the tag and GitHub Release were already created, leaving a
        // tagged-but-imageless release. Comments are noise here anyway.
        successComment: false,
        failComment: false,
        failTitle: false,
        releasedLabels: false,
      },
    ],
  ],
};
