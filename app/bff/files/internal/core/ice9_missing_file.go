package core

import (
	"fmt"

	"github.com/teamgram/proto/mtproto"
)

// missingFileLine is what the server says when a phone asked for a file and
// got nothing back. On 27 October an Android phone asked sixteen times for its
// own two just-sent pictures as photos of size "y", which the server holds as
// documents; the only trace was the storage layer's "no such file", naming
// neither who asked nor how, and the case could not be reproduced (#295).
// The line names the account, the client and its layer, and the location's
// shape, so the next one says where it came from.
func missingFileLine(userID int64, client string, layer int32, location *mtproto.InputFileLocation, offset int64, limit int32) string {
	return fmt.Sprintf("upload.getFile answered nothing: user %d, client %q layer %d asked for %s id %d thumb %q at %d+%d",
		userID, client, layer, location.GetPredicateName(), location.GetId(), location.GetThumbSize(), offset, limit)
}

// answeredNothing is true for a download that found no bytes at the start of
// a file: dfs turns "no such file" into an empty answer rather than an error.
func answeredNothing(file *mtproto.Upload_File, offset int64) bool {
	return offset == 0 && file != nil && len(file.GetBytes()) == 0
}
