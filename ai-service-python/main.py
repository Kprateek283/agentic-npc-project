import grpc
from concurrent import futures
from dotenv import load_dotenv

# Load environment variables from .env file BEFORE anything else
load_dotenv()

import uvicorn

import ai_pb2_grpc
from agent_manager import load_agents_on_startup
from api.app import api_port, app
from servicer import AIBrainServicer


def serve():
    # 1. Load all agents into the registry, once, before either transport starts.
    load_agents_on_startup()

    # 2. Start the gRPC server (production path for the Go orchestrator). Non-blocking.
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    ai_pb2_grpc.add_AIBrainServicer_to_server(AIBrainServicer(), server)
    server.add_insecure_port('[::]:50051')
    server.start()
    print("Python gRPC server listening on port 50051")

    # 3. Run the REST API in the main thread (blocks until shutdown).
    port = api_port()
    print(f"FastAPI listening on port {port}")
    uvicorn.run(app, host="0.0.0.0", port=port, log_level="info")

    server.stop(grace=None)


if __name__ == '__main__':
    serve()
