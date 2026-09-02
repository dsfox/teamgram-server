package invite

import "context"

// Recorder keeps the record of who brought whom (#47). The verifier holds one
// so that the moment a code is spent is the moment it is written down - the
// same place that decides two people racing the same code do not both get in.
//
// Nil means no record, which is how the CLI and the tests run.
type Recorder interface {
	Redeemed(ctx context.Context, code, phone string, at int32) error
}
