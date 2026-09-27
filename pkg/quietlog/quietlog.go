// Package quietlog keeps personal data out of go-zero's RPC logs.
//
// go-zero logs the whole request of every call its client sees fail, and of
// every call its server receives while the stat interceptor is on. For most
// calls that is ids; for some it is a phone number, a name, or an address book.
// A failed user_getUserIdByPhone put the number of somebody who is not on ice9
// into the error log - the one thing the contacts permission text promises is
// not kept.
//
// The methods are found rather than listed: every registered RPC whose request
// carries a personal field, looked for two messages deep, so an address book
// (a list of InputContact, each with a phone) counts and a method added later
// is covered without anybody remembering to add it.
package quietlog

import (
	"sort"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// personal are the field names that name or reach a person.
var personal = map[protoreflect.Name]bool{
	"phone":        true,
	"phone_number": true,
	"phones":       true,
	"first_name":   true,
	"last_name":    true,
}

// depth is how many messages down a personal field is looked for: the request
// itself, and the messages its fields hold.
const depth = 2

// Scanner finds the RPC methods whose requests carry personal data.
type Scanner struct {
	files *protoregistry.Files
}

// NewScanner reads the services registered in files.
func NewScanner(files *protoregistry.Files) *Scanner {
	return &Scanner{files: files}
}

// Methods returns the full gRPC names ("/user.RPCUser/user_getUserIdByPhone")
// of every method whose request carries a personal field, sorted.
func (s *Scanner) Methods() []string {
	var found []string
	s.files.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		services := file.Services()
		for i := 0; i < services.Len(); i++ {
			service := services.Get(i)
			methods := service.Methods()
			for j := 0; j < methods.Len(); j++ {
				method := methods.Get(j)
				if carries(method.Input(), depth) {
					found = append(found, "/"+string(service.FullName())+"/"+string(method.Name()))
				}
			}
		}
		return true
	})
	sort.Strings(found)
	return found
}

func carries(message protoreflect.MessageDescriptor, levels int) bool {
	if levels == 0 {
		return false
	}
	fields := message.Fields()
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		if personal[field.Name()] {
			return true
		}
		if field.Message() != nil && carries(field.Message(), levels-1) {
			return true
		}
	}
	return false
}

// KeepPersonalDataOut tells go-zero, client and server side, to log no content
// for every method the registered services have that carries personal data.
// It says how many it found either way: a count that only speaks when it is
// not zero cannot tell "nothing to hide" from "never ran".
func KeepPersonalDataOut() int {
	methods := NewScanner(protoregistry.GlobalFiles).Methods()
	for _, method := range methods {
		zrpc.DontLogClientContentForMethod(method)
		zrpc.DontLogContentForMethod(method)
	}
	logx.Infof("quietlog: %d methods log no content", len(methods))
	return len(methods)
}
