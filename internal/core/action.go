package core

// ActionPlan describes a consequential action before it runs. Every action
// ATLAS offers must be expressible as a plan so the UI can always tell the
// user what will happen, why, what changes, whether it is safe, and whether
// it can be undone. Phase 1 renders plans; it never executes them.
type ActionPlan struct {
	// Title is the plain-language name of the action, e.g. "Scan My System".
	Title string
	// WhatHappens explains the action in one or two sentences.
	WhatHappens string
	// Why explains the benefit to the user.
	Why string
	// WhatChanges lists concrete effects on the system. Empty means nothing changes.
	WhatChanges []string
	// Safety is a plain-language safety statement.
	Safety string
	// Reversible states whether and how the action can be undone.
	Reversible string
	// Technical holds optional detail (package IDs, commands) shown only in
	// Advanced mode.
	Technical []string
	// Available is false when the action is not yet implemented. The UI must
	// say so and must not pretend to run it.
	Available bool
	// Unavailability explains when the action arrives, if Available is false.
	Unavailability string
	// Destructive plans delete something. They need a deliberate key (r)
	// rather than Enter to confirm, so muscle memory can't confirm them.
	Destructive bool
}

// ChangesSystem reports whether the plan modifies anything on the machine.
func (p ActionPlan) ChangesSystem() bool {
	return len(p.WhatChanges) > 0
}
