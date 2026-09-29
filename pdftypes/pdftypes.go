// Package pdftypes exposes the primitive values used by PDF documents.
package pdftypes

import (
	"encoding/base64"
	"fmt"

	"github.com/lin-string/go-playa/geometry"
	"github.com/lin-string/go-playa/pdftypes/primitives"
)

type (
	Object        = primitives.Object
	Null          = primitives.Null
	Bool          = primitives.Bool
	Number        = primitives.Number
	Name          = primitives.Name
	String        = primitives.String
	Array         = primitives.Array
	InvalidArray  = primitives.InvalidArray
	Dict          = primitives.Dict
	Ref           = primitives.Ref
	ObjRef        = Ref
	Keyword       = primitives.Keyword
	ContentStream = Stream
	Matrix        = geometry.Matrix
	Point         = geometry.Point
	Rect          = geometry.Rect
)

func NumberValue(value Object) (float64, bool) { return primitives.NumberValue(value) }
func IntValue(value Object) (int, bool)        { return primitives.IntValue(value) }
func NameValue(value Object) (string, bool)    { return primitives.NameValue(value) }
func RefValue(value Object) (Ref, bool)        { return primitives.RefValue(value) }

// AsObject returns a JSON-friendly metadata view of a PDF primitive graph.
// Non-ASCII byte strings use Playa's base64: representation.
func AsObject(value Object) any {
	switch value := value.(type) {
	case nil, Null:
		return nil
	case Bool:
		return bool(value)
	case Number:
		return float64(value)
	case Name:
		return string(value)
	case String:
		return objectBytes(value)
	case Array:
		return objectArray(value)
	case InvalidArray:
		return objectArray(Array(value))
	case Dict:
		result := make(map[string]any, len(value))
		for key, item := range value {
			result[string(key)] = AsObject(item)
		}
		return result
	case Ref:
		return map[string]any{"ref": value.Object}
	case Stream:
		result, _ := AsObject(value.DictCopy()).(map[string]any)
		result["stream_id"] = nil
		return result
	default:
		return fmt.Sprint(value)
	}
}

func objectArray(value Array) []any {
	result := make([]any, len(value))
	for index, item := range value {
		result[index] = AsObject(item)
	}
	return result
}

func objectBytes(value String) string {
	for _, item := range value {
		if item >= 0x80 {
			return "base64:" + base64.StdEncoding.EncodeToString(value)
		}
	}
	return string(value)
}

// Resolve follows indirect references with a caller-provided object lookup.
// The optional default is returned when a reference cannot be resolved.
func Resolve(value Object, resolve func(Ref) (Object, bool), defaults ...Object) Object {
	var defaultValue Object
	if len(defaults) != 0 {
		defaultValue = defaults[0]
	}
	seen := make(map[Ref]struct{})
	for {
		ref, ok := value.(Ref)
		if !ok {
			return value
		}
		if _, ok := seen[ref]; ok {
			return defaultValue
		}
		seen[ref] = struct{}{}
		resolved, ok := resolve(ref)
		if !ok {
			return defaultValue
		}
		value = resolved
	}
}

func ResolveAll(value Object, resolve func(Ref) (Object, bool)) Object {
	return resolveAll(value, resolve, map[Ref]bool{})
}

func resolveAll(value Object, resolve func(Ref) (Object, bool), seen map[Ref]bool) Object {
	if ref, ok := value.(Ref); ok {
		if seen[ref] {
			return value
		}
		seen[ref] = true
		defer delete(seen, ref)
		if resolved, found := resolve(ref); found {
			return resolveAll(resolved, resolve, seen)
		}
		return value
	}
	switch value := value.(type) {
	case Array:
		out := make(Array, len(value))
		for i, item := range value {
			out[i] = resolveAll(item, resolve, seen)
		}
		return out
	case InvalidArray:
		out := make(InvalidArray, len(value))
		for i, item := range value {
			out[i] = resolveAll(item, resolve, seen)
		}
		return out
	case Dict:
		out := make(Dict, len(value))
		for key, item := range value {
			out[key] = resolveAll(item, resolve, seen)
		}
		return out
	case Stream:
		resolvedDict, ok := resolveAll(value.DictBorrowed(), resolve, seen).(Dict)
		if !ok {
			return value.ClonePDFObject()
		}
		return NewStreamOwned(resolvedDict, primitives.CloneBytes(value.DataBorrowed())).WithRefValue(value.Ref())
	default:
		return clonePDFObject(value)
	}
}
