"""Redis-backed session state store."""

import json
import logging
import re
from datetime import datetime, timezone
from typing import Optional

import redis

from app.config import Settings
from app.pipeline.silence import SessionState

logger = logging.getLogger(__name__)

# Pattern for valid Redis key characters


class SessionStore:
    def __init__(self, settings: Settings):
        self.settings = settings
        self.redis: Optional[redis.Redis] = None
        self._connect()

    def _connect(self) -> None:
        try:
            self.redis = redis.from_url(
                f"redis://{self.settings.redis_url}",
                password=self.settings.redis_password or None,
                db=self.settings.redis_db,
                decode_responses=True,
            )
            self.redis.ping()
        except (redis.ConnectionError, redis.TimeoutError) as exc:
            logger.warning("SessionStore redis unavailable: %s", exc)
            self.redis = None
        except Exception:  # noqa: BLE001
            logger.exception("SessionStore unexpected error during connection")
            self.redis = None


    def _key(self, user_id: str) -> str:
        return f"agent:session:{user_id}"

    def load(self, user_id: str) -> Optional[SessionState]:
        if self.redis is None:
            return None

        try:
            raw = self.redis.get(self._key(user_id))
            if not raw:
                return None
            obj = json.loads(raw)
            ts = obj.get("last_proactive_msg")
            last_proactive = datetime.fromtimestamp(ts, tz=timezone.utc) if ts else None
            return SessionState(
                user_id=user_id,
                in_editor_mode=obj.get("in_editor_mode", False),
                just_self_corrected=obj.get("just_self_corrected", False),
                is_typing=obj.get("is_typing", False),
                last_proactive_msg=last_proactive,
            )
        except (redis.ConnectionError, redis.TimeoutError) as exc:
            logger.error("Failed to load session for %s: %s", user_id, exc)
            return None
        except (json.JSONDecodeError, KeyError, TypeError) as exc:
            logger.error("Failed to parse session data for %s: %s", user_id, exc)
            return None
        except Exception:  # noqa: BLE001
            logger.exception("Failed to load session for %s: unexpected error", user_id)
            return None

    def save(self, session: SessionState) -> None:
        if self.redis is None:
            return

        payload = {
            "in_editor_mode": session.in_editor_mode,
            "just_self_corrected": session.just_self_corrected,
            "is_typing": session.is_typing,
            "last_proactive_msg": (
                session.last_proactive_msg.timestamp() if session.last_proactive_msg else None
            ),
        }
        try:
            self.redis.setex(
                self._key(session.user_id),
                self.settings.session_ttl_seconds,
                json.dumps(payload),
            )
        except (redis.ConnectionError, redis.TimeoutError) as exc:
            logger.error("Failed to save session for %s: %s", session.user_id, exc)
        except (TypeError, ValueError) as exc:
            logger.error("Failed to serialize session for %s: %s", session.user_id, exc)
        except Exception:  # noqa: BLE001
            logger.exception("Failed to save session for %s: unexpected error", session.user_id)

    def close(self) -> None:
        if self.redis is not None:
            self.redis.close()
