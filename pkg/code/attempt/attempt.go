// Package attempt describes one try at a sign-in code.
//
// It exists as a package of its own so that both the verifiers and whatever
// calls them can name the same thing without importing each other. What it
// carries beyond the code itself is the number being opened and whether that
// number already has an account - without those two, a verifier can only answer
// "is this code good", and the code being good is not the question. The
// question is whether it is good for this account.
package attempt

// Attempt is a sign-in code being checked.
type Attempt struct {
	// CodeHash identifies the transaction: the request for a code, the code
	// generated for it, and the tries spent against it.
	CodeHash string

	// Code is what the person typed.
	Code string

	// Generated is the code the server made for this attempt and delivered to
	// whatever sessions the account already has. Empty when there was nowhere
	// to deliver it.
	Generated string

	// PhoneNumber is the number being opened.
	PhoneNumber string

	// PhoneRegistered says whether that number already has an account. An
	// invitation that names no number must not open one that does: otherwise
	// any invitation is a key to every account whose number is known.
	PhoneRegistered bool
}
