package apirouter

import (
	"encoding"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"unsafe"
)

// Struct fields tagged with the "protect" option (e.g. `json:"password,protect"`) are
// omitted from responses unless the context has SetShowProtectedFields(true).
//
// encoding/json/v2 has no notion of this option, so in public mode a custom marshaler
// replaces any struct type containing protected fields with a "shadow" struct holding
// only the visible fields (embedded structs are flattened following Go's field
// visibility rules), which is then marshaled normally.

var (
	protectCache sync.Map // reflect.Type → *protectInfo (nil if the type has no protected fields)

	// publicJsonOpts are the json options used to marshal values in public mode
	publicJsonOpts = json.WithMarshalers(json.MarshalToFunc(marshalPublic))

	marshalerTypes = []reflect.Type{
		reflect.TypeFor[json.Marshaler](),
		reflect.TypeFor[json.MarshalerTo](),
		reflect.TypeFor[encoding.TextMarshaler](),
		reflect.TypeFor[encoding.TextAppender](),
	}
)

type protectInfo struct {
	shadow reflect.Type
	fields [][]int // index path in the original type for each field of shadow
}

type protectField struct {
	sf     reflect.StructField
	index  []int
	name   string // json name
	tag    string // json tag to use in shadow struct
	depth  int
	tagged bool
}

// marshalPublic is called by json/v2 for every value, as a pointer to that value.
func marshalPublic(enc *jsontext.Encoder, v any) error {
	rv := reflect.ValueOf(v).Elem()
	if rv.Kind() != reflect.Struct {
		return errors.ErrUnsupported
	}
	info := getProtectInfo(rv.Type())
	if info == nil {
		return errors.ErrUnsupported
	}

	sv := reflect.New(info.shadow).Elem()
	for i, idx := range info.fields {
		fv, err := rv.FieldByIndexErr(idx)
		if err != nil {
			// nil embedded pointer
			continue
		}
		// clear the read-only flag on values reached through unexported embedded structs
		fv = reflect.NewAt(fv.Type(), unsafe.Pointer(fv.UnsafeAddr())).Elem()
		sv.Field(i).Set(fv)
	}
	return json.MarshalEncode(enc, sv.Addr().Interface())
}

func getProtectInfo(t reflect.Type) *protectInfo {
	if v, ok := protectCache.Load(t); ok {
		return v.(*protectInfo)
	}
	info := buildProtectInfo(t)
	protectCache.Store(t, info)
	if info != nil {
		protectCache.Store(info.shadow, (*protectInfo)(nil))
	}
	return info
}

func buildProtectInfo(t reflect.Type) *protectInfo {
	pt := reflect.PointerTo(t)
	for _, m := range marshalerTypes {
		if pt.Implements(m) {
			// type handles its own marshaling
			return nil
		}
	}

	var fields []*protectField
	hasProtect := false

	var walk func(t reflect.Type, index []int, depth int, visited map[reflect.Type]bool)
	walk = func(t reflect.Type, index []int, depth int, visited map[reflect.Type]bool) {
		visited[t] = true
		defer delete(visited, t)

		for i := 0; i < t.NumField(); i++ {
			sf := t.Field(i)
			tag, tagged := sf.Tag.Lookup("json")
			if tag == "-" {
				continue
			}
			name, opts := splitJsonTag(tag)
			fidx := append(append([]int(nil), index...), i)

			if (sf.Anonymous && name == "") || hasJsonOpt(opts, "embed") {
				et := sf.Type
				if et.Kind() == reflect.Pointer {
					et = et.Elem()
				}
				if et.Kind() == reflect.Struct {
					if !visited[et] {
						walk(et, fidx, depth+1, visited)
					}
					continue
				}
			}
			if !sf.IsExported() {
				continue
			}
			if hasJsonOpt(opts, "protect") {
				hasProtect = true
				continue
			}

			f := &protectField{sf: sf, index: fidx, depth: depth, tagged: tagged && name != ""}
			if name == "" {
				f.name = sf.Name
				f.tag = sf.Name + opts
			} else {
				f.name = name
				f.tag = tag
			}
			fields = append(fields, f)
		}
	}
	walk(t, nil, 0, make(map[reflect.Type]bool))

	if !hasProtect {
		return nil
	}

	// apply Go visibility rules for fields sharing the same json name: the
	// shallowest wins, and on a tie a single tagged field wins, else all are dropped
	byName := make(map[string][]*protectField)
	for _, f := range fields {
		byName[f.name] = append(byName[f.name], f)
	}
	keep := make(map[*protectField]bool)
	for _, list := range byName {
		minDepth := list[0].depth
		for _, f := range list {
			minDepth = min(minDepth, f.depth)
		}
		var best []*protectField
		for _, f := range list {
			if f.depth == minDepth {
				best = append(best, f)
			}
		}
		if len(best) > 1 {
			var tagged []*protectField
			for _, f := range best {
				if f.tagged {
					tagged = append(tagged, f)
				}
			}
			best = tagged
		}
		if len(best) == 1 {
			keep[best[0]] = true
		}
	}

	info := &protectInfo{}
	var sfields []reflect.StructField
	for _, f := range fields {
		if !keep[f] {
			continue
		}
		sfields = append(sfields, reflect.StructField{
			Name: fmt.Sprintf("F%d", len(sfields)),
			Type: f.sf.Type,
			Tag:  reflect.StructTag("json:" + strconv.Quote(f.tag)),
		})
		info.fields = append(info.fields, f.index)
	}
	info.shadow = reflect.StructOf(sfields)
	return info
}

// splitJsonTag splits a json tag into its name and its options, the latter
// including the leading comma.
func splitJsonTag(tag string) (string, string) {
	if n := strings.IndexByte(tag, ','); n >= 0 {
		return tag[:n], tag[n:]
	}
	return tag, ""
}

func hasJsonOpt(opts, opt string) bool {
	for o := range strings.SplitSeq(opts, ",") {
		if strings.TrimSpace(o) == opt {
			return true
		}
	}
	return false
}
