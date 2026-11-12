// Package cleanup removes software and clears caches, through each package
// manager's own command, in the user's terminal (see internal/handover).
//
// Safety rules, enforced in PlanRemoval before anything is shown:
//   - Dependency records are read again, fresh, for every plan. Anything that
//     other installed software still needs is blocked, naming what needs it.
//   - Packages the system marks essential or required, and tools ATLAS or the
//     system relies on (sudo, apt, winget, …), are never removed.
//   - Runtimes and system components whose users can't be determined are
//     never removed.
//   - Software known to more than one package manager is left to the user,
//     so ATLAS can't pick the wrong one.
//   - No "assume yes", purge or delete-data flags are passed. Package managers
//     ask their own questions and users' settings and data are kept.
//
// Clean-ups only cover things whose exact contents ATLAS can list first:
// package caches, packages APT or DNF itself marks unneeded, old snap
// revisions, and old Scoop versions and downloads.
package cleanup
