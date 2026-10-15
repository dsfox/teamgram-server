package tones

import (
	"bytes"
	"testing"

	"github.com/teamgram/proto/mtproto"
)

// Three small mp3 files, each a real one: an MPEG frame sync at the start
// (make.sh writes no ID3 tag), under 20 KB, with a name and an id of its own.
func TestThreeSmallTonesOfTheirOwn(t *testing.T) {
	if len(All) != 3 {
		t.Fatalf("%d tones, the owner asked for two or three", len(All))
	}
	ids := map[int64]bool{}
	names := map[string]bool{}
	for _, tone := range All {
		data := tone.Bytes()
		if len(data) == 0 || len(data) > 20*1024 {
			t.Errorf("%s is %d bytes; a tone is a few kilobytes", tone.Name, len(data))
		}
		if len(data) < 2 || data[0] != 0xff || data[1]&0xe0 != 0xe0 {
			t.Errorf("%s does not start with an MPEG frame: % x", tone.Name, data[:4])
		}
		if ids[tone.Id] || names[tone.Name] {
			t.Errorf("%s repeats an id or a name", tone.Name)
		}
		ids[tone.Id], names[tone.Name] = true, true
		if tone.Id>>48 != 0x1ce9 {
			t.Errorf("%s has id %x, not one of ours", tone.Name, tone.Id)
		}
	}
}

func TestADocumentNamesTheToneAndItsSize(t *testing.T) {
	tone := All[0]
	document := tone.Document()
	if document.GetId() != tone.Id || document.GetSize2_INT64() != int64(len(tone.Bytes())) {
		t.Fatalf("the document is not the tone: %v", document)
	}
	if document.GetMimeType() != "audio/mpeg" || document.GetDcId() != 1 {
		t.Fatalf("mime %q, dc %d: not what a phone downloads", document.GetMimeType(), document.GetDcId())
	}
	var audio, name string
	for _, attribute := range document.GetAttributes() {
		switch attribute.GetPredicateName() {
		case mtproto.Predicate_documentAttributeAudio:
			audio = attribute.GetTitle().GetValue()
		case mtproto.Predicate_documentAttributeFilename:
			name = attribute.GetFileName()
		}
	}
	if audio != "Drop" || name != "Drop.mp3" {
		t.Fatalf("the phone would show %q / %q, not the tone's name", audio, name)
	}
	if len(Documents()) != len(All) {
		t.Fatalf("%d documents for %d tones", len(Documents()), len(All))
	}
}

func TestTheHashIsStableAndPositive(t *testing.T) {
	if Hash() <= 0 || Hash() != Hash() {
		t.Fatalf("hash %d", Hash())
	}
}

func TestServeFileHonoursOffsetAndLimitAndRefusesTheRest(t *testing.T) {
	tone := All[1]
	data := tone.Bytes()
	location := mtproto.MakeTLInputDocumentFileLocation(&mtproto.InputFileLocation{
		Id: tone.Id, AccessHash: accessHash, FileReference: []byte{},
	}).To_InputFileLocation()

	whole, ok := ServeFile(location, 0, 0)
	if !ok || !bytes.Equal(whole.GetBytes(), data) {
		t.Fatalf("the whole file was not served: ok=%v, %d of %d bytes", ok, len(whole.GetBytes()), len(data))
	}
	if whole.GetType().GetPredicateName() != mtproto.Predicate_storage_fileMp3 {
		t.Fatalf("served as %s, not mp3", whole.GetType().GetPredicateName())
	}
	first, _ := ServeFile(location, 0, 1024)
	second, _ := ServeFile(location, 1024, 1024)
	rest, _ := ServeFile(location, 2048, 1<<20)
	joined := append(append(append([]byte{}, first.GetBytes()...), second.GetBytes()...), rest.GetBytes()...)
	if len(first.GetBytes()) != 1024 || !bytes.Equal(joined, data) {
		t.Fatalf("the chunks do not add up to the file: %d + %d + %d of %d",
			len(first.GetBytes()), len(second.GetBytes()), len(rest.GetBytes()), len(data))
	}
	past, ok := ServeFile(location, int64(len(data))+10, 1024)
	if !ok || len(past.GetBytes()) != 0 {
		t.Fatalf("an offset past the end should be an empty answer: ok=%v, %d bytes", ok, len(past.GetBytes()))
	}

	foreign := mtproto.MakeTLInputDocumentFileLocation(&mtproto.InputFileLocation{Id: 123456}).To_InputFileLocation()
	if _, ok := ServeFile(foreign, 0, 0); ok {
		t.Fatal("a document that is not a tone was served from the binary")
	}
	photo := mtproto.MakeTLInputPhotoFileLocation(&mtproto.InputFileLocation{Id: tone.Id}).To_InputFileLocation()
	if _, ok := ServeFile(photo, 0, 0); ok {
		t.Fatal("a photo location with a tone's id was served as the tone")
	}
	if _, ok := ServeFile(nil, 0, 0); ok {
		t.Fatal("no location was served")
	}
}
