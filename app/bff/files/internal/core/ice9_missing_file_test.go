package core

import (
	"strings"
	"testing"

	"github.com/teamgram/proto/mtproto"
)

// The line for the case of 27 October names who asked and how (#295).
func TestAMissingFileSaysWhoAskedAndHow(t *testing.T) {
	location := mtproto.MakeTLInputPhotoFileLocation(&mtproto.InputFileLocation{
		Id:        2115102355165810688,
		ThumbSize: "y",
	}).To_InputFileLocation()
	line := missingFileLine(136909096, "android", 231, location, 0, 131072)
	for _, want := range []string{"user 136909096", `client "android" layer 231`,
		"inputPhotoFileLocation", "id 2115102355165810688", `thumb "y"`} {
		if !strings.Contains(line, want) {
			t.Errorf("%q does not say %q", line, want)
		}
	}

	empty := mtproto.MakeTLUploadFile(&mtproto.Upload_File{Bytes: []byte{}}).To_Upload_File()
	full := mtproto.MakeTLUploadFile(&mtproto.Upload_File{Bytes: []byte{1}}).To_Upload_File()
	if !answeredNothing(empty, 0) {
		t.Error("an empty first part is not counted as nothing")
	}
	if answeredNothing(full, 0) || answeredNothing(empty, 524288) {
		t.Error("a file with bytes, or the empty end of a file read to its end, is counted as missing")
	}
}
