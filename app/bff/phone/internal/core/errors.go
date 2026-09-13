package core

import (
	"errors"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/pkg/calls"
)

// rpcError turns what the registry and the state machine refuse into the
// error a phone knows, all of them 400: a 500 reads as "try again" on the
// phone, and a call that is gone was asked for ten times over before this
// existed. wrongMoment is what ErrWrongState means at the step that asks -
// already accepted for an answer, already declined for a hang-up. A server's
// own trouble stays what it is.
func rpcError(err error, wrongMoment error) error {
	switch {
	case errors.Is(err, calls.ErrNoCall), errors.Is(err, calls.ErrWrongParty), errors.Is(err, calls.ErrSelfCall):
		return mtproto.ErrCallPeerInvalid
	case errors.Is(err, calls.ErrBusy):
		return mtproto.ErrCallOccupyFailed
	case errors.Is(err, calls.ErrWrongState):
		return wrongMoment
	}
	return err
}
