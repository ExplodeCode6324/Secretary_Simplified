package store

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"secretarysimplified/contract"
	"sort"
	"strings"
)

type RetrievalPage struct {
	Events       []contract.ConversationEvent `json:"events"`
	DataClasses  []string                     `json:"data_classes"`
	Cursor       *string                      `json:"cursor"`
	OmittedCount int                          `json:"omitted_count"`
}
type memoryCursor struct {
	MaxRowID  int64  `json:"max_row_id"`
	LastRowID int64  `json:"last_row_id"`
	QueryHash string `json:"query_hash"`
}

func (s *Store) SearchMemoryPage(ctx context.Context, query string, entityIDs []string, cursor *string) (out RetrievalPage, err error) {
	out.Events = []contract.ConversationEvent{}
	out.DataClasses = []string{}
	if len(query) > 512 || len(entityIDs) > 20 {
		return out, errors.New("INVALID_QUERY")
	}
	ids := append([]string{}, entityIDs...)
	sort.Strings(ids)
	prefix, hasPrefix := turnPrefix(ctx)
	hash, _ := contract.ValueHash(map[string]any{"query": query, "entity_ids": ids, "prefix_turn": prefix.TurnID})
	cur := memoryCursor{QueryHash: hash}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if cursor != nil {
		b, e := base64.RawURLEncoding.DecodeString(*cursor)
		if e != nil {
			return out, errors.New("INVALID_CURSOR")
		}
		if _, e = contract.ParseJSON(b); e != nil {
			return out, errors.New("INVALID_CURSOR")
		}
		d := json.NewDecoder(strings.NewReader(string(b)))
		d.DisallowUnknownFields()
		if e = d.Decode(&cur); e != nil || cur.QueryHash != hash || cur.MaxRowID < 0 || cur.LastRowID < 1 || cur.LastRowID > cur.MaxRowID+1 {
			return out, errors.New("INVALID_CURSOR")
		}
	} else {
		if e = tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(rowid),0) FROM conversation_event").Scan(&cur.MaxRowID); e != nil {
			return out, e
		}
		if hasPrefix && cur.MaxRowID > prefix.RowID {
			cur.MaxRowID = prefix.RowID
		}
		cur.LastRowID = cur.MaxRowID + 1
	}
	sqlQuery := "SELECT e.rowid,e.payload_json,json_extract(t.payload_json,'$.input.data_class') FROM conversation_event e JOIN input_turn t ON json_extract(e.payload_json,'$.turn_id')=t.id WHERE e.rowid<=? AND e.rowid<? AND instr(json_extract(e.payload_json,'$.text'),?)>0"
	args := []any{cur.MaxRowID, cur.LastRowID, query}
	if hasPrefix {
		where, pArgs := prefixPredicate(prefix, "e")
		sqlQuery += " AND (" + where + ")"
		args = append(args, pArgs...)
	}
	if len(ids) > 0 {
		filters := []string{}
		for _, id := range ids {
			filters = append(filters, "instr(e.payload_json,?)>0")
			args = append(args, id)
		}
		sqlQuery += " AND (" + strings.Join(filters, " OR ") + ")"
	}
	sqlQuery += " ORDER BY e.rowid DESC LIMIT 11"
	rows, e := tx.QueryContext(ctx, sqlQuery, args...)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	last := int64(0)
	candidates := 0
	discoveredRefs, retainedRefs := map[string]bool{}, map[string]bool{}
	for rows.Next() {
		var rowID int64
		var b []byte
		var class string
		if e = rows.Scan(&rowID, &b, &class); e != nil {
			return out, e
		}
		var event contract.ConversationEvent
		var eventBody map[string]any
		if json.Unmarshal(b, &eventBody) != nil {
			return out, contract.ErrOutputClassUnknown
		}
		if eventBody["role"] != "MASTER" {
			if _, e = classificationBytes(b); e != nil {
				return out, e
			}
		}
		if e = contract.Decode("ConversationEvent", b, &event); e != nil {
			return out, e
		}
		if event.Role != "MASTER" {
			class, e = contract.ReadClassification(event.Extensions)
			if e != nil {
				return out, e
			}
		} else {
			var e error
			event.Extensions, e = contract.ClassifyExtensions(event.Extensions, class)
			if e != nil {
				return out, e
			}
		}
		for _, ref := range event.Evidence {
			discoveredRefs[ref.ObjectID] = true
		}
		candidates++
		if candidates == 11 {
			next := cur
			next.LastRowID = last
			raw, _ := json.Marshal(next)
			encoded := base64.RawURLEncoding.EncodeToString(raw)
			out.Cursor = &encoded
			break
		}
		last = rowID
		// Whole oversized candidates are omitted; do not imply their original text was served.
		if len([]byte(event.Text)) > 2048 {
			continue
		}
		out.Events = append(out.Events, event)
		out.DataClasses = append(out.DataClasses, class)
		for _, ref := range event.Evidence {
			retainedRefs[ref.ObjectID] = true
		}
	}
	// D09: ref-level net coverage loss, not a count of rows or duplicate citations.
	for id := range discoveredRefs {
		if !retainedRefs[id] {
			out.OmittedCount++
		}
	}
	return out, rows.Err()
}
