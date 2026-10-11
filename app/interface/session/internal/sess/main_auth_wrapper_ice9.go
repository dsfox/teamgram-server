package sess

import (
	"context"
	"time"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/app/service/status/status"

	"github.com/zeromicro/go-zero/core/logx"
)

// What ice9 adds to upstream's MainAuthWrapper in main_auth_wrapper.go, which
// keeps the two fields and a one-line call to each of these (#198).
//
// A backgrounded app keeps its connection and keeps pinging, and every ping
// used to put the session back online. Marking it offline once therefore
// lasted about a second, and a message arriving after that still woke nobody.
// The client's own word outlives its traffic: until it says it is back, it is
// away - and away is still reachable. reportedOffline holds that word, kept
// apart from the expiry clock so traffic cannot silently overrule it.

// awayRefresh is how often, in seconds, an away session writes its record
// again: well inside the ninety seconds the record lives (StatusExpire in
// status.yaml), so a connection that is still up never drops out of reach.
const awayRefresh = 30

// stayReachable keeps an away session in the status list with an expiry that
// has already passed. The two halves of the record answer two questions. Its
// presence says the connection can be reached, and every delivery goes by
// that - a call has to ring on an Android in the background, whose push only
// wakes a network that is already up (#186). Its expiry says whether the app
// is on screen, and the notification path goes by that, so the phone is
// still notified.
func (m *MainAuthWrapper) stayReachable(ctx context.Context) {
	date := time.Now().Unix()
	if m.AuthUserId == 0 || date < m.awayPublished+awayRefresh {
		return
	}
	m.publishStatus(ctx, date)
	m.awayPublished = date
}

// publishStatus writes this session's record: reachable through this server,
// on screen until expired.
func (m *MainAuthWrapper) publishStatus(ctx context.Context, expired int64) {
	if _, err := m.cb.Dao.StatusClient.StatusSetSessionOnline(
		ctx,
		&status.TLStatusSetSessionOnline{
			UserId: m.AuthUserId,
			Session: &status.SessionEntry{
				UserId:        m.AuthUserId,
				AuthKeyId:     m.authKeyId,
				Gateway:       m.cb.Dao.MyServerId,
				Expired:       expired,
				Layer:         m.Layer(),
				PermAuthKeyId: m.authKeyId,
				Client:        m.ClientName(),
			},
		}); err != nil {
		logx.WithContext(ctx).Errorf("status: cannot record session %d of %d: %v", m.authKeyId, m.AuthUserId, err)
	}
}

// setOnlineNow undoes setOfflineNow: the client is back on screen.
func (m *MainAuthWrapper) setOnlineNow(ctx context.Context) {
	if !m.reportedOffline {
		return
	}
	m.reportedOffline = false
	logx.WithContext(ctx).Infof("online on client request: %s", m)
	m.setOnline(ctx)
}

// setOfflineNow takes the client at its word that it went to the background:
// the app is no longer on screen, whatever its pings say, and the record says
// so at once - but the connection is still there, and so is the record
// (stayReachable).
func (m *MainAuthWrapper) setOfflineNow(ctx context.Context) {
	m.reportedOffline = true
	if m.AuthUserId > 0 {
		logx.WithContext(ctx).Infof("offline on client request: %s", m)
	}
	m.onlineExpired = 0
	m.awayPublished = 0
	m.stayReachable(ctx)
}

// resetIsForAnotherKey says an updateAccountResetAuthorization is not for this
// session. The update names the authorisation that was terminated. Matching on
// the user instead of the key deleted every session that user has, including
// the one that asked for the reset: it lost the answer to its own request - the
// server had already computed boolTrue - and then had to be announced a new
// session to carry on. `active_sessions` hung on exactly that.
func (m *MainAuthWrapper) resetIsForAnotherKey(ctx context.Context, upds *mtproto.Updates) bool {
	if upds.GetAuthKeyId() == m.authKeyId {
		return false
	}
	logx.WithContext(ctx).Infof(
		"updateAccountResetAuthorization for key %d, not ours (%d) - ignoring",
		upds.GetAuthKeyId(), m.authKeyId)
	return true
}
