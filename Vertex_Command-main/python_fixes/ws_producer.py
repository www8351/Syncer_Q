import asyncio
import json
import logging
from typing import Set, Dict, Any
import aiohttp

logger = logging.getLogger("ws_producer")

class WebSocketProducer:
    def __init__(self, ws_url: str, auth_token: str, event_queue: asyncio.Queue):
        self.ws_url = ws_url
        self.auth_token = auth_token
        self.event_queue = event_queue
        self.valid_events: Set[str] = {
            "order_filled",
            "order_pending",
            "order_cancelled",
            "position_flattened",
        }
        self._max_retries = 10
        self._seen_event_ids: Set[str] = set()
        self._max_dedup_cache = 10_000

    async def run(self) -> None:
        retry_count = 0
        backoff = 1.0

        async with aiohttp.ClientSession() as session:
            while retry_count < self._max_retries:
                try:
                    async with session.ws_connect(
                        self.ws_url,
                        headers={"Authorization": f"Bearer {self.auth_token}"},
                        heartbeat=30.0,
                    ) as ws:
                        logger.info(json.dumps({"event": "ws_connected", "url": self.ws_url}))
                        retry_count = 0
                        backoff = 1.0
                        await self._listen(ws)
                except aiohttp.ClientError as e:
                    logger.error(json.dumps({"event": "ws_connection_error", "error": str(e)}))
                except asyncio.CancelledError:
                    logger.info(json.dumps({"event": "ws_producer_cancelled"}))
                    return
                except Exception as e:
                    logger.critical(json.dumps({"event": "ws_critical_error", "error": str(e)}))

                retry_count += 1
                logger.warning(json.dumps({
                    "event": "ws_reconnecting",
                    "attempt": retry_count,
                    "delay": backoff,
                }))
                await asyncio.sleep(backoff)
                backoff = min(backoff * 2, 60.0)

        logger.error(json.dumps({"event": "ws_max_retries_exhausted", "retries": self._max_retries}))

    async def _listen(self, ws: aiohttp.ClientWebSocketResponse) -> None:
        async for msg in ws:
            if msg.type == aiohttp.WSMsgType.TEXT:
                try:
                    data: Dict[str, Any] = json.loads(msg.data)
                    event_type = data.get("type")

                    if event_type not in self.valid_events:
                        continue

                    event_id = data.get("event_id") or data.get("order_id")
                    if event_id:
                        dedup_key = f"{event_type}:{event_id}"
                        if dedup_key in self._seen_event_ids:
                            logger.debug(json.dumps({
                                "event": "ws_duplicate_skipped",
                                "dedup_key": dedup_key,
                            }))
                            continue
                        self._seen_event_ids.add(dedup_key)
                        if len(self._seen_event_ids) > self._max_dedup_cache:
                            to_remove = list(self._seen_event_ids)[:self._max_dedup_cache // 2]
                            for key in to_remove:
                                self._seen_event_ids.discard(key)

                    logger.info(json.dumps({
                        "event": "ws_event_queued",
                        "type": event_type,
                        "order_id": data.get("order_id"),
                        "parent_order_id": data.get("parent_order_id"),
                        "execution_type": data.get("execution_type"),
                        "symbol": data.get("symbol"),
                    }))

                    await self.event_queue.put(data)

                except json.JSONDecodeError:
                    logger.error(json.dumps({
                        "event": "ws_decode_error",
                        "raw_payload": msg.data[:500],
                    }))
            elif msg.type in (aiohttp.WSMsgType.CLOSED, aiohttp.WSMsgType.ERROR):
                logger.warning(json.dumps({
                    "event": "ws_closed_or_error",
                    "msg_type": str(msg.type),
                }))
                break
