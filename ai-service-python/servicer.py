import ai_pb2
import ai_pb2_grpc
from router import UnknownAgentError, route_event


class AIBrainServicer(ai_pb2_grpc.AIBrainServicer):
    """gRPC adapter: unpacks the protobuf request, delegates to the shared router,
    packs the protobuf response. Routing rules live in router.py."""

    def Think(self, request, context):
        print("\n--- New gRPC Request Received ---")

        emotions = request.current_emotions
        dynamic_context = {
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

        try:
            action_type, content = route_event(
                request.personality_path,
                request.event_type,
                request.question_text,
                dynamic_context,
            )
        except UnknownAgentError:
            print(f"ERROR: No agent found for key: {request.personality_path}")
            return ai_pb2.ActionResponse(action_type="SPEAK", content="Error: Agent not found.")

        return ai_pb2.ActionResponse(action_type=action_type, content=content)
