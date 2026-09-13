package m2reviewprobe

import (
    "context"
    "encoding/json"
    "fmt"
    "path/filepath"
    "secretarysimplified/config"
    ctxbuild "secretarysimplified/context"
    "secretarysimplified/contract"
    "secretarysimplified/core"
    "secretarysimplified/memory"
    "secretarysimplified/model"
    "secretarysimplified/store"
    "strings"
    "testing"
    "time"
)

type fake struct { p *model.Provider; calls int; output func(model.Request) json.RawMessage }
func (f *fake) Encode(r model.Request) ([]byte,error) { return f.p.Encode(r) }
func (f *fake) Generate(_ context.Context, r model.Request) (model.Result,error) { f.calls++; return model.Result{Output:f.output(r)},nil }
func setup(t *testing.T) (*store.Store, config.Config, *model.Provider) {
    t.Helper(); d:=t.TempDir(); s,e:=store.Init(filepath.Join(d,"db"),filepath.Join(d,"objects")); if e!=nil {t.Fatal(e)}; t.Cleanup(func(){s.Close()}); c:=config.Default(d); return s,c,model.New(c)
}
func env(origin, text string) contract.InputEnvelope { return contract.InputEnvelope{SchemaVersion:1,RequestID:contract.NewID(),SessionID:contract.NewID(),PrincipalID:"probe",Origin:origin,ReceivedAt:contract.Now(),Text:text,AttachmentRefs:[]contract.ObjectRef{},DataClass:"SYNTHETIC",Extensions:map[string]any{}} }
func decision(r model.Request, actions []contract.ActionProposal) json.RawMessage { b,_:=json.Marshal(contract.DecisionEnvelope{SchemaVersion:1,ContextID:r.ContextID,Reply:map[string]any{"text":"ok","evidence":[]any{}},Actions:actions,Controls:[]contract.Control{},Extensions:map[string]any{}}); return b }
func create() contract.ActionProposal { return contract.ActionProposal{OperationKey:"create_item",Kind:"CREATE_ITEM",Payload:map[string]any{"domain":"work","kind":"TASK","title":"from source","due_at":nil,"timezone":"UTC","priority":1}} }
func TestSourceModelCanCommitItem(t *testing.T) { s,c,p:=setup(t); f:=&fake{p:p}; f.output=func(r model.Request)json.RawMessage{return decision(r,[]contract.ActionProposal{create()})}; in:=env("SOURCE","untrusted source text"); turn,e:=s.AcceptInput(context.Background(),in,100); if e!=nil {t.Fatal(e)}; if e=(&core.Service{Store:s,Model:f,Config:c}).Process(context.Background(),turn); e!=nil {t.Fatal(e)}; items,e:=s.ListItems(context.Background()); if e!=nil {t.Fatal(e)}; if len(items)!=1 {t.Fatalf("items=%d",len(items))} }
func TestWorldProposalWithoutGrantIsPersisted(t *testing.T) { s,c,p:=setup(t); ctx:=context.Background(); obj,e:=s.PutObject(ctx,[]byte("evidence"),"text/plain","SYNTHETIC");if e!=nil{t.Fatal(e)}; entity,fact:=contract.NewID(),contract.NewID(); prop:=contract.WorldUpdateProposal{SchemaVersion:1,ID:contract.NewID(),RequestID:contract.NewID(),EntityID:entity,Predicate:"entity.relation",Operation:"ASSERT",FactID:fact,ExpectedRevision:0,Value:&map[string]any{"relation":"related","target_entity_id":contract.NewID()},Evidence:[]contract.EvidenceRef{{ObjectID:obj.ID,SHA256:obj.SHA256,Locator:"full",OriginID:obj.ID,DataClass:"SYNTHETIC"}},Basis:"INFERENCE",PolicyRevision:1,Reason:"inference",Extensions:map[string]any{}}; payload:=map[string]any{}; raw,_:=json.Marshal(prop);json.Unmarshal(raw,&payload); f:=&fake{p:p};f.output=func(r model.Request)json.RawMessage{return decision(r,[]contract.ActionProposal{{OperationKey:"world",Kind:"WORLD_PROPOSAL",Payload:payload}})}; in:=env("SOURCE","untrusted fact"); turn,e:=s.AcceptInput(ctx,in,100);if e!=nil{t.Fatal(e)};if e=(&core.Service{Store:s,Model:f,Config:c}).Process(ctx,turn);e!=nil{t.Fatalf("process: %v",e)};var proposals,tasks int;s.DB.QueryRow("SELECT count(*) FROM world_proposal").Scan(&proposals);s.DB.QueryRow("SELECT count(*) FROM task").Scan(&tasks);if proposals!=1||tasks!=1{t.Fatalf("proposals=%d tasks=%d",proposals,tasks)} }
func TestConflictArchivesUncommittedText(t *testing.T) { s,_,_:=setup(t); ctx:=context.Background(); in:=env("MASTER_CLI","one"); if _,e:=s.AcceptInput(ctx,in,100);e!=nil{t.Fatal(e)}; in.Text="two"; if _,e:=s.AcceptInput(ctx,in,100);e==nil || e.Error()!="IDEMPOTENCY_CONFLICT" {t.Fatal(e)}; var n int; if e:=s.DB.QueryRow("SELECT count(*) FROM object_ref").Scan(&n);e!=nil{t.Fatal(e)}; if n!=2 {t.Fatalf("object_ref=%d",n)} }
func TestSummaryDropsPendingQuestions(t *testing.T) { s,c,p:=setup(t); ctx:=context.Background(); in:=env("MASTER_CLI","question"); if _,e:=s.AcceptInput(ctx,in,100);e!=nil{t.Fatal(e)}; f:=&fake{p:p}; q:=contract.NewID(); f.output=func(r model.Request)json.RawMessage{ b,_:=json.Marshal(contract.ConversationSummaryDraft{SchemaVersion:1,Summary:"s",FocusEntityIDs:[]string{},PendingQuestionIDs:[]string{q},CommitmentItemIDs:[]string{},Extensions:map[string]any{}}); return b }; m:=memory.Service{Store:s,Model:f,Config:c,Epoch:time.Now()}; v,e:=m.Summarize(ctx,in.SessionID); if e==nil || e.Error()!="UNKNOWN_PENDING_QUESTION" {t.Fatalf("state=%v err=%v",v,e)} }
func TestEpochIsNotPersistent(t *testing.T) { s,c,p:=setup(t); ctx:=context.Background(); e:=time.Now().UTC(); m:=memory.Service{Store:s,Model:p,Config:c,Epoch:e}; if _,err:=m.Refresh(ctx,e);err!=nil{t.Fatal(err)}; m2:=memory.Service{Store:s,Model:p,Config:c,Epoch:e.Add(-48*time.Hour)}; if _,err:=m2.Refresh(ctx,e.Add(24*time.Hour));err!=nil{t.Fatal(err)}; var n int; if err:=s.DB.QueryRow("SELECT count(*) FROM consciousness_snapshot").Scan(&n);err!=nil{t.Fatal(err)}; if n!=2 {t.Fatalf("snapshots=%d",n)} }
func TestContextIsNotContextContract(t *testing.T) { s,c,p:=setup(t); turn,e:=s.AcceptInput(context.Background(),env("MASTER_CLI","x"),100);if e!=nil{t.Fatal(e)}; req,_,e:=(&ctxbuild.Builder{Store:s,Model:p,Config:c}).Build(context.Background(),turn,time.Now());if e!=nil{t.Fatal(e)}; if e=contract.Validate("Context",req.Input);e==nil{t.Fatal("unexpected valid Context")}; b,_:=json.Marshal(req.Input); if strings.Contains(string(b),"output_contract") {t.Fatal("output_contract unexpectedly present")} }
func TestProviderCanWireSecretUnderSyntheticLabel(t *testing.T) { c:=config.Default("/tmp"); p:=model.New(c); b,e:=p.Encode(model.Request{RootID:contract.NewID(),ContextID:contract.NewID(),SessionID:contract.NewID(),OutputType:"DecisionEnvelope",DataClass:"SYNTHETIC",Input:map[string]any{"secret_marker":"DO_NOT_SEND"}});if e!=nil{t.Fatal(e)};if !strings.Contains(string(b),"DO_NOT_SEND"){t.Fatal(fmt.Errorf("marker not in wire"))} }
