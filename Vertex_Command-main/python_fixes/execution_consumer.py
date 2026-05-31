import asyncio
import json
import logging
from typing import Dict, Any, List, Set, Optional

logger = logging.getLogger("execution_consumer")

class ExecutionConsumer:
    def __init__(
        self,
        event_queue: asyncio.Queue,
        slave_apis: List[Any],
        risk_manager: Any,
        tick_size: float = 0.25,
    ):
        self.event_queue = event_queue
        self.slave_apis = slave_apis
        self.risk_manager = risk_manager
        self.tick_size = tick_size
        self._processed_order_ids: Set[str] = set()
        self._active_master_orders: Dict[str, Dict[str, Any]] = {}
        self._max_dedup_cache = 10_000

    async def run(self) -> None:
        while True:
            event = await self.event_queue.get()
            try:
                await self._process_event(event)
            except Exception as e:
                logger.error(json.dumps({
                    "event": "consumer_error",
                    "error": str(e),
                    "event_type": event.get("type"),
                    "order_id": event.get("order_id"),
                }))
            finally:
                self.event_queue.task_done()

    async def _process_event(self, event: Dict[str, Any]) -> None:
        event_type = event.get("type")

        if event_type == "order_filled":
            await self._handle_fill(event)
        elif event_type == "order_cancelled":
            self._handle_cancel(event)
        elif event_type == "position_flattened":
            self._handle_flatten(event)

    async def _handle_fill(self, event: Dict[str, Any]) -> None:
        order_id = event.get("order_id")
        if not order_id:
            logger.warning(json.dumps({"event": "fill_missing_order_id", "raw": event}))
            return

        if order_id in self._processed_order_ids:
            logger.info(json.dumps({"event": "duplicate_fill_skipped", "order_id": order_id}))
            return
        self._mark_processed(order_id)

        is_entry = self._classify_fill_as_entry(event)

        logger.info(json.dumps({
            "event": "fill_classified",
            "order_id": order_id,
            "is_entry": is_entry,
            "parent_order_id": event.get("parent_order_id"),
            "execution_type": event.get("execution_type"),
            "direction": event.get("direction"),
            "symbol": event.get("symbol"),
        }))

        if not is_entry:
            logger.info(json.dumps({
                "event": "exit_fill_ignored",
                "order_id": order_id,
                "reason": "OCO/bracket exit fill — slaves manage own brackets",
            }))
            return

        master_fill_price = event.get("fill_price")
        direction = event.get("direction")

        if master_fill_price is None or direction is None:
            logger.error(json.dumps({
                "event": "fill_missing_fields",
                "order_id": order_id,
                "has_price": master_fill_price is not None,
                "has_direction": direction is not None,
            }))
            return

        master_sl_price = event.get("sl_price")
        master_tp_price = event.get("tp_price")

        sl_tick_offset: Optional[int] = None
        tp_tick_offset: Optional[int] = None

        if master_sl_price is not None:
            raw = ((master_fill_price - master_sl_price) * direction) / self.tick_size
            sl_tick_offset = round(raw)
            if sl_tick_offset <= 0:
                logger.warning(json.dumps({
                    "event": "sl_offset_invalid",
                    "order_id": order_id,
                    "raw_offset": raw,
                    "master_fill": master_fill_price,
                    "master_sl": master_sl_price,
                }))
                sl_tick_offset = None

        if master_tp_price is not None:
            raw = ((master_tp_price - master_fill_price) * direction) / self.tick_size
            tp_tick_offset = round(raw)
            if tp_tick_offset <= 0:
                logger.warning(json.dumps({
                    "event": "tp_offset_invalid",
                    "order_id": order_id,
                    "raw_offset": raw,
                    "master_fill": master_fill_price,
                    "master_tp": master_tp_price,
                }))
                tp_tick_offset = None

        symbol = event.get("symbol")
        qty = event.get("qty")
        position_key = f"{symbol}:{order_id}"

        self._active_master_orders[order_id] = {
            "symbol": symbol,
            "direction": direction,
            "qty": qty,
            "fill_price": master_fill_price,
        }

        if self.risk_manager:
            self.risk_manager.register_entry(position_key)

        tasks = [
            self._execute_slave(slave_api, event, sl_tick_offset, tp_tick_offset, direction)
            for slave_api in self.slave_apis
        ]
        results = await asyncio.gather(*tasks, return_exceptions=True)

        for i, result in enumerate(results):
            if isinstance(result, Exception):
                slave_id = getattr(self.slave_apis[i], "account_id", f"slave_{i}")
                logger.error(json.dumps({
                    "event": "slave_task_exception",
                    "slave_id": slave_id,
                    "error": str(result),
                    "order_id": order_id,
                }))

    def _classify_fill_as_entry(self, event: Dict[str, Any]) -> bool:
        exec_type = event.get("execution_type", "").lower()
        if exec_type in ("bracket_close", "oco_fill", "stop_loss", "take_profit", "liquidation"):
            return False

        parent_id = event.get("parent_order_id")
        if parent_id and parent_id in self._active_master_orders:
            return False

        if event.get("is_closing_order") is True:
            return False

        return True

    def _handle_cancel(self, event: Dict[str, Any]) -> None:
        order_id = event.get("order_id")
        logger.info(json.dumps({
            "event": "order_cancelled_received",
            "order_id": order_id,
            "cancel_reason": event.get("cancel_reason", "unknown"),
            "related_order_id": event.get("related_order_id"),
        }))

    def _handle_flatten(self, event: Dict[str, Any]) -> None:
        logger.info(json.dumps({
            "event": "position_flattened_received",
            "account_id": event.get("account_id"),
            "symbol": event.get("symbol"),
            "initiator": event.get("initiator", "unknown"),
        }))

    async def _execute_slave(
        self,
        slave_api: Any,
        event: Dict[str, Any],
        sl_tick_offset: Optional[int],
        tp_tick_offset: Optional[int],
        direction: int,
    ) -> None:
        slave_id = getattr(slave_api, "account_id", "unknown")
        order_id = event.get("order_id")

        try:
            fill_response = await slave_api.execute_market_order(
                symbol=event.get("symbol"),
                qty=event.get("qty"),
                direction=direction,
            )

            slave_fill_price = fill_response.get("fill_price")
            if slave_fill_price is None:
                logger.error(json.dumps({
                    "event": "slave_fill_no_price",
                    "slave_id": slave_id,
                    "raw_response": fill_response,
                }))
                return

            logger.info(json.dumps({
                "event": "slave_entry_filled",
                "slave_id": slave_id,
                "order_id": order_id,
                "slave_fill_price": slave_fill_price,
                "master_fill_price": event.get("fill_price"),
                "slippage": round(slave_fill_price - event.get("fill_price", 0), 4),
            }))

            slave_sl: Optional[float] = None
            slave_tp: Optional[float] = None

            if sl_tick_offset is not None:
                slave_sl = slave_fill_price - (sl_tick_offset * self.tick_size * direction)
            if tp_tick_offset is not None:
                slave_tp = slave_fill_price + (tp_tick_offset * self.tick_size * direction)

            if slave_sl is not None or slave_tp is not None:
                bracket_payload = {
                    "sl_price": slave_sl,
                    "tp_price": slave_tp,
                    "parent_id": fill_response.get("order_id"),
                }
                bracket_response = await slave_api.attach_brackets(bracket_payload)

                logger.info(json.dumps({
                    "event": "slave_brackets_attached",
                    "slave_id": slave_id,
                    "order_id": order_id,
                    "bracket_request": bracket_payload,
                    "bracket_response": bracket_response,
                }))

        except Exception as e:
            error_detail = getattr(e, "response_payload", None)
            logger.error(json.dumps({
                "event": "slave_execution_rejected",
                "slave_id": slave_id,
                "order_id": order_id,
                "error": str(e),
                "error_type": type(e).__name__,
                "response_payload": error_detail if error_detail else None,
                "cancelReason": error_detail.get("cancelReason") if isinstance(error_detail, dict) else None,
                "initiator": error_detail.get("initiator") if isinstance(error_detail, dict) else None,
            }))

    def _mark_processed(self, order_id: str) -> None:
        self._processed_order_ids.add(order_id)
        if len(self._processed_order_ids) > self._max_dedup_cache:
            to_remove = list(self._processed_order_ids)[:self._max_dedup_cache // 2]
            for oid in to_remove:
                self._processed_order_ids.discard(oid)
