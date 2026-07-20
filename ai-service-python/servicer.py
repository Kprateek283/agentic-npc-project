import logging
import time

import ai_pb2
import ai_pb2_grpc
from router import UnknownAgentError, route_event, stream_event

logger = logging.getLogger(__name__)


def _req_id(context) -> str:
    """The correlation id the Go orchestrator sent via gRPC metadata, or '-'."""
    for key, value in context.invocation_metadata():
        if key == "req-id":
            return value
    return "-"


def _dynamic_context(request) -> dict:
    """Unpack the protobuf request into the plain dict shape the router expects."""
    emotions = request.current_emotions
    return {
        "emotions": {
            "joy": emotions.joy,
            "sadness": emotions.sadness,
            "anger": emotions.anger,
            "fear": emotions.fear,
            "trust": emotions.trust,
        },
        "memories": [mem.description for mem in request.recent_memories],
        "quest_step": request.current_quest_step,
        "completion_rate": request.completion_rate,
    }


class AIBrainServicer(ai_pb2_grpc.AIBrainServicer):
    """gRPC adapter: unpacks the protobuf request, delegates to the shared router,
    packs the protobuf response. Routing rules live in router.py."""

    def Think(self, request, context):
        req_id = _req_id(context)
        start = time.perf_counter()

        dynamic_context = _dynamic_context(request)

        try:
            action_type, content = route_event(
                request.personality_path,
                request.event_type,
                request.question_text,
                dynamic_context,
            )
        except UnknownAgentError:
            logger.error("think req_id=%s event=%s agent=%s unknown_agent",
                         req_id, request.event_type, request.personality_path)
            return ai_pb2.ActionResponse(action_type="SPEAK", content="Error: Agent not found.")

        logger.info("think req_id=%s event=%s agent=%s action=%s dur_ms=%d",
                    req_id, request.event_type, request.personality_path,
                    action_type, (time.perf_counter() - start) * 1000)
        return ai_pb2.ActionResponse(action_type=action_type, content=content)

    def ThinkStream(self, request, context):
        """Streaming counterpart of Think (C3). Yields TokenChunk frames: RAG answers
        stream token-by-token, other events arrive as one content frame; both end with a
        done frame carrying the action_type."""
        req_id = _req_id(context)
        start = time.perf_counter()
        dynamic_context = _dynamic_context(request)

        try:
            for text, done, action_type in stream_event(
                request.personality_path,
                request.event_type,
                request.question_text,
                dynamic_context,
            ):
                yield ai_pb2.TokenChunk(text=text, done=done, action_type=action_type)
        except UnknownAgentError:
            logger.error("think_stream req_id=%s event=%s agent=%s unknown_agent",
                         req_id, request.event_type, request.personality_path)
            yield ai_pb2.TokenChunk(text="Error: Agent not found.", done=False, action_type="SPEAK")
            yield ai_pb2.TokenChunk(text="", done=True, action_type="SPEAK")
            return

        logger.info("think_stream req_id=%s event=%s agent=%s dur_ms=%d",
                    req_id, request.event_type, request.personality_path,
                    (time.perf_counter() - start) * 1000)
