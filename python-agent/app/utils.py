"""Shared utility helpers for the Python agent."""

from uuid import uuid4

from langchain_core.messages import BaseMessage


def ensure_message_id(message: BaseMessage) -> BaseMessage:
    """Ensure a LangChain message has a unique ID for checkpoint operations.

    LangGraph's RemoveMessage requires an ``id`` to be set. This helper
    assigns one lazily so callers don't need to worry about it.
    """
    if not getattr(message, "id", None):
        message.id = str(uuid4())
    return message
