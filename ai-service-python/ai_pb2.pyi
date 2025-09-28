from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class MemoryMessage(_message.Message):
    __slots__ = ("description", "importance", "event_type", "participants")
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    IMPORTANCE_FIELD_NUMBER: _ClassVar[int]
    EVENT_TYPE_FIELD_NUMBER: _ClassVar[int]
    PARTICIPANTS_FIELD_NUMBER: _ClassVar[int]
    description: str
    importance: float
    event_type: str
    participants: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, description: _Optional[str] = ..., importance: _Optional[float] = ..., event_type: _Optional[str] = ..., participants: _Optional[_Iterable[str]] = ...) -> None: ...

class EmotionStateMessage(_message.Message):
    __slots__ = ("joy", "sadness", "anger", "fear", "trust")
    JOY_FIELD_NUMBER: _ClassVar[int]
    SADNESS_FIELD_NUMBER: _ClassVar[int]
    ANGER_FIELD_NUMBER: _ClassVar[int]
    FEAR_FIELD_NUMBER: _ClassVar[int]
    TRUST_FIELD_NUMBER: _ClassVar[int]
    joy: float
    sadness: float
    anger: float
    fear: float
    trust: float
    def __init__(self, joy: _Optional[float] = ..., sadness: _Optional[float] = ..., anger: _Optional[float] = ..., fear: _Optional[float] = ..., trust: _Optional[float] = ...) -> None: ...

class EventRequest(_message.Message):
    __slots__ = ("event_type", "target_npc_id", "recent_memories", "current_emotions", "question_text")
    EVENT_TYPE_FIELD_NUMBER: _ClassVar[int]
    TARGET_NPC_ID_FIELD_NUMBER: _ClassVar[int]
    RECENT_MEMORIES_FIELD_NUMBER: _ClassVar[int]
    CURRENT_EMOTIONS_FIELD_NUMBER: _ClassVar[int]
    QUESTION_TEXT_FIELD_NUMBER: _ClassVar[int]
    event_type: str
    target_npc_id: str
    recent_memories: _containers.RepeatedCompositeFieldContainer[MemoryMessage]
    current_emotions: EmotionStateMessage
    question_text: str
    def __init__(self, event_type: _Optional[str] = ..., target_npc_id: _Optional[str] = ..., recent_memories: _Optional[_Iterable[_Union[MemoryMessage, _Mapping]]] = ..., current_emotions: _Optional[_Union[EmotionStateMessage, _Mapping]] = ..., question_text: _Optional[str] = ...) -> None: ...

class ActionResponse(_message.Message):
    __slots__ = ("action_type", "content")
    ACTION_TYPE_FIELD_NUMBER: _ClassVar[int]
    CONTENT_FIELD_NUMBER: _ClassVar[int]
    action_type: str
    content: str
    def __init__(self, action_type: _Optional[str] = ..., content: _Optional[str] = ...) -> None: ...
