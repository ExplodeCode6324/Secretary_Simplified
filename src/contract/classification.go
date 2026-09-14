package contract

import (
	"encoding/json"
	"errors"
)

const ClassificationKey = "security.classification"

var ErrOutputClassUnknown = errors.New("OUTPUT_CLASS_UNKNOWN")

func classRank(s string) int {
	switch s {
	case "SYNTHETIC":
		return 0
	case "PERSONAL":
		return 1
	case "SENSITIVE":
		return 2
	case "SECRET":
		return 3
	}
	return -1
}
func JoinClass(classes ...string) (string, error) {
	if len(classes) == 0 {
		return "", ErrOutputClassUnknown
	}
	best := ""
	rank := -1
	for _, c := range classes {
		r := classRank(c)
		if r < 0 {
			return "", ErrOutputClassUnknown
		}
		if r > rank {
			rank = r
			best = c
		}
	}
	return best, nil
}
func ReadClassification(ext map[string]any) (string, error) {
	v, ok := ext[ClassificationKey]
	if !ok {
		return "", ErrOutputClassUnknown
	}
	b, e := json.Marshal(v)
	if e != nil {
		return "", ErrOutputClassUnknown
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil || len(m) != 1 {
		return "", ErrOutputClassUnknown
	}
	c, ok := m["data_class"].(string)
	if !ok || classRank(c) < 0 {
		return "", ErrOutputClassUnknown
	}
	return c, nil
}

// ClassifyExtensions preserves other namespaces and joins an existing valid mark.
// A missing old mark is permitted only when the caller knows this is a new object;
// updating a legacy derived object requires ReadClassification before calling this.
func ClassifyExtensions(ext map[string]any, class string) (map[string]any, error) {
	c, e := JoinClass(class)
	if e != nil {
		return nil, e
	}
	if _, ok := ext[ClassificationKey]; ok {
		old, e := ReadClassification(ext)
		if e != nil {
			return nil, e
		}
		c, e = JoinClass(old, c)
		if e != nil {
			return nil, e
		}
	}
	out := map[string]any{}
	for k, v := range ext {
		out[k] = v
	}
	out[ClassificationKey] = map[string]any{"data_class": c}
	return out, nil
}
func CheckNoClassification(v any) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	var x any
	if e = json.Unmarshal(b, &x); e != nil {
		return e
	}
	var walk func(any) error
	walk = func(v any) error {
		switch x := v.(type) {
		case map[string]any:
			for k, v := range x {
				if k == ClassificationKey {
					return errors.New("CLASSIFICATION_INJECTION")
				}
				if e := walk(v); e != nil {
					return e
				}
			}
		case []any:
			for _, v := range x {
				if e := walk(v); e != nil {
					return e
				}
			}
		}
		return nil
	}
	return walk(x)
}

// RequireDerivedClass reads a known derived DTO or its decoded JSON object.
// The caller selects which schema is derived; this does not infer type from text.
func RequireDerivedClass(v any) (string, error) {
	b, e := json.Marshal(v)
	if e != nil {
		return "", ErrOutputClassUnknown
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return "", ErrOutputClassUnknown
	}
	ext, _ := m["extensions"].(map[string]any)
	return ReadClassification(ext)
}
