package reactions

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/teamgram/proto/mtproto"
)

// The set the image carries, read from where the repository keeps it.
const shipped = "../../teamgramd/reactions"

func loadShipped(t *testing.T) *Catalog {
	t.Helper()
	catalog, err := Load(shipped)
	if err != nil {
		t.Fatalf("the shipped set does not load: %v", err)
	}
	return catalog
}

func TestTheShippedSetIsTheOwnersNine(t *testing.T) {
	catalog := loadShipped(t)
	var emoji []string
	for _, reaction := range catalog.Reactions() {
		emoji = append(emoji, reaction.GetEmoticon())
	}
	want := []string{"👍", "👎", "❤️", "🔥", "😁", "😢", "😮", "🙏", "🎉"}
	if len(emoji) != len(want) {
		t.Fatalf("got %v", emoji)
	}
	for i := range want {
		if emoji[i] != want[i] {
			t.Fatalf("got %v, want %v", emoji, want)
		}
	}
	if catalog.Hash() == 0 {
		t.Fatal("zero is the hash every client cached the empty list under")
	}
	if !catalog.Offers("❤️") || catalog.Offers("🤡") || catalog.Offers("") {
		t.Fatal("offers exactly the set")
	}
}

// iOS drops a reaction missing any of the five required documents, and leaves
// one without the last two out of the menu; a nil one cannot be encoded at all.
func TestEveryReactionHasEveryDocumentAndEncodes(t *testing.T) {
	catalog := loadShipped(t)
	for _, reaction := range catalog.Available() {
		documents := map[string]*mtproto.Document{
			"static_icon":        reaction.StaticIcon,
			"appear_animation":   reaction.AppearAnimation,
			"select_animation":   reaction.SelectAnimation,
			"activate_animation": reaction.ActivateAnimation,
			"effect_animation":   reaction.EffectAnimation,
			"around_animation":   reaction.AroundAnimation,
			"center_icon":        reaction.CenterIcon,
		}
		for slot, document := range documents {
			if document == nil || document.Id == 0 || document.AccessHash == 0 || document.Size2_INT64 == 0 {
				t.Fatalf("%s: %s is missing or empty", reaction.Reaction, slot)
			}
		}
		if reaction.StaticIcon.MimeType != "image/webp" || reaction.SelectAnimation.MimeType != "application/x-tgsticker" {
			t.Fatalf("%s: a still icon and an animation, got %s and %s",
				reaction.Reaction, reaction.StaticIcon.MimeType, reaction.SelectAnimation.MimeType)
		}
		if int32(reaction.StaticIcon.AccessHash>>32) != int32(mtproto.CRC32_storage_fileWebp) {
			t.Fatalf("%s: the file service reads the storage type out of the access hash", reaction.Reaction)
		}
		named := false
		for _, attribute := range reaction.SelectAnimation.Attributes {
			if attribute.GetPredicateName() == mtproto.Predicate_documentAttributeFilename && attribute.FileName != "" {
				named = true
			}
		}
		if !named {
			t.Fatalf("%s: iOS plays an animation only when it has a file name", reaction.Reaction)
		}
	}

	answer := mtproto.MakeTLMessagesAvailableReactions(&mtproto.Messages_AvailableReactions{
		Hash:      catalog.Hash(),
		Reactions: catalog.Available(),
	})
	x := mtproto.NewEncodeBuf(4096)
	if err := answer.Encode(x, 214); err != nil {
		t.Fatalf("the answer does not encode: %v", err)
	}
}

func TestPlacePutsEveryFileWhereTheFileServiceReadsIt(t *testing.T) {
	catalog := loadShipped(t)
	files := t.TempDir()

	written, err := catalog.Place(files)
	if err != nil || written != 18 {
		t.Fatalf("first start writes all eighteen: %d, %v", written, err)
	}
	for _, reaction := range catalog.Available() {
		for _, document := range []*mtproto.Document{reaction.StaticIcon, reaction.SelectAnimation} {
			path := filepath.Join(files, "documents", fmtId(document.Id))
			info, err := os.Stat(path)
			if err != nil || info.Size() != document.Size2_INT64 {
				t.Fatalf("%s: %s is not there whole: %v", reaction.Reaction, path, err)
			}
		}
	}

	written, err = catalog.Place(files)
	if err != nil || written != 0 {
		t.Fatalf("the next start finds them in place: %d, %v", written, err)
	}

	broken := filepath.Join(files, "documents", fmtId(catalog.Available()[0].StaticIcon.Id))
	if err = os.WriteFile(broken, []byte("not a picture"), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err = catalog.Place(files)
	if err != nil || written != 1 {
		t.Fatalf("a damaged file is written again, and only that one: %d, %v", written, err)
	}
}

func TestASourceThatIsNotTheManifestsIsRefused(t *testing.T) {
	dir := t.TempDir()
	data, err := os.ReadFile(filepath.Join(shipped, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "1f44d.webp"), []byte("something else"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err = catalog.Place(t.TempDir()); err == nil {
		t.Fatal("a picture with the wrong bytes would be served as a reaction")
	}
}

func fmtId(id int64) string {
	return fmt.Sprintf("%d.dat", id)
}
