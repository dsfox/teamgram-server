package calls

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Audio is what a phone says it did with sound in a call, read from the stats
// log it sends afterwards (#288). The "bitrate" tgcalls records is there from
// every build, but it is the send bandwidth estimate, not the sound: it stays
// up while the microphone gives silence, so it tells a dead transport, not a
// silent phone. The "ice9" summary - the other side's level, whether Android
// silenced the recording, how the app stood when the call was answered - comes
// only from builds that write it. Whether a phone was heard is the OTHER
// phone's peer_heard; neither engine reports a phone's own level.
type Audio struct {
	// Send bitrate in kbit/s, as tgcalls sampled it: how many samples, the
	// highest, and how many were zero.
	BitrateSamples, BitrateMax, BitrateZero int
	// The summary of builds that write one; nil from older builds.
	Ice9 map[string]any
}

type audioLog struct {
	Bitrate []struct {
		B int `json:"b"`
	} `json:"bitrate"`
	Ice9 map[string]any `json:"ice9"`
}

// AudioOf reads the audio part of a stats log. False when the log is not the
// stats shape at all.
func AudioOf(debugLog []byte) (Audio, bool) {
	var log audioLog
	if err := json.Unmarshal(debugLog, &log); err != nil {
		return Audio{}, false
	}
	a := Audio{Ice9: log.Ice9}
	for _, r := range log.Bitrate {
		a.BitrateSamples++
		if r.B > a.BitrateMax {
			a.BitrateMax = r.B
		}
		if r.B == 0 {
			a.BitrateZero++
		}
	}
	return a, true
}

// String is one log line's worth, keys in a fixed order so two calls compare.
func (a Audio) String() string {
	out := fmt.Sprintf("send bitrate %d samples, max %d kbit/s, %d at zero", a.BitrateSamples, a.BitrateMax, a.BitrateZero)
	if a.Ice9 == nil {
		return out + "; no ice9 summary (an older build)"
	}
	keys := []string{"build", "device", "sdk", "incoming", "process_ms", "answer_visible", "answer_screen_on",
		"answer_started_service", "answer_importance", "answer_to_connect_ms", "fgs_types", "mic_granted",
		"silenced", "samples", "peer_max", "peer_heard", "peer_first_ms"}
	parts := make([]string, 0, len(a.Ice9))
	known := make(map[string]bool, len(keys))
	for _, k := range keys {
		known[k] = true
		if v, ok := a.Ice9[k]; ok {
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}
	}
	// Whatever else a client puts in its summary is written too, sorted, so a
	// key added on one platform is not lost on the server (iOS sends its own).
	rest := make([]string, 0, len(a.Ice9))
	for k := range a.Ice9 {
		if !known[k] {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	for _, k := range rest {
		parts = append(parts, fmt.Sprintf("%s=%v", k, a.Ice9[k]))
	}
	return out + "; " + strings.Join(parts, " ")
}
