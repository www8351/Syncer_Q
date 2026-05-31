"""
Targeted diagnostic: intercepts and logs the exact broker API payload
during any close/flatten event to determine if the close was initiated
by the server (risk engine / OCO logic) or by the broker itself.

Wire this into your slave API client as a decorator or middleware.
"""
import json
import logging
import time
import functools
from typing import Any, Callable, Dict, Optional

logger = logging.getLogger("close_inspector")

CLOSE_INDICATORS = {
    "cancelReason", "cancel_reason", "closeReason", "close_reason",
    "initiator", "initiated_by", "rejection_text", "rejectionText",
    "liquidation_reason", "flatten_source", "order_action",
}

class CloseEventInspector:
    def __init__(self):
        self._close_log: list = []

    def inspect_response(
        self,
        slave_id: str,
        action: str,
        request_payload: Dict[str, Any],
        response_payload: Any,
        elapsed_ms: float,
    ) -> None:
        entry = {
            "timestamp": time.time(),
            "slave_id": slave_id,
            "action": action,
            "elapsed_ms": round(elapsed_ms, 2),
            "request": request_payload,
        }

        if isinstance(response_payload, dict):
            entry["response"] = response_payload
            close_fields = {}
            for key in CLOSE_INDICATORS:
                if key in response_payload:
                    close_fields[key] = response_payload[key]
            if close_fields:
                entry["close_indicators"] = close_fields

            is_broker_initiated = any(
                response_payload.get(k) in ("broker", "exchange", "risk_desk", "liquidation", "margin_call")
                for k in ("initiator", "initiated_by", "flatten_source")
            )
            is_server_initiated = any(
                response_payload.get(k) in ("server", "client", "api", "user", "system")
                for k in ("initiator", "initiated_by", "flatten_source")
            )

            if is_broker_initiated:
                entry["verdict"] = "BROKER_INITIATED"
            elif is_server_initiated:
                entry["verdict"] = "SERVER_INITIATED"
            else:
                entry["verdict"] = "UNKNOWN_SOURCE"
        else:
            entry["response_raw"] = str(response_payload)[:1000]
            entry["verdict"] = "UNPARSEABLE"

        self._close_log.append(entry)
        logger.warning(json.dumps({"event": "close_event_inspected", **entry}))

    def get_recent_closes(self, n: int = 20) -> list:
        return self._close_log[-n:]


_inspector = CloseEventInspector()


def intercept_closes(func: Callable) -> Callable:
    """
    Decorator for slave API methods that may close positions.
    Wraps execute_market_order, flatten_position, cancel_order, etc.

    Usage:
        slave_api.flatten_position = intercept_closes(slave_api.flatten_position)
    """
    @functools.wraps(func)
    async def wrapper(*args, **kwargs):
        slave_id = "unknown"
        if args and hasattr(args[0], "account_id"):
            slave_id = getattr(args[0], "account_id", "unknown")

        request_info = {"args": str(args[1:])[:500], "kwargs": {k: str(v)[:200] for k, v in kwargs.items()}}
        t0 = time.monotonic()

        try:
            result = await func(*args, **kwargs)
            elapsed = (time.monotonic() - t0) * 1000
            _inspector.inspect_response(slave_id, func.__name__, request_info, result, elapsed)
            return result
        except Exception as e:
            elapsed = (time.monotonic() - t0) * 1000
            error_payload = getattr(e, "response_payload", {"error": str(e), "type": type(e).__name__})
            _inspector.inspect_response(slave_id, func.__name__, request_info, error_payload, elapsed)
            raise

    return wrapper


def get_inspector() -> CloseEventInspector:
    return _inspector
