# Releasing ATLAS

A release is a version tag and a GitHub release with every download. Users install it
with the one-line installers (`install.ps1`, `install.sh`), which always fetch the
latest published release. Nothing is published without a person doing it.

## 1. Check the branch

The **windows** and **release** workflows must be green on the commit you release. Every
change to `packaging/` runs **release** as a dry run (version
`0.0.0-dev.<commit>`): it builds all files and checks them, but publishes nothing.

To build the same files on your own machine (Linux, with Go 1.27.4, `zip`, `dpkg-deb`
and `rpmbuild`):

```bash
packaging/build-release.sh 0.2.0 dist
```

## 2. Tag the version

```bash
git tag -a v0.2.0 -m "ATLAS 0.2.0"
```

```bash
git push origin v0.2.0
```

The **release** workflow then:

1. runs the tests on Ubuntu and Windows;
2. builds `atlas_<version>_windows_{amd64,arm64}.zip`, `atlas_<version>_linux_{amd64,arm64}.tar.gz`,
   `atlas-lifecycle_<version>_{amd64,arm64}.deb`, `atlas-lifecycle-<version>-1.{x86_64,aarch64}.rpm`
   and `SHA256SUMS`;
3. checks them: the Windows zip runs, reports the version and passes the self-test; the
   checksums match; the `.deb` installs, runs and removes on Ubuntu, and the `.rpm` on
   Fedora;
4. creates a **draft** release with the notes in `packaging/release-notes.md`.

The installers rely on these file names (`atlas_<version>_<os>_<arch>.zip/.tar.gz`,
`atlas.exe` at the top of the Windows zip, `atlas_<version>_linux_<arch>/atlas` in the
tarball) and on `SHA256SUMS`. Keep them unchanged.

## 3. Publish the release

Open the draft on GitHub (Releases), read it, and press **Publish release**. From that
moment the one-line installers install it, and running them again updates existing
installs.

Publishing starts **install-check**: on clean Windows (Windows PowerShell 5.1 and
PowerShell 7) and in Ubuntu, Fedora and Alpine it runs the installers as users do, starts
`atlas`, installs again (the update path) and uninstalls, checking that the Windows user
PATH is restored exactly. Wait for it to pass.

## Notes

- **Command clash.** The MongoDB Atlas CLI also installs a command called `atlas`. With
  both installed, whichever comes first on PATH wins; the installers say so when they
  see another one.
- **Linux package name.** `atlas-lifecycle`, because Fedora already ships a package
  called `atlas` (a maths library). The command is still `atlas`.
- **Code signing.** Windows builds aren't signed yet, so SmartScreen may warn about an
  `atlas.exe` downloaded with a browser. Signing needs a certificate or an Azure Trusted
  Signing account in your name.
- **Catalogue.** Before a release, run the **catalog-check** workflow (Actions → catalog-check →
  Run workflow) to confirm every Discover Apps package ID still exists.
