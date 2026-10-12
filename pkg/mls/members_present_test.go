package mls

import "testing"

// Who is still behind a conversation's leaves, which decides whether a claim
// on its chat is told to wait (#147, #247). The rows are the ones each case
// had on the server; the clock is `at`, and "seen" holds only devices that are
// answering and signed in, as signedInSeen reads them.
const at int32 = 10_000_000

type presentCase struct {
	held   []memberRow
	behind map[string]deviceKey
	seen   map[deviceKey]int32
	named  map[deviceKey]bool
	missed map[deviceKey]int32
}

func leaf(name string, user int64) memberRow {
	return memberRow{Leaf: []byte(name), UserId: user, JoinedAt: at - 86400}
}

func (c presentCase) count() presence {
	return presentLeaves(c.held, c.behind, c.seen, c.named, c.missed)
}

var (
	alicePhone = deviceKey{user: 1, key: 11}
	aliceNew   = deviceKey{user: 1, key: 12}
	bobPhone   = deviceKey{user: 2, key: 21}
	bobTablet  = deviceKey{user: 2, key: 22}
	bobUnnamed = deviceKey{user: 2, key: 23}
	gammaPhone = deviceKey{user: 3, key: 31}
	alphaNew   = deviceKey{user: 4, key: 42}
	alphaWiped = deviceKey{user: 4, key: 41}
	deltaWiped = deviceKey{user: 5, key: 51}
	deltaNew   = deviceKey{user: 5, key: 52}
	everyNamed = map[deviceKey]bool{alicePhone: true, aliceNew: true, bobPhone: true, bobTablet: true,
		gammaPhone: true, alphaNew: true, alphaWiped: true, deltaWiped: true, deltaNew: true}
)

// The incident of #247: gamma's phone is in the conversation and answering,
// alpha's new phone is not. The chat must hold, so that gamma lets alpha in.
func TestALivePhoneInTheConversationHoldsTheChat(t *testing.T) {
	got := presentCase{
		held:   []memberRow{leaf("3/1", 3), leaf("4/1", 4)},
		behind: map[string]deviceKey{"3/1": gammaPhone, "4/1": alphaWiped},
		seen:   map[deviceKey]int32{gammaPhone: at, alphaWiped: at - 86400, alphaNew: at},
		named:  everyNamed,
	}.count()
	if got.present != 1 || got.replaced != 1 {
		t.Fatalf("gamma present and alpha's wiped phone replaced, got %+v", got)
	}
}

// 833b on 12 October: every leaf a phone wiped since, both current phones
// outside. Nobody is behind any of it, so the chat is let go.
func TestAConversationOfPhonesAllSetUpAgainHoldsNobody(t *testing.T) {
	got := presentCase{
		held:   []memberRow{leaf("4/1", 4), leaf("5/1", 5), leaf("5/2", 5)},
		behind: map[string]deviceKey{"4/1": alphaWiped, "5/1": deltaWiped, "5/2": {user: 5, key: 50}},
		// The second of delta's leaves is past the fortnight: not seen at all.
		seen:  map[deviceKey]int32{alphaWiped: at - 2*86400, deltaWiped: at - 7*86400, alphaNew: at, deltaNew: at},
		named: everyNamed,
	}.count()
	if got.present+got.unaccounted != 0 || got.replaced != 2 || got.gone != 1 {
		t.Fatalf("two replaced and one gone, nobody present, got %+v", got)
	}
}

// A person whose only phone is in the conversation and has been off for days
// is not replaced by anything: the chat holds while the other side set up
// again, and that phone lets the new one in when it comes back.
func TestAnOnlyPhoneOffForDaysStillHoldsTheChat(t *testing.T) {
	got := presentCase{
		held:   []memberRow{leaf("1/1", 1), leaf("2/1", 2)},
		behind: map[string]deviceKey{"1/1": alicePhone, "2/1": bobPhone},
		seen:   map[deviceKey]int32{alicePhone: at - 3*86400, aliceNew: at, bobPhone: at - 5*86400},
		named:  everyNamed,
	}.count()
	if got.present != 1 {
		t.Fatalf("bob's only phone is present, got %+v", got)
	}
}

