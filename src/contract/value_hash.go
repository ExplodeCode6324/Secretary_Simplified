package contract

import "encoding/json"

// ValueHash applies the contract's semantic JSON encoding before SHA-256.
func ValueHash(v any) (string, error) {
	b, e := json.Marshal(v)
	if e != nil {
		return "", e
	}
	b, e = CanonicalJSON(b)
	if e != nil {
		return "", e
	}
	return Hash(b), nil
}
