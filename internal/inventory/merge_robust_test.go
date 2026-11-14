package inventory

import (
	"testing"

	"github.com/deepakraj1207/atlas/internal/core"
)

func reg(name, version, publisher string) core.SoftwareItem {
	return core.SoftwareItem{Name: name, Version: version, Publisher: publisher, Source: SourceRegistry, Kind: core.KindApp}
}

func wingetItem(name, version, id string) core.SoftwareItem {
	return core.SoftwareItem{Name: name, Version: version, PackageID: id, Source: SourceWinget, Kind: core.KindApp, Origin: "winget"}
}

func count(items []core.SoftwareItem) int { return len(items) }

// The audit's P1 scenarios. Merging must stay conservative: when in doubt,
// two entries stay two entries.
func TestMergeDoesNotJoinDifferentSoftware(t *testing.T) {
	for _, c := range []struct {
		name  string
		items []core.SoftwareItem
		want  int
	}{
		{"same name, different publishers", []core.SoftwareItem{
			reg("Remote Desktop Manager", "2024.1", "Devolutions inc."), reg("Remote Desktop Manager", "2024.1", "Another Company Ltd")}, 2},
		{"different editions", []core.SoftwareItem{
			reg("Visual Studio Community 2022", "17.10.1", "Microsoft Corporation"),
			wingetItem("Visual Studio Professional 2022", "17.10.1", "Microsoft.VisualStudio.2022.Professional")}, 2},
		{"different editions in parentheses", []core.SoftwareItem{
			reg("Python 3.12.1 (32-bit)", "3.12.1", "Python Software Foundation"),
			reg("Python 3.12.1 (64-bit)", "3.12.1", "Python Software Foundation"),
			{Name: "Python 3.x", Version: "3.12.1", PackageID: "python", Source: SourceChocolatey, Kind: core.KindApp}}, 3},
		{"portable copy beside an installed one (same version)", []core.SoftwareItem{
			reg("jq", "1.7.1", "jqlang"), {Name: "jq", Version: "1.7.1", PackageID: "main/jq", Source: SourceScoop, Kind: core.KindApp}}, 2},
		{"different versions side by side", []core.SoftwareItem{
			reg("Zoom", "5.17.1", "Zoom Video Communications"), reg("Zoom", "6.0.2", "Zoom Video Communications")}, 2},
		{"localized installs", []core.SoftwareItem{
			reg("Microsoft Office Professionnel Plus 2021 - fr-fr", "16.0.17", "Microsoft Corporation"),
			reg("Microsoft Office Professional Plus 2021 - en-us", "16.0.17", "Microsoft Corporation")}, 2},
		{"same source, different package IDs", []core.SoftwareItem{
			wingetItem("Zoom", "6.0.2", "Zoom.Zoom"), wingetItem("Zoom", "6.0.2", "Zoom.ZoomRooms")}, 2},
		{"Linux: the deb and the Snap are separate copies", []core.SoftwareItem{
			{Name: "firefox", Version: "131.0", Source: SourceDpkg, Kind: core.KindApp},
			{Name: "firefox", Version: "131.0", Source: SourceSnap, Kind: core.KindApp}}, 2},
		{"Linux: the rpm and the Flatpak are separate copies", []core.SoftwareItem{
			{Name: "gimp", Version: "2.10.38", Source: SourceRPM, Kind: core.KindApp},
			{Name: "GIMP", Version: "2.10.38", Source: SourceFlatpak, Kind: core.KindApp}}, 2},
		{"unversioned Scoop app beside two installed versions", []core.SoftwareItem{
			reg("Python", "3.11.9", ""), reg("Python", "3.12.3", ""), {Name: "Python", PackageID: "main/python", Source: SourceScoop, Kind: core.KindApp}}, 3},
		{"installer record fitting both editions", []core.SoftwareItem{
			reg("Python 3.12.1 (32-bit)", "3.12.1", "Python Software Foundation"),
			reg("Python 3.12.1 (64-bit)", "3.12.1", "Python Software Foundation"),
			{Name: "Python", Version: "3.12.1", PackageID: "python312", Source: SourceChocolatey, Kind: core.KindApp}}, 3},
		{"truncated winget name before the Windows entries", []core.SoftwareItem{
			wingetItem("Microsoft Visual C++ 2015-2022 Redistributable…", "14.38.33135.0", "Microsoft.VCRedist.2015+.x64"),
			reg("Microsoft Visual C++ 2015-2022 Redistributable (x86) - 14.38.33135", "14.38.33135.0", "Microsoft Corporation"),
			reg("Microsoft Visual C++ 2015-2022 Redistributable (x64) - 14.38.33135", "14.38.33135.0", "Microsoft Corporation")}, 3},
		{"truncated winget name matching two architectures", []core.SoftwareItem{
			reg("Microsoft Visual C++ 2015-2022 Redistributable (x64) - 14.38.33135", "14.38.33135.0", "Microsoft Corporation"),
			reg("Microsoft Visual C++ 2015-2022 Redistributable (x86) - 14.38.33135", "14.38.33135.0", "Microsoft Corporation"),
			wingetItem("Microsoft Visual C++ 2015-2022 Redistributable…", "14.38.33135.0", "Microsoft.VCRedist.2015+.x64")}, 3},
	} {
		if got := count(Merge(c.items)); got != c.want {
			t.Errorf("%s: %d entries after merging, want %d: %+v", c.name, got, c.want, Merge(c.items))
		}
	}
}

