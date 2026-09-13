package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
)

// ParseJSON rejects duplicate keys, including keys nested in opaque objects.
func ParseJSON(b []byte) (any, error) {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	v, e := readValue(d)
	if e != nil {
		return nil, e
	}
	if _, e = d.Token(); e != io.EOF {
		return nil, fmt.Errorf("INVALID_JSON: trailing content")
	}
	return v, nil
}
func readValue(d *json.Decoder) (any, error) {
	t, e := d.Token()
	if e != nil {
		return nil, e
	}
	if delim, ok := t.(json.Delim); ok {
		switch delim {
		case '{':
			m := map[string]any{}
			for d.More() {
				k, e := d.Token()
				if e != nil {
					return nil, e
				}
				key, ok := k.(string)
				if !ok {
					return nil, fmt.Errorf("INVALID_JSON")
				}
				if _, ok = m[key]; ok {
					return nil, fmt.Errorf("DUPLICATE_JSON_KEY: %s", key)
				}
				v, e := readValue(d)
				if e != nil {
					return nil, e
				}
				m[key] = v
			}
			_, e = d.Token()
			return m, e
		case '[':
			a := []any{}
			for d.More() {
				v, e := readValue(d)
				if e != nil {
					return nil, e
				}
				a = append(a, v)
			}
			_, e = d.Token()
			return a, e
		}
		return nil, fmt.Errorf("INVALID_JSON")
	}
	return t, nil
}

// CanonicalJSON is the v1 semantic encoding: integer numbers only, sorted keys,
// UTF-8 and no HTML escaping. Floating point identity payloads are rejected.
func CanonicalJSON(b []byte) ([]byte, error) {
	v, e := ParseJSON(b)
	if e != nil {
		return nil, e
	}
	var check func(any) error
	check = func(v any) error {
		switch x := v.(type) {
		case json.Number:
			if _, e := strconv.ParseInt(string(x), 10, 64); e != nil {
				return fmt.Errorf("NON_INTEGER_SEMANTIC_NUMBER")
			}
		case map[string]any:
			for _, v := range x {
				if e := check(v); e != nil {
					return e
				}
			}
		case []any:
			for _, v := range x {
				if e := check(v); e != nil {
					return e
				}
			}
		}
		return nil
	}
	if e = check(v); e != nil {
		return nil, e
	}
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	if e = enc.Encode(v); e != nil {
		return nil, e
	}
	return bytes.TrimSuffix(out.Bytes(), []byte("\n")), nil
}
