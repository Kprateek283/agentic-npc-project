import grpc
from concurrent import futures
#import time
import os
from dotenv import load_dotenv

import ai_pb2
import ai_pb2_grpc

# --- NEW IMPORT ---
# Import the function from our new agent file.
from herbalist_agent import ask_herbalist

load_dotenv()


class AIBrainServicer(ai_pb2_grpc.AIBrainServicer):

    def Think(self, request, context):
        print(f"--- New Request for NPC: {request.target_npc_id} ---")
        log_level = os.getenv("LOG_LEVEL", "DEBUG")

        # --- NEW ROUTING LOGIC ---
        # If the event is a question, use the powerful LangChain agent.
        if request.event_type == "PLAYER_ASKED_QUESTION":
            print("Decision: Event is a question. Routing to Herbalist LangChain agent.")

            # 1. Assemble the context for the agent.
            npc_context = {
                "name": "Herbalist",  # In a real game, you'd look this up by ID
                "emotions": request.current_emotions,
                "memories": request.recent_memories
            }

            # 2. Call the agent with the context and the question text.
            llm_response = ask_herbalist(npc_context, request.question_text)

            # 3. Return the agent's response.
            return ai_pb2.ActionResponse(
                action_type="SPEAK",
                content=llm_response
            )

        # --- FALLBACK LOGIC ---
        # For all other simple events, use our old emotion-based rules.
        else:
            print("Decision: Event is a simple interaction. Using emotion-based rules.")
            emotions = request.current_emotions
            if log_level.upper() == "INFO":
                print(
                    f"Received Emotions: Anger={emotions.anger:.2f}, Fear={emotions.fear:.2f}, Trust={emotions.trust:.2f}")

            if emotions.fear > 0.6:
                return ai_pb2.ActionResponse(action_type="SPEAK", content="Please, don't hurt me!")

            if emotions.anger > 0.7:
                return ai_pb2.ActionResponse(action_type="SPEAK", content="I have nothing to say to you. Get lost.")

            return ai_pb2.ActionResponse(action_type="SPEAK", content="A fine day, isn't it? Welcome.")
        # --- END OF ROUTING LOGIC ---


def serve():
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    ai_pb2_grpc.add_AIBrainServicer_to_server(AIBrainServicer(), server)
    print("Starting Python gRPC server on port 50051...")
    server.add_insecure_port('[::]:50051')
    server.start()
    server.wait_for_termination()


if __name__ == '__main__':
    serve()