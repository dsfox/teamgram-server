package calls

import "testing"

// The stats log a phone sends after a call (tgcalls v2, "network" records
// with the route once it connected): what the connectivity metric reads.
const sampleStatsLog = `{"bitrate":[{"t":"0","b":32}],"network":[
 {"t":"0","c":0},
 {"t":"1","c":1,"local":"p2p","remote":"turn","network":{"local":{"type":"srflx","protocol":"udp","address":"77.37.130.1:55885"},"remote":{"type":"relay","protocol":"udp","address":"5.23.53.210:50987"}}},
 {"t":"20","c":0,"failed":1}
]}`

func TestTheRouteIsReadFromTheStatsLog(t *testing.T) {
	route, ok := RouteOf([]byte(sampleStatsLog))
	if !ok {
		t.Fatal("no route read from a log that has one")
	}
	if route.Local != "p2p" || route.Remote != "turn" || route.LocalType != "srflx" || route.RemoteType != "relay" {
		t.Fatalf("read %+v", route)
	}
	if !route.Relayed() {
		t.Error("a turn leg is a relayed call")
	}
	if route.String() != "local p2p/srflx, remote turn/relay" {
		t.Errorf("says %q", route.String())
	}
}

func TestALogWithoutAConnectionHasNoRoute(t *testing.T) {
	for name, log := range map[string]string{
		"never connected": `{"network":[{"t":"0","c":0},{"t":"9","c":0,"failed":1}]}`,
		"not json":        `NativeNetworkingImpl route changed: ...`,
		"empty":           ``,
	} {
		if route, ok := RouteOf([]byte(log)); ok {
			t.Errorf("%s: read %+v from it", name, route)
		}
	}
}
