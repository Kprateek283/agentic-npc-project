import importlib

from fastapi.testclient import TestClient

app_module = importlib.import_module("api.app")


def test_health_no_agents(monkeypatch):
    monkeypatch.setattr(app_module, "live_agents", {})
    client = TestClient(app_module.app)
    response = client.get("/health")
    assert response.status_code == 503
    data = response.json()
    assert data["status"] == "no_agents"
    assert data["agents_loaded"] == 0


def test_health_with_agents(monkeypatch):
    monkeypatch.setattr(app_module, "live_agents", {"elara": object()})
    client = TestClient(app_module.app)
    response = client.get("/health")
    assert response.status_code == 200
    data = response.json()
    assert data["status"] == "ok"
    assert data["agents_loaded"] == 1
