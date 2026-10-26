package deletion

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
)

type result int64

func (r result) LastInsertId() (int64, error) { return 0, nil }
func (r result) RowsAffected() (int64, error) { return int64(r), nil }

type call struct {
	query string
	args  []interface{}
}

func recorder(calls *[]call, fail string) Exec {
	return func(query string, args ...interface{}) (sql.Result, error) {
		*calls = append(*calls, call{query, args})
		if fail != "" && strings.Contains(query, fail) {
			return nil, errors.New("refused")
		}
		return result(2), nil
	}
}

func TestForgetBlanksThenDropsReactionsThenRows(t *testing.T) {
	var calls []call
	gone, err := Forget(recorder(&calls, ""), "messages", 7, []int32{11, 12})
	if err != nil || gone != 2 {
		t.Fatalf("Forget = %d, %v", gone, err)
	}
	if len(calls) != 3 {
		t.Fatalf("%d statements, want 3", len(calls))
	}
	if !strings.HasPrefix(calls[0].query, "update user_pts_updates") || len(calls[0].args) != 1 {
		t.Fatalf("first is not the blanking of the copies: %q %v", calls[0].query, calls[0].args)
	}
	if !strings.HasPrefix(calls[1].query, "delete from message_reactions") || len(calls[1].args) != 2 {
		t.Fatalf("second is not the reactions: %q %v", calls[1].query, calls[1].args)
	}
	if !strings.HasPrefix(calls[2].query, "delete from messages") || len(calls[2].args) != 1 {
		t.Fatalf("third is not the rows: %q %v", calls[2].query, calls[2].args)
	}
	for _, c := range calls {
		if !strings.Contains(c.query, "(11, 12)") {
			t.Fatalf("a statement lost the ids: %q", c.query)
		}
		if strings.Count(c.query, "?") != len(c.args) {
			t.Fatalf("%d placeholders, %d arguments: %q", strings.Count(c.query, "?"), len(c.args), c.query)
		}
	}
}

func TestARowAnOlderServerMarkedGoesToo(t *testing.T) {
	if strings.Contains(DropRows("messages", []int32{1}), "deleted") {
		t.Fatal("the rows are dropped only while unmarked: an older server's marked rows would stay for good")
	}
}

func TestAServiceMessageIsNotGivenWords(t *testing.T) {
	if strings.Contains(Blanked, "json_set") || !strings.Contains(Blanked, "json_replace(update_data, '$.message_MESSAGE.message', '')") {
		t.Fatalf("the words are set rather than replaced, so a service message gains an empty text: %s", Blanked)
	}
	for _, path := range []string{"media", "entities", "reply_to.quote_text"} {
		if !strings.Contains(Blanked, "'$.message_MESSAGE."+path+"'") {
			t.Fatalf("the copy keeps its %s", path)
		}
	}
}

func TestNothingAskedNothingDone(t *testing.T) {
	var calls []call
	if gone, err := Forget(recorder(&calls, ""), "messages", 7, nil); gone != 0 || err != nil || len(calls) != 0 {
		t.Fatalf("an empty list ran %d statements", len(calls))
	}
}

func TestAFailureStopsBeforeTheRowsGo(t *testing.T) {
	var calls []call
	if _, err := Forget(recorder(&calls, "user_pts_updates"), "messages", 7, []int32{1}); err == nil {
		t.Fatal("a failed blanking was not reported")
	}
	if len(calls) != 1 {
		t.Fatalf("after the copies could not be blanked, %d more statement(s) ran: the row would go and its words stay", len(calls)-1)
	}
}