// Bob's phone in the conversation has been quiet for three hours while his new
// tablet, not let in yet, is in use: the phone counts as replaced. Alice's
// phone is present, so the chat holds and alice lets the tablet in.
func TestOneSideReplacedHoldsWhileTheOtherIsPresent(t *testing.T) {
	got := presentCase{
		held:   []memberRow{leaf("1/1", 1), leaf("2/1", 2)},
		behind: map[string]deviceKey{"1/1": alicePhone, "2/1": bobPhone},
		seen:   map[deviceKey]int32{alicePhone: at, bobPhone: at - 3*3600, bobTablet: at},
		named:  everyNamed,
	}.count()
	if got.present != 1 || got.replaced != 1 {
		t.Fatalf("alice present and bob's phone replaced, got %+v", got)
	}
}

// Two phones of one person both in use, the one outside seen a few minutes
// later than the one inside: a round apart is not a replacement.
func TestTwoPhonesInUseAreNotAReplacement(t *testing.T) {
	got := presentCase{
		held:   []memberRow{leaf("2/1", 2)},
		behind: map[string]deviceKey{"2/1": bobPhone},
		seen:   map[deviceKey]int32{bobPhone: at - 600, bobTablet: at},
		named:  everyNamed,
	}.count()
	if got.present != 1 {
		t.Fatalf("bob's phone is present ten minutes behind his tablet, got %+v", got)
	}
}

// A leaf from a build before key packages carried names matches no device.
// It is held while its owner has an answering phone that publishes no name -
// the phone that could be behind it - and not once nobody could be.
func TestALeafNobodyCanMatchIsHeldOnlyForAPhoneThatCouldBeBehindIt(t *testing.T) {
	unnamed := map[deviceKey]bool{alicePhone: true}
	held := presentCase{
		held:  []memberRow{leaf("2/1", 2)},
		seen:  map[deviceKey]int32{bobUnnamed: at},
		named: unnamed,
	}.count()
	if held.unaccounted != 1 {
		t.Fatalf("held for bob's unnamed phone, got %+v", held)
	}
	nobody := presentCase{
		held: []memberRow{leaf("7000000001/1", 7_000_000_001)},
	}.count()
	if nobody.present+nobody.unaccounted != 0 || nobody.gone != 1 {
		t.Fatalf("a person with no devices holds nothing (#147), got %+v", nobody)
	}
	// A phone with a name, outside the conversation, proves the leaf is not
	// its own: only an unnamed phone can be behind an unmatched leaf.
	named := presentCase{
		held:  []memberRow{leaf("2/1", 2)},
		seen:  map[deviceKey]int32{bobTablet: at},
		named: everyNamed,
	}.count()
	if named.present+named.unaccounted != 0 {
		t.Fatalf("bob's named tablet is not behind a leaf of another name, got %+v", named)
	}
}

// A device signed out keeps its row in mls_devices; signedInSeen leaves it out,
// and with it its leaf. A device that missed a commit of this group after its
// leaf joined can never reach the group's epoch: gone too, as Holding says.
func TestSignedOutAndBehindAreGone(t *testing.T) {
	signedOut := presentCase{
		held:   []memberRow{leaf("1/1", 1)},
		behind: map[string]deviceKey{"1/1": alicePhone},
		seen:   map[deviceKey]int32{},
		named:  everyNamed,
	}.count()
	if signedOut.gone != 1 {
		t.Fatalf("a signed-out phone is gone, got %+v", signedOut)
	}
	behind := presentCase{
		held:   []memberRow{leaf("1/1", 1)},
		behind: map[string]deviceKey{"1/1": alicePhone},
		seen:   map[deviceKey]int32{alicePhone: at},
		named:  everyNamed,
		missed: map[deviceKey]int32{alicePhone: at - 60},
	}.count()
	if behind.gone != 1 {
		t.Fatalf("a phone that missed a commit since joining is gone, got %+v", behind)
	}
}
