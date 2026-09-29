package document

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

func TestParseObjectBodyWithResolverUsesIndirectLength(t *testing.T) {
	objects := map[Ref]Object{{Object: 7}: Number(1)}
	resolve := func(o Object) Object {
		if ref, ok := o.(Ref); ok {
			return objects[ref]
		}
		return o
	}
	obj, err := parseObjectBodyWithResolver([]byte("<< /Length 7 0 R >>\nstream\nAendstream trailing"), resolve)
	stream, ok := obj.(Stream)
	if err != nil || !ok || string(stream.DataBorrowed()) != "A" {
		t.Fatalf("object=%#v stream=%#v err=%v", obj, stream, err)
	}
}

func TestParseObjectBodyWithResolverFollowsMultiLevelIndirectLength(t *testing.T) {
	objects := map[Ref]Object{
		{Object: 7}: Ref{Object: 8},
		{Object: 8}: Number(11),
	}
	resolve := func(o Object) Object {
		if ref, ok := o.(Ref); ok {
			return objects[ref]
		}
		return o
	}
	obj, err := parseObjectBodyWithResolver([]byte("<< /Length 7 0 R >>\nstream\nAendstreamBendstreamCendstream trailing"), resolve)
	stream, ok := obj.(Stream)
	if err != nil || !ok || string(stream.DataBorrowed()) != "AendstreamB" {
		t.Fatalf("multi-level object=%#v stream=%#v err=%v", obj, stream, err)
	}
}

func TestParseObjectBodyReturnsTypedStreamError(t *testing.T) {
	_, err := parseObjectBodyWithResolver([]byte("<< /Length 5 >> stream abc"), nil)
	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("error type = %T, want ParseError", err)
	}
	if parseErr.Operation() != "stream" {
		t.Fatalf("stream parse error = %#v", parseErr)
	}
}

func TestStreamLengthOverflowDoesNotPanic(t *testing.T) {
	// The padding makes start large enough that the declared length overflows
	// int when added before the slice bounds are checked.
	length := strconv.FormatInt(int64(^uint(0)>>1)-1024, 10)
	body := []byte(strings.Repeat(" ", 2048) + "<< /Length " + length + " >>\nstream\nAendstream")

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("stream length caused panic: %v", recovered)
		}
	}()
	_, _ = parseObjectBodyWithResolver(body, nil)
}

func TestIndirectBodyLengthOverflowDoesNotPanic(t *testing.T) {
	length := strconv.FormatInt(int64(^uint(0)>>1)-1024, 10)
	data := []byte("1 0 obj\n" + strings.Repeat(" ", 2048) + "<< /Length " + length + " >>\nstream\nAendstream\nendobj")

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("indirect stream length caused panic: %v", recovered)
		}
	}()
	_, _ = indirectBody(data, 0)
}
