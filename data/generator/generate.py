#!/usr/bin/env python3
"""RAZE synthetic financial data generator.

Produces payments + settlements with known ground truth and applies controlled
corruption classes so the reconciliation engine and AI layer can be evaluated.

All amounts are integer minor units (paise). No floating point.
"""

from __future__ import annotations

import argparse
import json
import random
from datetime import datetime, timedelta, timezone
from pathlib import Path

KINDS_PAYMENT = "payment"
KINDS_SETTLEMENT = "settlement"


def _ts(dt: datetime) -> str:
    return dt.astimezone(timezone.utc).isoformat()


def round_minor(amount_minor: int) -> int:
    """Round to whole rupees (multiples of 100 paise) for realism."""
    return (amount_minor // 100) * 100


def apply_fee_tax(gross_minor: int, fee_bps: int = 150, tax_bps: int = 1800):
    """fee = fee_bps/10000 of gross; tax = tax_bps/10000 of fee; net = gross - fee - tax."""
    fee = round_minor(gross_minor * fee_bps // 10000)
    tax = round_minor(fee * tax_bps // 10000)
    return fee, tax, gross_minor - fee - tax


class Generator:
    def __init__(self, seed: int):
        self.rng = random.Random(seed)
        self.settlement_seq = 1
        self.payment_seq = 1

    def _next_settlement(self) -> str:
        s = f"SET_{self.settlement_seq:05d}"
        self.settlement_seq += 1
        return s

    def _next_payment(self) -> str:
        p = f"PAY_{self.payment_seq:05d}"
        self.payment_seq += 1
        return p

    def _payment_amount(self) -> int:
        return round_minor(self.rng.randint(5000, 5000000))

    def _time(self, base: datetime, max_hours: int = 0) -> datetime:
        return base + timedelta(hours=self.rng.randint(-max_hours, max_hours))

    # -- corruption helpers -------------------------------------------------

    def _corrupt_amount(self, amount: int) -> int:
        delta = self.rng.choice([-500, -200, 100, 300, 700])
        return max(0, amount + delta)

    def _shift_date(self, dt: datetime) -> datetime:
        return dt + timedelta(days=self.rng.choice([-10, 12, 20]))

    def _break_ref(self, ref: str) -> str | None:
        if self.rng.random() < 0.5:
            return None
        return f"UNKNOWN_{self.rng.randint(10000, 99999)}"

    def generate(self, n_settlements: int, corruption_rate: float) -> list[dict]:
        records: list[dict] = []
        ground_truth: dict[str, str | None] = {}
        base = datetime(2026, 8, 1, tzinfo=timezone.utc)
        batch_sizes = [1] * 6 + [2, 2, 3, 4]  # mostly 1:1, some N:1 batches

        for _ in range(n_settlements):
            set_id = self._next_settlement()
            set_time = self._time(base, max_hours=72)
            batch = self.rng.choice(batch_sizes)
            payments: list[dict] = []
            for _ in range(batch):
                pay_id = self._next_payment()
                amt = self._payment_amount()
                payments.append({
                    "source": "synthetic_payments",
                    "is_synthetic": True,
                    "external_id": pay_id,
                    "kind": KINDS_PAYMENT,
                    "status": "active",
                    "amount_minor": amt,
                    "fee_minor": 0,
                    "tax_minor": 0,
                    "net_minor": amt,  # no fee/tax on payments
                    "currency": "INR",
                    "occurred_at": _ts(self._time(set_time, max_hours=48)),
                    "ref_external_id": set_id,
                })
                ground_truth[pay_id] = set_id

            gross = sum(p["amount_minor"] for p in payments)
            fee, tax, net = apply_fee_tax(gross)

            settlement = {
                "source": "synthetic_settlements",
                "is_synthetic": True,
                "external_id": set_id,
                "kind": KINDS_SETTLEMENT,
                "status": "active",
                "amount_minor": gross,
                "fee_minor": fee,
                "tax_minor": tax,
                "net_minor": net,  # clean net BEFORE any corruption below
                "currency": "INR",
                "occurred_at": _ts(set_time),
                "ref_external_id": None,
            }

            # Decide corruption class for this group.
            roll = self.rng.random()
            if roll < corruption_rate:
                cls = self.rng.choice([
                    "amount_mismatch", "missing_settlement", "duplicate_transaction",
                    "date_shift", "reference_corruption", "fee_discrepancy", "tax_discrepancy",
                ])
            else:
                cls = "clean"

            apply_cls = {
                "amount_mismatch": lambda: settlement.update(
                    amount_minor=self._corrupt_amount(gross)),
                "missing_settlement": lambda: ground_truth.update({p["external_id"]: None for p in payments}),
                "duplicate_transaction": lambda: records.append(self._duplicate_payment(payments[0], set_id)),
                "date_shift": lambda: [p.update(occurred_at=_ts(self._shift_date(datetime.fromisoformat(p["occurred_at"])))) for p in payments],
                "reference_corruption": lambda: [p.update(ref_external_id=self._break_ref(set_id)) for p in payments],
                "fee_discrepancy": lambda: settlement.update(fee_minor=fee - 250),
                "tax_discrepancy": lambda: settlement.update(tax_minor=tax + 180),
            }

            if cls == "missing_settlement":
                # The settlement record itself is omitted.
                for p in payments:
                    p["ref_external_id"] = set_id  # keep the dangling reference
                records.extend(payments)
                ground_truth.update({p["external_id"]: None for p in payments})
                continue
            if cls in apply_cls:
                apply_cls[cls]()

            records.extend(payments)
            records.append(settlement)

        return records, ground_truth

    def _duplicate_payment(self, src: dict, set_id: str) -> dict:
        dup = dict(src)
        dup["external_id"] = f"{src['external_id']}D"
        dup["ref_external_id"] = set_id
        return dup


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--n-settlements", type=int, default=200)
    ap.add_argument("--corruption-rate", type=float, default=0.30)
    ap.add_argument("--seed", type=int, default=42)
    ap.add_argument("--out", type=Path, default=Path("data/benchmark/records.json"))
    args = ap.parse_args()

    gen = Generator(args.seed)
    records, truth = gen.generate(args.n_settlements, args.corruption_rate)

    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps({"records": records}, indent=2))

    truth_path = args.out.with_name("ground_truth.json")
    truth_path.write_text(json.dumps(truth, indent=2))

    n_pay = sum(1 for r in records if r["kind"] == KINDS_PAYMENT)
    n_set = sum(1 for r in records if r["kind"] == KINDS_SETTLEMENT)
    print(f"wrote {len(records)} records to {args.out}")
    print(f"  payments={n_pay} settlements={n_set} ground_truth={len(truth)}")
    print(f"wrote ground truth to {truth_path}")


if __name__ == "__main__":
    main()
