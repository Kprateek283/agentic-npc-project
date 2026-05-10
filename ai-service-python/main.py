import grpc
from concurrent import futures
from dotenv import load_dotenv

# Load environment variables from .env file BEFORE anything else
load_dotenv()

import ai_pb2_grpc
from agent_manager import load_agents_on_startup  # <-- Import from new file
from servicer import AIBrainServicer  # <-- Import from new file


def serve():
    # 1. Load all agents into the registry
    load_agents_on_startup()

    # 2. Create the gRPC server
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    ai_pb2_grpc.add_AIBrainServicer_to_server(AIBrainServicer(), server)

    # 3. Start the server
    print("Starting Python gRPC server on port 50051...")
    server.add_insecure_port('[::]:50051')
    server.start()
    server.wait_for_termination()


if __name__ == '__main__':
    serve()