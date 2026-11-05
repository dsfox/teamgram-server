package core

import (
	"time"

	"github.com/teamgram/proto/mtproto"
)

// MlsGetKeyLog answers for one person's keys (#44): the log's signed head,
// every key of theirs it holds with the proof that it does, and the proof
// that the log now holds the one the caller saw last.
//
// Anybody may ask about anybody: the same keys are in every key package the
// server hands to whoever starts a conversation with that person.
//
// mls.getKeyLog user_id:long known_size:long = mls.KeyLog;
func (c *MlsCore) MlsGetKeyLog(in *mtproto.TLMlsGetKeyLog) (*mtproto.Mls_KeyLog, error) {
	answer, err := c.svcCtx.KeyLog.Read(c.ctx, in.GetUserId(), in.GetKnownSize(), int32(time.Now().Unix()))
	if err != nil {
		c.Logger.Errorf("mls.getKeyLog - %v", err)
		return nil, mtproto.ErrInternalServerError
	}
	if in.GetKnownSize() > answer.Head.Size {
		// The caller saw a larger log than there is: the log was cut short, or
		// this is another server. Said here so the server's own log shows it;
		// the phone sees the sizes and refuses.
		c.Logger.Errorf("mls.getKeyLog - %d saw a log of %d, which has %d", c.MD.UserId, in.GetKnownSize(), answer.Head.Size)
	}

	out := &mtproto.Mls_KeyLog{Head: &mtproto.Mls_KeyLogHead{
		Size:      answer.Head.Size,
		Root:      answer.Head.Root[:],
		Date:      answer.Head.Date,
		LogKey:    answer.Head.LogKey,
		Signature: answer.Head.Signature,
	}}
	for i, e := range answer.Entries {
		proof := make([][]byte, 0, len(answer.Proofs[i]))
		for _, h := range answer.Proofs[i] {
			proof = append(proof, append([]byte{}, h[:]...))
		}
		out.Entries = append(out.Entries, &mtproto.Mls_KeyLogEntry{
			Index: e.Index, Name: e.Name, SignatureKey: e.SignatureKey, Proof: proof,
		})
	}
	for _, h := range answer.Consistency {
		out.Consistency = append(out.Consistency, append([]byte{}, h[:]...))
	}
	c.Logger.Infof("mls.getKeyLog - %d key(s) of %d in a log of %d", len(out.Entries), in.GetUserId(), answer.Head.Size)
	return out, nil
}
