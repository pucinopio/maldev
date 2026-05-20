// Package canonical encodes Go values to a deterministic JSON form suitable
// for signing: object keys are recursively sorted, no insignificant whitespace
// is emitted, HTML characters are not escaped, and time.Time values are
// rendered in RFC3339Nano UTC.
package canonical

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// Marshal returns a deterministic JSON encoding of v. Map keys are sorted at
// every nesting level so that the output is stable across Go map-iteration
// order, making the byte slice safe for use as a signing payload.
func Marshal(v any) ([]byte, error) {
	norm, err := normalise(v)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(norm); err != nil {
		return nil, err
	}
	out := buf.Bytes()
	if len(out) > 0 && out[len(out)-1] == '\n' {
		out = out[:len(out)-1]
	}
	return out, nil
}

// normalise round-trips v through encoding/json so that struct field tags are
// honoured, then decodes the result into a plain any tree with UseNumber so
// that integer values survive without precision loss.
func normalise(v any) (any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var tree any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&tree); err != nil {
		return nil, err
	}
	return walk(tree)
}

// walk recursively sorts map keys and normalises time strings.
func walk(v any) (any, error) {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := orderedMap{}
		for _, k := range keys {
			nv, err := walk(x[k])
			if err != nil {
				return nil, err
			}
			out = append(out, kv{Key: k, Val: nv})
		}
		return out, nil
	case []any:
		for i, e := range x {
			nv, err := walk(e)
			if err != nil {
				return nil, err
			}
			x[i] = nv
		}
		return x, nil
	case string:
		// Detect RFC3339 strings produced by time.Time.MarshalJSON and
		// re-render in RFC3339Nano UTC so timezone variants hash identically.
		if t, err := time.Parse(time.RFC3339Nano, x); err == nil {
			return t.UTC().Format(time.RFC3339Nano), nil
		}
		return x, nil
	default:
		return x, nil
	}
}

type kv struct {
	Key string
	Val any
}

type orderedMap []kv

func (o orderedMap) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, p := range o {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, err := json.Marshal(p.Key)
		if err != nil {
			return nil, err
		}
		buf.Write(kb)
		buf.WriteByte(':')
		// Use a non-escaping encoder so HTML characters in string values are
		// preserved verbatim rather than being replaced with \uXXXX sequences.
		vb, err := marshalNoEscape(p.Val)
		if err != nil {
			return nil, fmt.Errorf("canonical: value at %q: %w", p.Key, err)
		}
		buf.Write(vb)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// marshalNoEscape encodes v as JSON without HTML-escaping <, >, or &.
func marshalNoEscape(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	b := buf.Bytes()
	if len(b) > 0 && b[len(b)-1] == '\n' {
		b = b[:len(b)-1]
	}
	return b, nil
}