func TestMergeStillJoinsTheSameInstall(t *testing.T) {
	for _, c := range []struct {
		name  string
		items []core.SoftwareItem
	}{
		{"winget reads the Windows entry", []core.SoftwareItem{reg("Git", "2.56.0", "The Git Development Community"), wingetItem("Git", "2.56.0", "Git.Git")}},
		{"truncated winget name, one candidate", []core.SoftwareItem{
			reg("Microsoft Visual C++ 2015-2022 Redistributable (x64) - 14.38.33135", "14.38.33135.0", "Microsoft Corporation"),
			wingetItem("Microsoft Visual C++ 2015-2022 Redistributable…", "14.38.33135.0", "Microsoft.VCRedist.2015+.x64")}},
		{"Chocolatey record of the same install", []core.SoftwareItem{
			reg("7-Zip 26.05 (x64)", "26.05", "Igor Pavlov"), {Name: "7-Zip (Install)", Version: "26.5.0", PackageID: "7zip.install", Source: SourceChocolatey, Kind: core.KindApp}}},
		{"localized name, same install (winget shows the same string)", []core.SoftwareItem{
			reg("Mozilla Firefox (x64 de)", "131.0", "Mozilla"), wingetItem("Mozilla Firefox (x64 de)", "131.0", "Mozilla.Firefox.de")}},
		{"publisher written two ways", []core.SoftwareItem{
			reg("Remote Desktop Manager", "2024.1", "Devolutions inc."), reg("Remote Desktop Manager", "2024.1", "Devolutions Inc")}},
		{"the same Windows entry seen twice", []core.SoftwareItem{reg("Git", "2.56.0", "The Git Development Community"), reg("Git", "2.56.0", "The Git Development Community")}},
		{"same name and publisher, case and spacing differ", []core.SoftwareItem{
			reg("Notepad++  (64-bit x64)", "8.6", "Notepad++ Team"), wingetItem("notepad++ (64-bit x64)", "8.6", "Notepad++.Notepad++")}},
	} {
		if got := Merge(c.items); len(got) != 1 {
			t.Errorf("%s: want one entry, got %d: %+v", c.name, len(got), got)
		}
	}
}

// A Windows entry without a version is still the primary record when winget
// reports the same program with one: removal keeps using Windows' uninstaller.
func TestWeakMatchKeepsTheHigherPrioritySource(t *testing.T) {
	out := Merge([]core.SoftwareItem{reg("Contoso Tool", "", "Contoso"), wingetItem("Contoso Tool", "4.2", "Contoso.Tool")})
	if len(out) != 1 {
		t.Fatalf("want one entry, got %+v", out)
	}
	if got := out[0]; got.Source != SourceRegistry || got.Version != "4.2" || got.PackageID != "Contoso.Tool" || len(got.AlsoFoundIn) != 1 {
		t.Errorf("want the Windows entry, filled in from winget: %+v", got)
	}
}

func TestNormPublisher(t *testing.T) {
	for in, want := range map[string]string{"Devolutions inc.": "devolutions", "Microsoft Corporation": "microsoft",
		"JetBrains s.r.o.": "jetbrains s r o", "Igor Pavlov": "igor pavlov", "Co": "co", "": ""} {
		if got := normPublisher(in); got != want {
			t.Errorf("normPublisher(%q) = %q, want %q", in, got, want)
		}
	}
}
