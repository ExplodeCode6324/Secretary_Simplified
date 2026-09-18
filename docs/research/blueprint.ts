// Compilable interface blueprint, not a production implementation.
import { Agent, type AgentMessage, type AgentTool, type StreamFn } from '@earendil-works/pi-agent-core';
import type { Model, Api } from '@earendil-works/pi-ai';

export type TrustedContext = Readonly<{
  principalId: string; requestId: string; rootBudgetId: string;
  ownerEpoch: number; inputGeneration: number;
  taskId?: string; taskRevision?: number; attemptId?: string;
}>;
export interface DurableJournal {
  appendPiEvent(ctx: TrustedContext, event: unknown): Promise<void>;
  markStorageFailed(error: unknown): void;
}
export interface ModelGateway {
  forContext(ctx: TrustedContext): StreamFn;
}
export function createSecretaryAgent(
  ctx: TrustedContext, model: Model<Api>, systemPrompt: string,
  journal: DurableJournal, gateway: ModelGateway,
  messages: AgentMessage[], tools: AgentTool[],
): Agent {
  const agent = new Agent({
    initialState: { model, systemPrompt, messages, tools }, toolExecution: 'sequential',
    streamFn: gateway.forContext(ctx),
  });
  agent.subscribe(async event => {
    if (event.type !== 'message_end' && event.type !== 'tool_execution_end') return;
    try { await journal.appendPiEvent(ctx, event); }
    catch (error) { journal.markStorageFailed(error); agent.abort(); throw error; }
  });
  return agent;
}

// The application validates execute arguments, permissions, resources and budget
// again inside each AgentTool.execute; the constructor does not provide a sandbox.
