package calls

import (
	"encoding/json"
	"fmt"
)

// Route is how a call's media travelled, as the phone reports it after the
// call: each leg direct ("p2p") or through the relay ("turn"), and the ICE
// candidate types the selected pair was made of. This is the connectivity
// metric the plan asks for - "connect without a relay" measured, not
// guessed - read from the stats log tgcalls writes and phone.saveCallDebug
// carries.
type Route struct {
	Local, Remote         string // "p2p" or "turn"
	LocalType, RemoteType string // host, srflx, prflx, relay
}

// Relayed says the media went through a relay on either leg.
func (r Route) Relayed() bool {
	return r.Local == "turn" || r.Remote == "turn"
}

func (r Route) String() string {
	return fmt.Sprintf("local %s/%s, remote %s/%s", r.Local, r.LocalType, r.Remote, r.RemoteType)
}

// statsLog is the part of tgcalls' stats log this reads: the network records
// in order, each saying whether the call was connected at that moment and,
// when it was, over what.
type statsLog struct {
	Network []struct {
		Connected int    `json:"c"`
		Local     string `json:"local"`
		Remote    string `json:"remote"`
		Network   *struct {
			Local  struct{ Type string } `json:"local"`
			Remote struct{ Type string } `json:"remote"`
		} `json:"network"`
	} `json:"network"`
}

// RouteOf reads the route the call last connected over. False when the log
// is not the stats shape or the call never connected.
func RouteOf(debugLog []byte) (Route, bool) {
	var log statsLog
	if err := json.Unmarshal(debugLog, &log); err != nil {
		return Route{}, false
	}
	var route Route
	found := false
	for _, record := range log.Network {
		if record.Connected != 1 || record.Local == "" {
			continue
		}
		route = Route{Local: record.Local, Remote: record.Remote}
		if record.Network != nil {
			route.LocalType, route.RemoteType = record.Network.Local.Type, record.Network.Remote.Type
		}
		found = true
	}
	return route, found
}
