package document

import (
	"github.com/lin-string/go-playa/contentdata"
)

func testXObject(spec contentdata.XObjectSpec) XObjectObject {
	return newXObjectObject(spec)
}

func testXObjectStream(stream Stream) XObjectObject {
	return testXObject(contentdata.XObjectSpec{Stream: stream})
}

func testXObjectRefStream(ref Ref, stream Stream) XObjectObject {
	return testXObject(contentdata.XObjectSpec{Ref: ref, Stream: stream})
}
