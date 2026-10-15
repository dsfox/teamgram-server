// Package tones holds the notification sounds ice9 hands out: three short
// files made by make.sh and embedded in the binary, listed by
// account.getSavedRingtones and served by upload.getFile under fixed document
// ids. No database row and no file store, so the stand and production answer
// alike and nothing has to be installed (#282).
package tones

import (
	"embed"
	"hash/fnv"
	"strconv"

	"github.com/teamgram/proto/mtproto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

//go:embed drop.mp3 chime.mp3 knock.mp3
var files embed.FS

// Tone is one sound as the clients see it: a document with a fixed id.
type Tone struct {
	Id   int64
	Name string
	File string
	// Whole seconds, rounded up: what documentAttributeAudio carries.
	Duration int32
}

// All is the list in the order the clients show it. The ids are ours alone -
// no document the media service makes starts with 0x1ce9 - and the iOS
// client knows them too (Ice9Tones.swift), to list them under Alert Tones.
var All = []Tone{
	{Id: 0x1ce9_0000_0000_0001, Name: "Drop", File: "drop.mp3", Duration: 1},
	{Id: 0x1ce9_0000_0000_0002, Name: "Chime", File: "chime.mp3", Duration: 1},
	{Id: 0x1ce9_0000_0000_0003, Name: "Knock", File: "knock.mp3", Duration: 1},
}

const (
	// One access hash for all three: nothing checks it, since the files are
	// public and the ids are known.
	accessHash int64 = 0x1ce9_0000_0000_00ff
	// 15 October 2026, when the tones were made; a document's date and a
	// file's mtime, both fixed so a phone never sees them change.
	date int32 = 1760486400
	// What the media service puts on every document it makes.
	dcId int32 = 1
	mime       = "audio/mpeg"
)

// ById is the tone with that document id.
func ById(id int64) (Tone, bool) {
	for _, tone := range All {
		if tone.Id == id {
			return tone, true
		}
	}
	return Tone{}, false
}

// Bytes is the whole file of a tone.
func (tone Tone) Bytes() []byte {
	data, err := files.ReadFile(tone.File)
	if err != nil {
		// The files are embedded at build time; a name that is not there is a
		// mistake in All, caught by the test.
		panic("tones: " + tone.File + " is not embedded: " + err.Error())
	}
	return data
}

// Document is the tone as account.getSavedRingtones lists it.
func (tone Tone) Document() *mtproto.Document {
	data := tone.Bytes()
	return mtproto.MakeTLDocument(&mtproto.Document{
		Id:            tone.Id,
		AccessHash:    accessHash,
		FileReference: []byte{},
		Date:          date,
		MimeType:      mime,
		Size2:         int64(len(data)),
		Size2_INT32:   int32(len(data)),
		Size2_INT64:   int64(len(data)),
		DcId:          dcId,
		Attributes: []*mtproto.DocumentAttribute{
			mtproto.MakeTLDocumentAttributeAudio(&mtproto.DocumentAttribute{
				Duration:         float64(tone.Duration),
				Duration_INT32:   tone.Duration,
				Duration_FLOAT64: float64(tone.Duration),
				Title:            &wrapperspb.StringValue{Value: tone.Name},
				Performer:        &wrapperspb.StringValue{Value: "ice9"},
			}).To_DocumentAttribute(),
			mtproto.MakeTLDocumentAttributeFilename(&mtproto.DocumentAttribute{
				FileName: tone.Name + ".mp3",
			}).To_DocumentAttribute(),
		},
	}).To_Document()
}

// Documents is the whole list.
func Documents() []*mtproto.Document {
	documents := make([]*mtproto.Document, 0, len(All))
	for _, tone := range All {
		documents = append(documents, tone.Document())
	}
	return documents
}

// Hash names this list to a client, which hands it back to ask "still the
// same?". Any stable value over the ids will do; positive, so no client
// takes it for a flag.
func Hash() int64 {
	sum := fnv.New64a()
	for _, tone := range All {
		sum.Write([]byte(strconv.FormatInt(tone.Id, 10)))
		sum.Write([]byte{0})
	}
	return int64(sum.Sum64() >> 1)
}

// ServeFile answers upload.getFile for a tone's document: the bytes from
// offset, at most limit of them, as upload.file. False for any other location,
// which the file store then answers as before. An offset past the end is an
// empty answer, which is how a client learns a download is complete.
func ServeFile(location *mtproto.InputFileLocation, offset int64, limit int32) (*mtproto.Upload_File, bool) {
	if location == nil || location.GetPredicateName() != mtproto.Predicate_inputDocumentFileLocation {
		return nil, false
	}
	tone, ok := ById(location.GetId())
	if !ok {
		return nil, false
	}
	data := tone.Bytes()
	if offset < 0 || offset > int64(len(data)) {
		offset = int64(len(data))
	}
	end := int64(len(data))
	if limit > 0 && offset+int64(limit) < end {
		end = offset + int64(limit)
	}
	return mtproto.MakeTLUploadFile(&mtproto.Upload_File{
		Type:  mtproto.MakeTLStorageFileMp3(nil).To_Storage_FileType(),
		Mtime: date,
		Bytes: data[offset:end],
	}).To_Upload_File(), true
}
