"""RAZE AI investigation service.

Advisory only. All authoritative financial mutations happen in the Go control
plane. This service fails closed: any error results in a non-2xx response and
the control plane routes the item to human review.
"""

from __future__ import annotations

from typing import Any

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

from . import service

app = FastAPI(title="RAZE AI investigation service", version="0.1.0")


class AdviseRequest(BaseModel):
    item_id: int
    record_id: int
    decision: str
    confidence: float = 0.0
    candidates: list[dict[str, Any]] = []
    evidence: list[dict[str, Any]] = []
    reasons: list[str] = []
    investigation: dict[str, Any] = {}


class AdviseResponse(BaseModel):
    recommendation: str
    confidence: float
    investigation: dict[str, Any]
    model_version: str


@app.get("/healthz")
def healthz() -> dict[str, str]:
    return {"status": "ok"}


@app.post("/advise", response_model=AdviseResponse)
def advise(req: AdviseRequest) -> AdviseResponse:
    try:
        return AdviseResponse(**service.advise(req.model_dump()))
    except Exception as exc:  # fail closed
        raise HTTPException(status_code=500, detail=f"investigation failed: {exc}") from exc
