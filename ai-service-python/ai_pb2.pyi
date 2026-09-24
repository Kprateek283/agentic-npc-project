from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional

DESCRIPTOR: _descriptor.FileDescriptor

class EventRequest(_message.Message):
    __slots__ = ("personality_path", "backstory_path", "lore_path", "speaker_emotions", "general_mood", "memory_lines", "event_type", "question_text", "source_entity_id", "current_quest_step", "completion_rate")
    class SpeakerEmotionsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: float
        def __init__(self, key: _Optional[str] = ..., value: _Optional[float] = ...) -> None: ...
    class GeneralMoodEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: float
        def __init__(self, key: _Optional[str] = ..., value: _Optional[float] = ...) -> None: ...
    PERSONALITY_PATH_FIELD_NUMBER: _ClassVar[int]
    BACKSTORY_PATH_FIELD_NUMBER: _ClassVar[int]
    LORE_PATH_FIELD_NUMBER: _ClassVar[int]
    SPEAKER_EMOTIONS_FIELD_NUMBER: _ClassVar[int]
    GENERAL_MOOD_FIELD_NUMBER: _ClassVar[int]
    MEMORY_LINES_FIELD_NUMBER: _ClassVar[int]
    EVENT_TYPE_FIELD_NUMBER: _ClassVar[int]
    QUESTION_TEXT_FIELD_NUMBER: _ClassVar[int]
    SOURCE_ENTITY_ID_FIELD_NUMBER: _ClassVar[int]
    CURRENT_QUEST_STEP_FIELD_NUMBER: _ClassVar[int]
    COMPLETION_RATE_FIELD_NUMBER: _ClassVar[int]
    personality_path: str
    backstory_path: str
    lore_path: str
    speaker_emotions: _containers.ScalarMap[str, float]
    general_mood: _containers.ScalarMap[str, float]
    memory_lines: _containers.RepeatedScalarFieldContainer[str]
    event_type: str
    question_text: str
    source_entity_id: str
    current_quest_step: int
    completion_rate: float
    def __init__(self, personality_path: _Optional[str] = ..., backstory_path: _Optional[str] = ..., lore_path: _Optional[str] = ..., speaker_emotions: _Optional[_Mapping[str, float]] = ..., general_mood: _Optional[_Mapping[str, float]] = ..., memory_lines: _Optional[_Iterable[str]] = ..., event_type: _Optional[str] = ..., question_text: _Optional[str] = ..., source_entity_id: _Optional[str] = ..., current_quest_step: _Optional[int] = ..., completion_rate: _Optional[float] = ...) -> None: ...

class ActionResponse(_message.Message):
    __slots__ = ("action_type", "content")
    ACTION_TYPE_FIELD_NUMBER: _ClassVar[int]
    CONTENT_FIELD_NUMBER: _ClassVar[int]
    action_type: str
    content: str
    def __init__(self, action_type: _Optional[str] = ..., content: _Optional[str] = ...) -> None: ...

class TokenChunk(_message.Message):
    __slots__ = ("text", "done", "action_type")
    TEXT_FIELD_NUMBER: _ClassVar[int]
    DONE_FIELD_NUMBER: _ClassVar[int]
    ACTION_TYPE_FIELD_NUMBER: _ClassVar[int]
    text: str
    done: bool
    action_type: str
    def __init__(self, text: _Optional[str] = ..., done: bool = ..., action_type: _Optional[str] = ...) -> None: ...
