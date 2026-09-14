// Package core validates proposals and commits each complete decision atomically.
package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"secretarysimplified/config"
	ctxbuild "secretarysimplified/context"
	"secretarysimplified/contract"
	"secretarysimplified/diagnostics"
	"secretarysimplified/memory"
	"secretarysimplified/model"
	"secretarysimplified/store"
	"time"
)

type Service struct {
	Store   *store.Store
	Model   model.Client
	Config  config.Config
	GrantID string
}

func (s *Service) Step(ctx context.Context) error {
	turns, e := s.Store.PendingTurns(ctx)
	if e != nil {
		return e
	}
	for _, t := range turns {
		if e = s.Process(ctx, t); e != nil {
			return e
		}
	}
	return nil
}
func (s *Service) Run(ctx context.Context) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if e := s.Step(ctx); e != nil && ctx.Err() != nil {
			return ctx.Err()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
func (s *Service) Process(ctx context.Context, t contract.InputTurn) error {
	current, e := s.Store.GetTurn(ctx, t.ID)
	if e != nil {
		return e
	}
	if current.State == "COMMITTED" || current.State == "FAILED" {
		return nil
	}
	t = current
	client := diagnostics.Recorded(s.Model, s.Store, s.Config)
	builder := ctxbuild.Builder{Store: s.Store, Model: client, Config: s.Config, GrantID: s.GrantID}
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		req, manifest, e := builder.Build(ctx, t, time.Now())
		if e != nil {
			last = e
			break
		}
		if e = s.Store.ChargeBudget(ctx, t.IntentID, "model_calls", 1); e != nil {
			last = e
			break
		}
		if e = s.Store.ChargeBudget(ctx, t.IntentID, "output_tokens", s.Config.Limits.OutputTokens); e != nil {
			last = e
			break
		}
		r, e := client.Generate(ctx, req)
		if e != nil {
			builder.ValidationFeedback = r.ValidationIssues
			diagnostics.RecordDecisionOutcome(s.Config, s.Store, r.CallID, manifest.ID, t.IntentID, attempt+1, "PROVIDER_FAILED", errorCode(e), req.DataClass)
			last = e
			continue
		}
		var d contract.DecisionEnvelope
		if e = contract.Decode("DecisionEnvelope", r.Output, &d); e != nil {
			last = e
			continue
		}
		if e = contract.CheckNoClassification(d); e != nil {
			last = e
			break
		}
		if d.ContextID != manifest.ID {
			builder.ValidationFeedback = []string{"/context_id must exactly match current Context.context_id"}
			last = errors.New("CONTEXT_ID_MISMATCH")
			diagnostics.RecordDecisionOutcome(s.Config, s.Store, r.CallID, manifest.ID, t.IntentID, attempt+1, "SEMANTIC_REJECTED", "CONTEXT_ID_MISMATCH", req.DataClass)
			continue
		}
		if len(d.Controls) > 0 {
			reads := 0
			for _, control := range d.Controls {
				if control.Kind == "READ_MEMORY" {
					reads++
				}
			}
			if reads > 0 {
				if reads != 1 || len(d.Controls) != 1 || len(d.Actions) != 0 {
					last = errors.New("INVALID_READ_MEMORY_COMBINATION")
					continue
				}
				control := d.Controls[0]
				var query string
				var entityIDs []string
				var cursor *string
				decode(control.Payload["query"], &query)
				decode(control.Payload["entity_ids"], &entityIDs)
				decode(control.Payload["cursor"], &cursor)
				if cursor != nil && (builder.RetrievalCursor == nil || *cursor != *builder.RetrievalCursor) {
					last = errors.New("INVALID_CURSOR")
					continue
				}
				if e = s.Store.ChargeBudget(ctx, t.IntentID, "retrievals", 1); e != nil {
					last = e
					break
				}
				page, readErr := s.Store.SearchMemoryPage(ctx, query, entityIDs, cursor)
				if readErr != nil {
					last = readErr
					break
				}
				builder.RetrievalServed = true
				builder.RetrievedEvents = page.Events
				builder.RetrievedClasses = page.DataClasses
				builder.RetrievalCursor = page.Cursor
				builder.RetrievalOmitted = page.OmittedCount
				diagnostics.RecordDecisionOutcome(s.Config, s.Store, r.CallID, manifest.ID, t.IntentID, attempt+1, "READ_MEMORY", "bounded retrieval requested", req.DataClass)
				attempt--
				continue
			}
		}
		if e = validateNaturalReminderPolicy(d.Actions); e != nil {
			last = e
			builder.ValidationFeedback = []string{"REMINDER_POLICY_UNSUPPORTED: CREATE_JOB notify.local/alarm.play through natural language requires misfire=FIRE_ONCE_WITHIN_GRACE and grace_seconds=300. Natural-language custom late policies are unsupported; explain that authenticated Typed configuration is required and return no actions or controls for an explicitly custom intent. Do not silently replace such an intent with defaults."}
			diagnostics.RecordDecisionOutcome(s.Config, s.Store, r.CallID, manifest.ID, t.IntentID, attempt+1, "SEMANTIC_REJECTED", "REMINDER_POLICY_UNSUPPORTED", req.DataClass)
			continue
		}
		keys := []string{}
		seen := map[string]bool{}
		for _, a := range d.Actions {
			if seen[a.OperationKey] {
				return errors.New("DUPLICATE_OPERATION")
			}
			seen[a.OperationKey] = true
			keys = append(keys, a.OperationKey)
		}
		e = s.Store.FinishDecisionTurn(ctx, t.ID, d.Reply, keys, manifest.ReadSet, req.DataClass, func(tx *sql.Tx, turn contract.InputTurn) error {
			_, carriesQuestions := d.Reply["questions"]
			if len(d.Actions) > 0 || len(d.Controls) > 0 || carriesQuestions || turn.Input.AnswerToQuestionID != nil {
				if turn.Input.Origin != "MASTER_CLI" {
					return errors.New("AUTHORIZATION_DENIED")
				}
				if _, e := store.CheckCoreAuthorityTx(ctx, tx, s.GrantID, turn.PrincipalID, time.Now()); e != nil {
					return e
				}
			}
			if e := store.CheckReadSetTx(ctx, tx, manifest.ReadSet); e != nil {
				return e
			}
			ids := map[string]string{}
			for _, a := range d.Actions {
				if a.Kind == "CREATE_ITEM" {
					ids[a.OperationKey] = contract.NewID()
				}
			}
			for _, a := range d.Actions {
				if e = s.apply(ctx, tx, turn, a, ids, false, req.DataClass); e != nil {
					return e
				}
			}
			for _, control := range d.Controls {
				if e = s.Store.ApplyControlTx(ctx, tx, control, t.IntentID, filepath.Join(s.Config.DataDir, "objects", "artifacts"), req.DataClass); e != nil {
					return e
				}
			}
			raw, _ := json.Marshal(d)
			_, e := tx.ExecContext(ctx, "INSERT INTO decision_record VALUES(?,?,?,?,?,?)", contract.NewID(), t.IntentID, manifest.ID, "COMMITTED", string(raw), contract.Now())
			return e
		})
		if e == nil {
			diagnostics.RecordDecisionOutcome(s.Config, s.Store, r.CallID, manifest.ID, t.IntentID, attempt+1, "COMMITTED", "", req.DataClass)
			if needed, checkErr := s.Store.ShouldSummarize(ctx, t.SessionID); checkErr == nil && needed {
				epoch, _ := time.Parse(time.RFC3339Nano, s.Config.Epoch)
				mem := memory.Service{Store: s.Store, Model: s.Model, Config: s.Config, Epoch: epoch}
				_, _ = mem.Summarize(ctx, t.SessionID)
			}
			return nil
		}
		diagnostics.RecordDecisionOutcome(s.Config, s.Store, r.CallID, manifest.ID, t.IntentID, attempt+1, "COMMIT_REJECTED", errorCode(e), req.DataClass)
		last = e
		if e.Error() != "CONFLICT" {
			break
		}
	}
	code := "DECISION_REJECTED"
	if last != nil {
		code = errorCode(last)
	}
	return s.Store.FinishTurn(ctx, t.ID, map[string]any{"text": "Request was recorded, but no actions were committed: " + code, "evidence": []any{}}, []string{}, nil)
}

