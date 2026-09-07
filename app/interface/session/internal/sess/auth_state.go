package sess

import "github.com/teamgram/proto/mtproto"

func shouldRefreshAuthState(state int) bool {
	switch state {
	case mtproto.AuthStateNew,
		mtproto.AuthStateWaitInit,
		mtproto.AuthStateUnauthorized:
		return true
	default:
		return false
	}
}

func rpcErrorForAuthState(state int) error {
	switch state {
	case mtproto.AuthStateNew,
		mtproto.AuthStateWaitInit:
		// The transport key is not initialised yet: tell the client to restart
		// the handshake, which it can do without losing its account.
		return mtproto.ErrAuthRestart
	default:
		// AuthStateUnauthorized and the terminal states: the key has a session
		// but no user behind it - the account was deleted or the authorization
		// revoked. AUTH_RESTART here made the client loop the handshake for
		// ever (the A15 hung on the number screen while still making authorized
		// calls, ice9 #179). AUTH_KEY_UNREGISTERED tells it to sign out, which
		// both clients do. The state was refreshed against the authsession
		// service just before this, so a key whose account still exists is
		// already Normal and never reaches here.
		return mtproto.ErrAuthKeyUnregistered
	}
}
