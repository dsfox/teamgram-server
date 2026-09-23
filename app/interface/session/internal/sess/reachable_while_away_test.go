package sess

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/app/interface/session/internal/dao"
	statusclient "github.com/teamgram/teamgram-server/app/service/status/client"
	"github.com/teamgram/teamgram-server/app/service/status/status"
)

// A phone that says it went to the background keeps its connection, and the
// record the status service holds for it answers two questions: is the app
// on screen (its expiry, read by the notification path) and can it be
// reached over that connection (its presence, read by every delivery and by
// the phone that rings). Saying "away" must answer the first with no and
// leave the second yes. It used to delete the record, so a call to an Android
// in the background rang nobody: its push only wakes a network that is
// already up (#186).
func TestAPhoneThatSaidItIsAwayStaysReachable(t *testing.T) {
	record := &recordingStatusClient{}
	manager := NewMainAuthWrapperManager(&dao.Dao{StatusClient: record})
	wrapper := NewMainAuthWrapper(1001, 2002, mtproto.AuthStateNormal, nil, 0, manager)
	defer wrapper.Stop()
	ctx := context.Background()

	wrapper.setOnline(ctx)
	if !record.lastOnScreen(t) {
		t.Fatal("a connected phone on screen is not recorded as on screen")
	}

	wrapper.setOfflineNow(ctx)
	if record.removed() {
		t.Fatal("saying it went away took the phone out of reach: nothing can be delivered over its live connection")
	}
	if record.lastOnScreen(t) {
		t.Fatal("a phone that said it went away is still counted as on screen, so nobody would notify it")
	}

	// Its pings go on. They keep it reachable and do not bring it back on
	// screen - and a later one refreshes the record before it lapses.
	wrapper.setOnline(ctx)
	wrapper.awayPublished -= awayRefresh + 1
	written := record.count()
	wrapper.setOnline(ctx)
	if record.count() == written {
		t.Fatal("a ping after the refresh interval wrote nothing: the record would lapse while the connection lives")
	}
	if record.removed() || record.lastOnScreen(t) {
		t.Fatal("a ping of a phone that is away changed what the record says")
	}

	wrapper.setOnlineNow(ctx)
	if !record.lastOnScreen(t) {
		t.Fatal("a phone back on screen is not recorded as on screen")
	}
}

var _ statusclient.StatusClient = (*recordingStatusClient)(nil)

// recordingStatusClient keeps every record the session writes.
type recordingStatusClient struct {
	statusclient.StatusClient
	mu      sync.Mutex
	online  []*status.SessionEntry
	offline int
}

func (r *recordingStatusClient) StatusSetSessionOnline(_ context.Context, in *status.TLStatusSetSessionOnline) (*mtproto.Bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.online = append(r.online, in.GetSession())
	return mtproto.BoolTrue, nil
}

func (r *recordingStatusClient) StatusSetSessionOffline(_ context.Context, _ *status.TLStatusSetSessionOffline) (*mtproto.Bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.offline++
	return mtproto.BoolTrue, nil
}

func (r *recordingStatusClient) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.online)
}

func (r *recordingStatusClient) removed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.offline > 0
}

// lastOnScreen is what sync reads: an expiry still ahead means the app is open.
func (r *recordingStatusClient) lastOnScreen(t *testing.T) bool {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.online) == 0 {
		t.Fatal("no record was written at all")
	}
	return r.online[len(r.online)-1].GetExpired() > time.Now().Unix()
}
