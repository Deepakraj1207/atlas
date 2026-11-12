// Package analysis turns the saved software list into plain-language
// findings (Smart Analysis) and a map of which runtimes other software
// needs (Dependencies). It is read-only.
//
// Facts come from three places, and each finding says which:
//   - the saved software list and update check;
//   - package managers' own dependency records (dpkg, RPM, Chocolatey, Scoop);
//   - on Windows, Windows' own record of when programs were last opened.
//
// Nothing is inferred from missing data. If Windows has no record of a
// program being opened, ATLAS says nothing about it rather than calling it unused.
package analysis
