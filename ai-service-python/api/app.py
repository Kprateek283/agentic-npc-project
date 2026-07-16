"""FastAPI layer for the AI service.

Runs alongside the gRPC server in the same process and shares the same in-memory
agent registry. gRPC is the production path used by the Go orchestrator; REST is the
surface used by evals (I1), benchmarks (M4), Docker healthchecks (I4) and demos.
Both transports dispatch through router.route_event.
"""

import os
import traceback

import config
from agent_manager import live_agents
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field
from router import KNOWN_EVENTS, UnknownAgentError, default_context, route_event

app = FastAPI(title="Agentic NPC AI Service", version="1.0.0")


class Emotions(BaseModel):
    joy: float = 0.0
    sadness: float = 0.0
    anger: float = 0.0
    fear: float = 0.0
    trust: float = 0.0


class DynamicContext(BaseModel):
    emotions: Emotions = Field(default_factory=Emotions)
    memories: list[str] = Field(default_factory=list)
    quest_step: int = 0
    completion_rate: float = 0.0


class ChatRequest(BaseModel):
    npc: str = Field(description="Agent key: personality path or NPC directory name (e.g. 'elara')")
    question: str
    context: DynamicContext = Field(default_factory=DynamicContext)


class EventRequest(BaseModel):
    npc: str = Field(description="Agent key: personality path or NPC directory name (e.g. 'elara')")
    event_type: str
    text: str = Field(default="", description="Item id / keyword / free text for the event")
    context: DynamicContext = Field(default_factory=DynamicContext)


class ActionResponse(BaseModel):
    action_type: str
    content: str


class NpcSummary(BaseModel):
    key: str
    npc: str
    name: str
    occupation: str


def _dispatch(npc: str, event_type: str, text: str, context: DynamicContext) -> ActionResponse:
    ctx = default_context() | context.model_dump()
    try:
        action_type, content = route_event(npc, event_type, text, ctx)
    except UnknownAgentError:
        raise HTTPException(status_code=404, detail=f"No agent loaded for key '{npc}'")
    except Exception as exc:
        # Provider/LLM failure: log it in full for operators, return only the type to the
        # caller — a stack trace must never reach a game client.
        traceback.print_exc()
        raise HTTPException(status_code=502, detail=f"AI provider call failed: {type(exc).__name__}") from exc
    return ActionResponse(action_type=action_type, content=content)


@app.get("/health")
def health() -> dict:
    return {
        "status": "ok",
        "agents_loaded": len(live_agents),
        "llm_provider": config.LLM_PROVIDER,
        "chat_model": config.CHAT_MODEL_HEAVY,
        "embedding_model": config.EMBEDDING_MODEL,
        "vector_store": config.VECTOR_STORE,
        "retriever_k": config.RETRIEVER_K,
    }


@app.get("/v1/npcs", response_model=list[NpcSummary])
def list_npcs() -> list[NpcSummary]:
    return [
        NpcSummary(
            key=key,
            npc=key.split("/")[-2],
            name=agent.npc_name,
            occupation=agent.npc_occupation,
        )
        for key, agent in live_agents.items()
    ]


@app.post("/v1/chat", response_model=ActionResponse)
def chat(req: ChatRequest) -> ActionResponse:
    return _dispatch(req.npc, "PLAYER_ASKED_QUESTION", req.question, req.context)


@app.post("/v1/event", response_model=ActionResponse)
def event(req: EventRequest) -> ActionResponse:
    if req.event_type not in KNOWN_EVENTS:
        raise HTTPException(
            status_code=422,
            detail=f"Unknown event_type '{req.event_type}'. Known: {sorted(KNOWN_EVENTS)}",
        )
    return _dispatch(req.npc, req.event_type, req.text, req.context)


def api_port() -> int:
    return int(os.getenv("API_PORT", "8000"))