// Only Process, the natural-language decision path, invokes this guard.
// Authenticated Typed requests carry explicit policy fields and remain unchanged.
func validateNaturalReminderPolicy(actions []contract.ActionProposal) error {
	for _, a := range actions {
		if a.Kind != "CREATE_JOB" {
			continue
		}
		var command contract.Command
		if e := decode(a.Payload["command"], &command); e != nil {
			return e
		}
		if command.Capability != "notify.local" && command.Capability != "alarm.play" {
			continue
		}
		var grace int
		if e := decode(a.Payload["grace_seconds"], &grace); e != nil {
			return e
		}
		if a.Payload["misfire"] != "FIRE_ONCE_WITHIN_GRACE" || grace != 300 {
			return errors.New("REMINDER_POLICY_UNSUPPORTED")
		}
	}
	return nil
}

func errorCode(e error) string {
	for _, c := range []string{"OUTPUT_CLASS_UNKNOWN", "CLASSIFICATION_INJECTION", "QUESTION_TEXT_EMPTY", "QUESTION_ANSWER_EMPTY", "QUESTION_ALREADY_RESOLVED", "QUESTION_NOT_FOUND_IN_SESSION", "QUESTION_AUTHORITY_DENIED", "QUESTION_ITEM_NOT_IN_CONTEXT", "QUESTION_DUPLICATE", "QUESTION_CAPACITY_EXCEEDED", "REMINDER_POLICY_UNSUPPORTED", "CONFLICT", "DISCLOSURE_DENIED", "BUDGET_EXHAUSTED", "CONTEXT_REQUIRED_OVERFLOW", "MODEL_HTTP_429", "MODEL_HTTP_401", "MODEL_SECRET_UNAVAILABLE"} {
		if e.Error() == c {
			return c
		}
	}
	return "DECISION_REJECTED"
}
func decode(v any, out any) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, out)
}
func (s *Service) apply(ctx context.Context, tx *sql.Tx, t contract.InputTurn, a contract.ActionProposal, ids map[string]string, trusted bool, dataClass string) error {
	if e := contract.Validate("ActionProposal", a); e != nil {
		return e
	}
	rawPayload, err := json.Marshal(a.Payload)
	if err != nil {
		return err
	}
	var p map[string]any
	if err = json.Unmarshal(rawPayload, &p); err != nil {
		return err
	}
	now := contract.Now()
	switch a.Kind {
	case "CREATE_ITEM":
		if !trusted {
			if e := store.ChargeCoreActionTx(ctx, tx, t.IntentID); e != nil {
				return e
			}
		}
		v := contract.Item{SchemaVersion: 1, ID: ids[a.OperationKey], Revision: 1, Domain: p["domain"].(string), Kind: p["kind"].(string), Title: p["title"].(string), Status: "OPEN", Timezone: p["timezone"].(string), TimeState: "UNKNOWN", DependencyIDs: []string{}, Evidence: []contract.EvidenceRef{}, CreatedAt: now, UpdatedAt: now, Extensions: map[string]any{}}
		for _, ref := range t.Input.AttachmentRefs {
			if ref.MediaType == "text/plain" && ref.SHA256 == contract.Hash([]byte(t.Input.Text)) {
				v.Evidence = append(v.Evidence, contract.EvidenceRef{ObjectID: ref.ID, SHA256: ref.SHA256, Locator: "full_text", OriginID: ref.ID, DataClass: ref.DataClass})
			}
		}
		decode(p["priority"], &v.Priority)
		if p["due_at"] != nil {
			x := p["due_at"].(string)
			v.DueAt = &x
			v.TimeState = "CONFIRMED"
		}
		v.Extensions, err = contract.ClassifyExtensions(v.Extensions, dataClass)
		if err != nil {
			return err
		}
		return store.PutItemTx(ctx, tx, v, 0)
	case "UPDATE_ITEM":
		if !trusted {
			if e := store.ChargeCoreActionTx(ctx, tx, t.IntentID); e != nil {
				return e
			}
		}
		var id string
		decode(p["item_id"], &id)
		var raw []byte
		if e := tx.QueryRowContext(ctx, "SELECT payload_json FROM item WHERE id=?", id).Scan(&raw); e != nil {
			return e
		}
		var v contract.Item
		if e := contract.Decode("Item", raw, &v); e != nil {
			return e
		}
		var expected int
		decode(p["expected_revision"], &expected)
		changes := p["changes"].(map[string]any)
		for k, x := range changes {
			switch k {
			case "title":
				v.Title = x.(string)
			case "status":
				if x == "DONE" && t.Input.Origin != "MASTER_CLI" {
					return errors.New("PERMISSION_DENIED")
				}
				v.Status = x.(string)
			case "priority":
				decode(x, &v.Priority)
			case "due_at":
				v.DueAt = nil
				v.TimeState = "UNKNOWN"
				if x != nil {
					q := x.(string)
					v.DueAt = &q
					v.TimeState = "CONFIRMED"
				}
			}
		}
		v.Revision = expected + 1
		v.UpdatedAt = now
		oldClass, classErr := contract.ReadClassification(v.Extensions)
		if classErr != nil {
			return classErr
		}
		joined, classErr := contract.JoinClass(oldClass, dataClass)
		if classErr != nil {
			return classErr
		}
		v.Extensions, classErr = contract.ClassifyExtensions(v.Extensions, joined)
		if classErr != nil {
			return classErr
		}
		return store.PutItemTx(ctx, tx, v, expected)
	case "CANCEL_TASK":
		if !trusted {
			if e := store.ChargeCoreActionTx(ctx, tx, t.IntentID); e != nil {
				return e
			}
		}
		id, _ := p["task_id"].(string)
		var expected, current int
		decode(p["expected_revision"], &expected)
		if e := tx.QueryRowContext(ctx, "SELECT revision FROM task WHERE id=?", id).Scan(&current); e != nil {
			return e
		}
		if current != expected {
			return errors.New("REVISION_CONFLICT")
		}
		return s.Store.CancelTaskTx(ctx, tx, id)
	case "SUBMIT_TASK":
		var c contract.Command
		decode(p["command"], &c)
		c.OperationKey = a.OperationKey
		c.Extensions, err = contract.ClassifyExtensions(c.Extensions, dataClass)
		if err != nil {
			return err
		}
		criteria, e := store.DeriveCriteria(c)
		if e != nil {
			return e
		}
		var proposed []contract.Criterion
		decode(p["criteria"], &proposed)
		if e = matchCriteria(criteria, proposed); e != nil {
			return e
		}
		_, e = s.Store.RegisterImmediateTx(ctx, tx, t.IntentID, t.IntentID, c, criteria, s.GrantID)
		return e
	case "CREATE_JOB":
		var schedule contract.Schedule
		decode(p["schedule"], &schedule)
		var c contract.Command
		decode(p["command"], &c)
		c.OperationKey = a.OperationKey
		c.Extensions, err = contract.ClassifyExtensions(c.Extensions, dataClass)
		if err != nil {
			return err
		}
		criteria, e := store.DeriveCriteria(c)
		if e != nil {
			return e
		}
		template := p["task_template"].(map[string]any)
		var proposed []contract.Criterion
		decode(template["criteria"], &proposed)
		if e = matchCriteria(criteria, proposed); e != nil {
			return e
		}
		template["criteria"] = criteria
		if key, ok := template["item_operation_key"].(string); ok {
			if ids[key] == "" {
				return errors.New("INVALID_ITEM_OPERATION")
			}
			template["item_id"] = ids[key]
			template["item_operation_key"] = nil
		}
		job := contract.ScheduledJob{SchemaVersion: 1, ID: contract.NewID(), Revision: 1, RootID: t.IntentID, Enabled: true, Schedule: schedule, Command: c, TaskTemplate: template, Misfire: p["misfire"].(string), Overlap: "SKIP", MaxAttempts: 3, UpdatedAt: now, Extensions: map[string]any{}}
		decode(p["grace_seconds"], &job.GraceSeconds)
		job.Extensions, err = contract.ClassifyExtensions(job.Extensions, dataClass)
		if err != nil {
			return err
		}
		return s.Store.RegisterJobTx(ctx, tx, job, s.GrantID)
	case "WORLD_PROPOSAL":
		var proposal contract.WorldUpdateProposal
		decode(p, &proposal)
		proposalClass := dataClass
		for _, ref := range proposal.Evidence {
			proposalClass, err = contract.JoinClass(proposalClass, ref.DataClass)
			if err != nil {
				return err
			}
		}
		proposal.Extensions, err = contract.ClassifyExtensions(proposal.Extensions, proposalClass)
		if err != nil {
			return err
		}
		proposal.RequestID = t.RequestID
		proposal.PolicyRevision = 1
		if !trusted {
			proposal.Basis = "INFERENCE"
		}
		if proposal.Basis == "MASTER_EXPLICIT" && t.Input.Origin != "MASTER_CLI" {
			return errors.New("PERMISSION_DENIED")
		}
		if e := s.Store.PutProposalTx(ctx, tx, proposal); e != nil {
			return e
		}
		cmd := contract.Command{SchemaVersion: 1, OperationKey: a.OperationKey, Capability: "world.update", CapabilityVersion: 1, Arguments: map[string]any{"proposal_id": proposal.ID}, ExpectedRevisions: []contract.ReadRef{}, Extensions: map[string]any{}}
		cmd.Extensions, err = contract.ClassifyExtensions(cmd.Extensions, dataClass)
		if err != nil {
			return err
		}
		crit := []contract.Criterion{{ID: contract.NewID(), Kind: "world_revision_matches", Expected: map[string]any{"fact_id": proposal.FactID, "revision": proposal.ExpectedRevision + 1}, EvidencePolicy: "WorldCommitService"}}
		_, e := s.Store.RegisterImmediateTx(ctx, tx, t.IntentID, t.IntentID, cmd, crit, s.GrantID)
		return e
	default:
		return fmt.Errorf("UNSUPPORTED_ACTION: %s", a.Kind)
	}
}
func matchCriteria(expected, proposed []contract.Criterion) error {
	if len(expected) != len(proposed) {
		return errors.New("CRITERION_MISMATCH")
	}
	for i, c := range expected {
		a, _ := json.Marshal(c.Expected)
		b, _ := json.Marshal(proposed[i].Expected)
		if c.Kind != proposed[i].Kind || string(a) != string(b) {
			return errors.New("CRITERION_MISMATCH")
		}
	}
	return nil
}

