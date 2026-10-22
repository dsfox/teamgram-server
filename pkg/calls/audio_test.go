package calls

import (
	"strings"
	"testing"
)

func TestAnOlderBuildSaysItsBitrateAndNothingMore(t *testing.T) {
	a, ok := AudioOf([]byte(sampleStatsLog))
	if !ok {
		t.Fatal("a stats log was not read")
	}
	if a.BitrateSamples != 1 || a.BitrateMax != 32 || a.Ice9 != nil {
		t.Fatalf("read %+v", a)
	}
	if !strings.Contains(a.String(), "no ice9 summary") {
		t.Errorf("says %q", a.String())
	}
}

func TestTheSummaryOfANewerBuildIsWrittenOut(t *testing.T) {
	log := `{"bitrate":[{"b":0},{"b":40},{"b":0}],"network":[],
	 "ice9":{"build":"13.0.2","incoming":true,"answer_visible":false,"silenced":"no","mic_max":0,"mic_heard":0,"peer_max":0.4,"peer_heard":12}}`
	a, ok := AudioOf([]byte(log))
	if !ok {
		t.Fatal("not read")
	}
	if a.BitrateSamples != 3 || a.BitrateMax != 40 || a.BitrateZero != 2 {
		t.Fatalf("bitrate read as %+v", a)
	}
	line := a.String()
	for _, want := range []string{"build=13.0.2", "answer_visible=false", "silenced=no", "mic_heard=0", "peer_heard=12"} {
		if !strings.Contains(line, want) {
			t.Errorf("%q is missing %q", line, want)
		}
	}
}

func TestSomethingElseIsNotAStatsLog(t *testing.T) {
	if _, ok := AudioOf([]byte("NativeNetworkingImpl route changed")); ok {
		t.Error("text read as a stats log")
	}
}
