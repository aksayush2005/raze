#!/usr/bin/env python3
"""Evaluate a reconciliation run against synthetic ground truth.

Usage:
    python3 data/generator/evaluate.py --base http://localhost:8080 --job 1 \
        --truth data/benchmark/ground_truth.json

Ground truth maps payment external_id -> expected settlement external_id
(or null when the settlement is intentionally missing).
"""

from __future__ import annotations

import argparse
import json
import urllib.request
from typing import Any


def get(url: str) -> Any:
    with urllib.request.urlopen(url) as resp:
        return json.load(resp)


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--base", default="http://localhost:8080")
    ap.add_argument("--job", type=int, required=True)
    ap.add_argument("--truth", required=True)
    args = ap.parse_args()

    truth: dict[str, str | None] = json.loads(open(args.truth).read())

    payload = get(f"{args.base}/api/v1/jobs/{args.job}/items?limit=1000")["items"]
    counts = {"RECONCILED": 0, "REVIEW": 0, "ESCALATED": 0}
    correct = 0
    reconciled_total = 0
    for item in payload:
        detail = get(f"{args.base}/api/v1/items/{item['id']}")
        rec = detail["record"]
        if rec["kind"] != "payment":
            continue
        status = detail["item"]["status"]
        counts[status] = counts.get(status, 0) + 1
        expected = truth.get(rec["external_id"])
        actual = None
        if detail.get("match_record"):
            actual = detail["match_record"]["external_id"]
        if status == "RESOLVED":
            reconciled_total += 1
            if actual == expected:
                correct += 1

    print(f"payments by outcome: {json.dumps(counts)}")
    if reconciled_total:
        precision = correct / reconciled_total
        print(f"reconciled precision (correct / auto-reconciled): {precision:.3f} ({correct}/{reconciled_total})")
    else:
        print("no auto-reconciled payments to evaluate")


if __name__ == "__main__":
    main()