// Typed applies the same transaction path without a language model round trip.
func (s *Service) Typed(ctx context.Context, requestID, session string, actions []contract.ActionProposal) (contract.InputTurn, error) {
	return s.TypedClass(ctx, requestID, session, actions, "PERSONAL")
}

func (s *Service) TypedClass(ctx context.Context, requestID, session string, actions []contract.ActionProposal, dataClass string) (contract.InputTurn, error) {
	if _, err := contract.JoinClass(dataClass); err != nil {
		return contract.InputTurn{}, err
	}
	if err := contract.CheckNoClassification(actions); err != nil {
		return contract.InputTurn{}, err
	}
	raw, e := json.Marshal(actions)
	if e != nil {
		return contract.InputTurn{}, e
	}
	// Execution binds template fields; do not mutate the caller-owned proposal.
	if e = json.Unmarshal(raw, &actions); e != nil {
		return contract.InputTurn{}, e
	}
	for _, a := range actions {
		if e = contract.Validate("ActionProposal", a); e != nil {
			return contract.InputTurn{}, e
		}
	}
	in := contract.InputEnvelope{SchemaVersion: 1, RequestID: requestID, SessionID: session, PrincipalID: "master", Origin: "MASTER_CLI", ReceivedAt: contract.Now(), Text: string(raw), AttachmentRefs: []contract.ObjectRef{}, DataClass: dataClass, Extensions: map[string]any{}}
	ids := map[string]string{}
	keys := []string{}
	for _, a := range actions {
		keys = append(keys, a.OperationKey)
		if a.Kind == "CREATE_ITEM" {
			ids[a.OperationKey] = typedItemID(requestID, a.OperationKey)
		}
	}
	t, e := s.Store.AcceptTyped(ctx, in, s.Config.Limits.Queue, map[string]any{"text": "Typed actions committed. Execution is asynchronous.", "evidence": []any{}}, keys, func(tx *sql.Tx, t contract.InputTurn) error {
		for _, a := range actions {
			if e := s.apply(ctx, tx, t, a, ids, true, dataClass); e != nil {
				return e
			}
		}
		return nil
	})
	if e != nil {
		return t, e
	}
	return s.Store.GetTurn(ctx, t.ID)
}

func typedItemID(request, operation string) string {
	hash, _ := contract.ValueHash([]string{"typed-item-v1", "master", request, operation})
	return contract.DeriveID(hash)
}
