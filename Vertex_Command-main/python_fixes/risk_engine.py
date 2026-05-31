import dataclasses
import time
from typing import Optional, Dict

@dataclasses.dataclass(frozen=True)
class RiskEvaluation:
    is_safe: bool
    action: str
    reason: Optional[str] = None

class RiskManager:
    def __init__(
        self,
        per_contract_spread_usd: float = 12.50,
        spread_multiplier: float = 1.5,
        entry_cooldown_sec: float = 3.0,
    ):
        self.per_contract_spread_usd = per_contract_spread_usd
        self.spread_multiplier = spread_multiplier
        self.entry_cooldown_sec = entry_cooldown_sec
        self._entry_timestamps: Dict[str, float] = {}

    def register_entry(self, position_key: str) -> None:
        self._entry_timestamps[position_key] = time.monotonic()

    def clear_entry(self, position_key: str) -> None:
        self._entry_timestamps.pop(position_key, None)

    def _is_within_cooldown(self, position_key: str) -> bool:
        entry_ts = self._entry_timestamps.get(position_key)
        if entry_ts is None:
            return False
        return (time.monotonic() - entry_ts) < self.entry_cooldown_sec

    def check_eod_drawdown(
        self,
        current_balance: float,
        eod_starting_balance: float,
        open_unrealized_pnl: float,
        max_drawdown_limit: float,
        open_position_qty: int = 1,
        position_key: Optional[str] = None,
    ) -> RiskEvaluation:

        if position_key and self._is_within_cooldown(position_key):
            return RiskEvaluation(
                is_safe=True,
                action="HOLD",
                reason=f"Entry cooldown active for {position_key}, skipping risk eval"
            )

        liquidation_threshold = eod_starting_balance - max_drawdown_limit
        current_equity = current_balance + open_unrealized_pnl
        deficit = liquidation_threshold - current_equity

        if deficit <= 0:
            return RiskEvaluation(is_safe=True, action="HOLD")

        dynamic_tolerance = self.per_contract_spread_usd * abs(open_position_qty) * self.spread_multiplier

        if deficit <= dynamic_tolerance and open_unrealized_pnl < 0:
            return RiskEvaluation(
                is_safe=True,
                action="HOLD",
                reason=(
                    f"Deficit ${deficit:.2f} within dynamic tolerance "
                    f"${dynamic_tolerance:.2f} (qty={open_position_qty})"
                ),
            )

        return RiskEvaluation(
            is_safe=False,
            action="FLATTEN",
            reason=(
                f"EOD Drawdown breached. Equity: ${current_equity:.2f}, "
                f"Threshold: ${liquidation_threshold:.2f}, Deficit: ${deficit:.2f}, "
                f"Tolerance: ${dynamic_tolerance:.2f}"
            ),
        )
