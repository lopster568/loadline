package canon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

// Kind enumerates the JSON value kinds the ordered model represents.
type Kind int

const (
	KindNull Kind = iota
	KindBool
	KindNumber
	KindString
	KindArray
	KindObject
)

// Value is a JSON value that remembers object key order. Methodology 1.5 fixes
// the canonical serialization at "key order as the server sent it", which
// encoding/json's map decoding destroys.
type Value struct {
	Kind   Kind
	Bool   bool
	Number json.Number
	Str    string
	Arr    []*Value
	Keys   []string
	Fields map[string]*Value
}

// Parse decodes JSON into the ordered model.
func Parse(b []byte) (*Value, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	v, err := parseValue(dec)
	if err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, fmt.Errorf("trailing content after JSON value")
	}
	return v, nil
}

func parseValue(dec *json.Decoder) (*Value, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	return parseFromToken(dec, tok)
}

func parseFromToken(dec *json.Decoder, tok json.Token) (*Value, error) {
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			obj := &Value{Kind: KindObject, Fields: map[string]*Value{}}
			for {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				if d, ok := kt.(json.Delim); ok && d == '}' {
					return obj, nil
				}
				key, ok := kt.(string)
				if !ok {
					return nil, fmt.Errorf("object key is not a string: %v", kt)
				}
				val, err := parseValue(dec)
				if err != nil {
					return nil, err
				}
				if _, dup := obj.Fields[key]; !dup {
					obj.Keys = append(obj.Keys, key)
				}
				obj.Fields[key] = val
			}
		case '[':
			arr := &Value{Kind: KindArray}
			for {
				et, err := dec.Token()
				if err != nil {
					return nil, err
				}
				if d, ok := et.(json.Delim); ok && d == ']' {
					return arr, nil
				}
				val, err := parseFromToken(dec, et)
				if err != nil {
					return nil, err
				}
				arr.Arr = append(arr.Arr, val)
			}
		default:
			return nil, fmt.Errorf("unexpected delimiter %v", t)
		}
	case string:
		return &Value{Kind: KindString, Str: t}, nil
	case json.Number:
		return &Value{Kind: KindNumber, Number: t}, nil
	case bool:
		return &Value{Kind: KindBool, Bool: t}, nil
	case nil:
		return &Value{Kind: KindNull}, nil
	}
	return nil, fmt.Errorf("unsupported token %T", tok)
}

// Get returns the named field of an object value.
func (v *Value) Get(key string) (*Value, bool) {
	if v == nil || v.Kind != KindObject {
		return nil, false
	}
	f, ok := v.Fields[key]
	return f, ok
}

// Set inserts or replaces a field, appending to the key order when new.
func (v *Value) Set(key string, val *Value) {
	if v.Fields == nil {
		v.Fields = map[string]*Value{}
	}
	if _, exists := v.Fields[key]; !exists {
		v.Keys = append(v.Keys, key)
	}
	v.Fields[key] = val
}

// Delete removes a field and its key-order entry.
func (v *Value) Delete(key string) {
	if v == nil || v.Kind != KindObject {
		return
	}
	if _, ok := v.Fields[key]; !ok {
		return
	}
	delete(v.Fields, key)
	for i, k := range v.Keys {
		if k == key {
			v.Keys = append(v.Keys[:i], v.Keys[i+1:]...)
			break
		}
	}
}

// Clone deep-copies the value.
func (v *Value) Clone() *Value {
	if v == nil {
		return nil
	}
	out := &Value{Kind: v.Kind, Bool: v.Bool, Number: v.Number, Str: v.Str}
	switch v.Kind {
	case KindArray:
		out.Arr = make([]*Value, len(v.Arr))
		for i, e := range v.Arr {
			out.Arr[i] = e.Clone()
		}
	case KindObject:
		out.Keys = append([]string(nil), v.Keys...)
		out.Fields = make(map[string]*Value, len(v.Fields))
		for k, f := range v.Fields {
			out.Fields[k] = f.Clone()
		}
	}
	return out
}

// Serialize renders the value on a single line with no insignificant
// whitespace, in stored key order.
func (v *Value) Serialize() string {
	var buf bytes.Buffer
	v.write(&buf, false)
	return buf.String()
}

// SerializeSorted renders the value with object keys and tool-like array
// members ordered deterministically. It backs the order-insensitive digest.
func (v *Value) SerializeSorted() string {
	var buf bytes.Buffer
	v.write(&buf, true)
	return buf.String()
}

func (v *Value) write(buf *bytes.Buffer, sorted bool) {
	switch v.Kind {
	case KindNull:
		buf.WriteString("null")
	case KindBool:
		if v.Bool {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case KindNumber:
		buf.WriteString(v.Number.String())
	case KindString:
		writeJSONString(buf, v.Str)
	case KindArray:
		buf.WriteByte('[')
		elems := v.Arr
		if sorted {
			elems = sortedArrayCopy(elems)
		}
		for i, e := range elems {
			if i > 0 {
				buf.WriteByte(',')
			}
			e.write(buf, sorted)
		}
		buf.WriteByte(']')
	case KindObject:
		buf.WriteByte('{')
		keys := v.Keys
		if sorted {
			keys = append([]string(nil), v.Keys...)
			sort.Strings(keys)
		}
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			writeJSONString(buf, k)
			buf.WriteByte(':')
			v.Fields[k].write(buf, sorted)
		}
		buf.WriteByte('}')
	}
}

// sortedArrayCopy orders array members by their "name" field when every member
// carries one, which is the tools array. Other arrays keep their order because
// element order is semantic in JSON Schema constructs such as "required".
func sortedArrayCopy(in []*Value) []*Value {
	named := make([]string, len(in))
	for i, e := range in {
		n, ok := e.Get("name")
		if !ok || n.Kind != KindString {
			return in
		}
		named[i] = n.Str
	}
	out := append([]*Value(nil), in...)
	sort.SliceStable(out, func(i, j int) bool { return named[i] < named[j] })
	return out
}

func writeJSONString(buf *bytes.Buffer, s string) {
	var tmp bytes.Buffer
	enc := json.NewEncoder(&tmp)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		buf.WriteString(`""`)
		return
	}
	buf.Write(bytes.TrimRight(tmp.Bytes(), "\n"))
}
