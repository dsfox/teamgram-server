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
		// ice9: transport key not initialised yet: the client restarts the handshake
		return mtproto.ErrAuthRestart
	default:
		// ice9: Unauthorized and terminal states get AUTH_KEY_UNREGISTERED, so the client signs out (#179)
		return mtproto.ErrAuthKeyUnregistered
	}
}
