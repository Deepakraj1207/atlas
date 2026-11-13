# Built by packaging/linux/build-rpm.sh from an already compiled binary.
# The binary is already stripped (-s -w) and may be for another
# architecture than the build machine, so skip rpmbuild's strip and
# debug-info steps, which would run the build machine's tools on it.
%global debug_package %{nil}
%global _build_id_links none
%global __os_install_post %{nil}
Name:           atlas-lifecycle
Version:        %{atlas_version}
Release:        %{atlas_release}
Summary:        Understand, update and tidy installed software (ATLAS)
License:        MIT
URL:            https://github.com/Deepakraj1207/atlas
# A static Go binary: no automatic library dependencies to find.
AutoReqProv:    no

%description
ATLAS is a terminal app that shows what is installed on your computer and
helps you keep it tidy and up to date, in plain language. On Linux it reads
dpkg, RPM, Snap and Flatpak, checks for updates, points out runtimes that no
longer get security fixes, and installs or removes software through your own
package managers. Nothing happens without a preview you confirm.

The command is "atlas".

%install
install -D -m 0755 %{atlas_binary} %{buildroot}%{_bindir}/atlas
install -D -m 0644 %{atlas_src}/README.md %{buildroot}%{_docdir}/%{name}/README.md
install -D -m 0644 %{atlas_src}/LICENSE %{buildroot}%{_licensedir}/%{name}/LICENSE

%files
%{_bindir}/atlas
%doc %{_docdir}/%{name}/README.md
%license %{_licensedir}/%{name}/LICENSE
