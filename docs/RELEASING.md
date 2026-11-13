# Releasing ATLAS

A release is a version tag, a GitHub release with every download, and (for Windows) a
winget package. Nothing is published without a person doing it.

## 1. Check the branch

The **windows** and **release** workflows must be green on the commit you release. Every
change to `packaging/` or `tools/release/` runs **release** as a dry run (version
`0.0.0-dev.<commit>`): it builds all files and checks them, but publishes nothing.

To build the same files on your own machine (Linux, with Go 1.27.4, `zip`, `dpkg-deb`
and `rpmbuild`):

```bash
packaging/build-release.sh 0.1.0 dist
```

## 2. Tag the version

```bash
git tag -a v0.1.0 -m "ATLAS 0.1.0"
```

```bash
git push origin v0.1.0
```

The **release** workflow then:

1. runs the tests on Ubuntu and Windows;
2. builds `atlas_<version>_windows_{amd64,arm64}.zip`, `atlas_<version>_linux_{amd64,arm64}.tar.gz`,
   `atlas-lifecycle_<version>_{amd64,arm64}.deb`, `atlas-lifecycle-<version>-1.{x86_64,aarch64}.rpm`,
   `SHA256SUMS` and `winget-manifests.zip`;
3. checks them: the Windows zip runs, reports the version and passes the self-test, and
   `winget validate` accepts the manifests; the checksums match; the `.deb` installs,
   runs and removes on Ubuntu, and the `.rpm` on Fedora;
4. creates a **draft** release with the notes in `packaging/release-notes.md`.

## 3. Publish the release

Open the draft on GitHub (Releases), read it, and press **Publish release**.

Publishing starts **winget-check**: on a clean Windows machine it installs ATLAS from
the release's winget manifests, exactly as winget users will get it, runs `atlas
-version` and the self-test, and uninstalls it. Wait for it to pass.

## 4. Submit to winget

winget packages live in [microsoft/winget-pkgs](https://github.com/microsoft/winget-pkgs).
Submitting is a pull request there, under your GitHub account; Microsoft's checks and
reviewers then validate it, which usually takes a few days.

1. Download `winget-manifests.zip` from the release and unzip it. It contains
   `manifests/d/Deepakraj1207/ATLAS/<version>/` with three files.
2. Fork microsoft/winget-pkgs, copy that folder to the same path in your fork, commit
   and open a pull request. (Alternatively, Microsoft's `wingetcreate submit` tool does
   the fork and pull request for you with a GitHub token.)
3. Answer any questions from the validation bot or reviewers.

Once merged, `winget install Deepakraj1207.ATLAS` (or `winget install --moniker atlas`)
works for everyone. For later versions, repeat with the new folder; the identifier never
changes.

## Notes

- **Identifier.** `Deepakraj1207.ATLAS` is permanent once accepted. winget requires the
  `Publisher.Name` form, so a bare `atlas` isn't possible; the moniker `atlas` covers
  that. Other packages also match the word "atlas" in searches, so `winget install
  atlas` may list several; `--moniker atlas` is exact.
- **Command clash.** The MongoDB Atlas CLI also installs a command called `atlas`. With
  both installed, whichever comes first on PATH wins.
- **Linux package name.** `atlas-lifecycle`, because Fedora already ships a package
  called `atlas` (a maths library). The command is still `atlas`.
- **Code signing.** Windows builds aren't signed yet, so SmartScreen may warn about a
  downloaded `atlas.exe`. Signing needs a certificate or an Azure Trusted Signing account
  in your name; installs through winget aren't affected.
- **Catalogue.** Before a release, run the **catalog-check** workflow (Actions → catalog-check →
  Run workflow) to confirm every Discover Apps package ID still exists.
