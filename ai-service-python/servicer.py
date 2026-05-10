import ai_pb2
import ai_pb2_grpc
import agent_manager  # <-- Import our new manager


class AIBrainServicer(ai_pb2_grpc.AIBrainServicer):

    def Think(self, request, context):
        """
        This is the "AI Router".
        It finds the correct agent and calls its appropriate "brain".
        """
        print(f"\n--- New gRPC Request Received ---")

        print(f"\nRequest----", request)

        agent_key = request.personality_path
        agent = agent_manager.get_agent(agent_key)  # <-- Use the manager

        if not agent:
            print(f"ERROR: No agent found for key: {agent_key}")
            return ai_pb2.ActionResponse(action_type="SPEAK", content="Error: Agent not found.")

        # Format all dynamic data, including the new quest context.
        dynamic_context = {
            "emotions": request.current_emotions,
            "memories": request.recent_memories,
            "quest_step": request.current_quest_step,
            "completion_rate": request.completion_rate,
        }

        # Route to the correct "brain" based on the event type
        if request.event_type == "PLAYER_ASKED_QUESTION":
            print(f"Routing to RAG agent for: {agent_key}")
            llm_response = agent.run_rag_agent(dynamic_context, request.question_text)
            return ai_pb2.ActionResponse(action_type="SPEAK", content=llm_response)

        elif request.event_type in ("PLAYER_SUBMITTED_QUEST_ITEM", "PLAYER_INTERACT", "PLAYER_INTERACT_QUEST",
                                    "PLAYER_GAVE_GIFT", "PLAYER_ATTACKED"):
            # We route all these interactions to the complex brain
            print(f"Routing to Quest (LangGraph) agent for: {agent_key}")

            # Use the actual item name (keyword) for gifts, otherwise use question_text
            item_or_question = request.question_text
            if request.event_type == "PLAYER_GAVE_GIFT":
                # The Go service sends the item_id in question_text for gifts now
                # But if it were still empty, we would use a placeholder or check keyword
                # For now, assuming question_text contains the item ID for gifts too
                item_or_question = request.question_text  # Or potentially check request.keyword if needed

            event_description = f"Player event: {request.event_type}, Item/Keyword: {item_or_question}"

            print("Item of keyword",event_description)
            llm_response = agent.run_quest_agent(dynamic_context, event_description)
            return ai_pb2.ActionResponse(action_type="SPEAK", content=llm_response)

        else:
            # Fallback for any other event we haven't planned for
            print(f"Routing to simple emotion-based rules (default) for event: {request.event_type}")
            emotions = request.current_emotions
            if emotions.anger > 0.7:
                return ai_pb2.ActionResponse(action_type="SPEAK", content="Get lost.")
            return ai_pb2.ActionResponse(action_type="SPEAK", content="Greetings.")