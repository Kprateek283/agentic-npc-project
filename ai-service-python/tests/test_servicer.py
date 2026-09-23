import types

import servicer


class FakeContext:
    def __init__(self, active_count=None):
        self._active_count = active_count
        self._calls = 0

    def invocation_metadata(self):
        return []

    def is_active(self):
        if self._active_count is None:
            return True
        self._calls += 1
        return self._calls <= self._active_count


def make_request():
    return types.SimpleNamespace(
        personality_path="elara",
        event_type="PLAYER_ASKED_QUESTION",
        question_text="hello",
        speaker_emotions={"anger": 0.5},
        general_mood={"joy": 0.2},
        memory_lines=["You gave an apple."],
        current_quest_step=1,
        completion_rate=0.5,
        source_entity_id="player1",
    )


def test_dynamic_context_unpacking():
    req = make_request()
    ctx = servicer._dynamic_context(req)
    assert ctx["speaker_emotions"] == {"anger": 0.5}
    assert ctx["general_mood"] == {"joy": 0.2}
    assert ctx["memory_lines"] == ["You gave an apple."]
    assert ctx["quest_step"] == 1
    assert ctx["completion_rate"] == 0.5
    assert ctx["speaker"] == "player1"


def test_think_stream_cancelled_after_n_chunks(monkeypatch):
    closed = []

    def fake_stream_event(*args, **kwargs):
        try:
            i = 0
            while True:
                yield (f"t{i}", False, "")
                i += 1
        except GeneratorExit:
            closed.append(True)
            raise

    monkeypatch.setattr(servicer, "stream_event", fake_stream_event)

    srv = servicer.AIBrainServicer()
    chunks = list(srv.ThinkStream(make_request(), FakeContext(active_count=3)))

    assert len(chunks) == 3
    assert [c.text for c in chunks] == ["t0", "t1", "t2"]
    assert all(c.done is False for c in chunks)
    assert len(closed) > 0


def test_think_stream_active_client(monkeypatch):
    frames = [
        ("Hello ", False, ""),
        ("world!", False, ""),
        ("", True, "SPEAK"),
    ]

    def fake_stream_event(*args, **kwargs):
        for frame in frames:
            yield frame

    monkeypatch.setattr(servicer, "stream_event", fake_stream_event)

    srv = servicer.AIBrainServicer()
    chunks = list(srv.ThinkStream(make_request(), FakeContext()))

    assert len(chunks) == 3
    assert chunks[0].text == "Hello "
    assert chunks[0].done is False
    assert chunks[0].action_type == ""

    assert chunks[1].text == "world!"
    assert chunks[1].done is False
    assert chunks[1].action_type == ""

    assert chunks[2].text == ""
    assert chunks[2].done is True
    assert chunks[2].action_type == "SPEAK"
