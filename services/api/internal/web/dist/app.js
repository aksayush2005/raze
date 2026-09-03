/* RAZE operations interface — workflow-first, reads authoritative API only. */
"use strict";

const $ = (id) => document.getElementById(id);
const app = $("app");
const fmt = new Intl.NumberFormat("en-IN", { style: "currency", currency: "INR" });

function money(minor) {
  return fmt.format((minor ?? 0) / 100);
}

async function api(path, opts = {}) {
  const res = await fetch(path, {
    headers: { "Content-Type": "application/json", ...(opts.headers || {}) },
    ...opts,
  });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(body.error || `HTTP ${res.status}`);
  return body;
}

function esc(s) {
  return String(s ?? "").replace(/[&<>"']/g, (c) => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
  }[c]));
}

function pill(status) {
  return `<span class="pill ${esc(status)}">${esc(status)}</span>`;
}

function time(iso) {
  return iso ? new Date(iso).toLocaleString() : "—";
}

/* ---------- routing ---------- */
window.addEventListener("hashchange", route);
async function route() {
  const hash = location.hash || "#jobs";
  const parts = hash.slice(1).split("/").filter(Boolean);
  try {
    if (parts[0] === "records") return renderRecords();
    if (parts[0] === "items" && parts[1]) return renderItem(parts[1]);
    if (parts[0] === "jobs" && parts[1]) return renderJob(parts[1]);
    return renderJobs();
  } catch (err) {
    app.innerHTML = `<div class="card error">${esc(err.message)}</div>`;
  }
}

/* ---------- jobs ---------- */
async function renderJobs() {
  const { jobs } = await api("/api/v1/jobs?limit=50");
  app.innerHTML = `
    <div class="toolbar">
      <button onclick="createJob()">Run reconciliation</button>
      <label class="btn ghost">Import records (JSON)
        <input type="file" accept="application/json" class="hidden" onchange="importRecords(event)">
      </label>
      <button class="ghost" onclick="syncRazorpay()">Sync Razorpay (test)</button>
    </div>
    <div class="card">
      <h2>Reconciliation jobs</h2>
      <table>
        <thead><tr><th>ID</th><th>Name</th><th>Status</th><th>Records</th>
        <th>Matched</th><th>Review</th><th>Escalated</th><th>Created</th></tr></thead>
        <tbody>
          ${jobs.length ? jobs.map(j => `
            <tr onclick="location.hash='#jobs/${j.id}'">
              <td class="mono">#${j.id}</td><td>${esc(j.name)}</td>
              <td>${pill(j.status)}</td><td>${j.total_records}</td>
              <td class="ok">${j.matched}</td><td class="warn">${j.review}</td>
              <td class="danger">${j.escalated}</td><td class="muted">${time(j.created_at)}</td>
            </tr>`).join("")
            : `<tr><td colspan="8" class="muted">No jobs yet. Import records and run a reconciliation.</td></tr>`}
        </tbody>
      </table>
    </div>`;
}

async function createJob() {
  const { id } = await api("/api/v1/jobs", {
    method: "POST",
    body: JSON.stringify({ name: "reconciliation", config: {} }),
  });
  location.hash = `#jobs/${id}`;
}

async function importRecords(evt) {
  const file = evt.target.files[0];
  if (!file) return;
  const text = await file.text();
  const { imported } = await api("/api/v1/records/import", {
    method: "POST",
    headers: { "Idempotency-Key": `import-${Date.now()}` },
    body: text,
  });
  alert(`Imported ${imported} records`);
  route();
}

async function syncRazorpay() {
  try {
    const r = await api("/api/v1/records/sync/razorpay", { method: "POST" });
    alert(`Imported ${r.imported} Razorpay test-mode settlements`);
  } catch (err) {
    alert(err.message);
  }
}

/* ---------- job detail ---------- */
async function renderJob(id) {
  const job = await api(`/api/v1/jobs/${id}`);
  const status = new URLSearchParams(location.hash.split("?")[1]).get("s") || "";
  const { items } = await api(`/api/v1/jobs/${id}/items?limit=500&status=${status}`);

  const filters = ["", "RESOLVED", "REVIEW", "ESCALATED"]
    .map(s => `<button class="ghost ${s === status ? "ok" : ""}" onclick="location.hash='#jobs/${id}?s=${s}'">${s || "ALL"}</button>`)
    .join(" ");

  app.innerHTML = `
    <div class="toolbar"><button class="ghost" onclick="location.hash='#jobs'">← Jobs</button></div>
    <div class="card">
      <h2>Job #${job.id} — ${esc(job.name)}</h2>
      <div class="stat-row">
        <div class="stat"><div class="num">${pill(job.status)}</div><div class="label">status</div></div>
        <div class="stat"><div class="num">${job.total_records}</div><div class="label">records</div></div>
        <div class="stat"><div class="num" style="color:var(--ok)">${job.matched}</div><div class="label">matched</div></div>
        <div class="stat"><div class="num" style="color:var(--warn)">${job.review}</div><div class="label">review</div></div>
        <div class="stat"><div class="num" style="color:var(--danger)">${job.escalated}</div><div class="label">escalated</div></div>
      </div>
    </div>
    <div class="card">
      <div class="toolbar">${filters}</div>
      <table>
        <thead><tr><th>Item</th><th>Record</th><th>Kind</th><th>Amount</th><th>Status</th><th>Confidence</th><th>Decision</th></tr></thead>
        <tbody>
          ${items.length ? items.map(it => `
            <tr onclick="location.hash='#items/${it.id}'">
              <td class="mono">#${it.id}</td><td class="mono">${esc(it.record_external || "")}</td>
              <td>${esc(it.kind || "")}</td><td>${money(it.amount_minor)}</td>
              <td>${pill(it.status)}</td>
              <td>${it.confidence == null ? "—" : (it.confidence * 100).toFixed(0) + "%"}</td>
              <td>${esc(it.decision || "—")}</td>
            </tr>`).join("")
            : `<tr><td colspan="7" class="muted">No items in this filter.</td></tr>`}
        </tbody>
      </table>
    </div>`;
}

/* ---------- item workspace ---------- */
async function renderItem(id) {
  const d = await api(`/api/v1/items/${id}`);
  const it = d.item;
  const rec = d.record;

  const candidates = (d.candidates || []).map(c => `
    <li><span class="mono">#${c.target_record_id}</span>
      <span class="tag">${esc(c.strategy)}</span> sim=${(c.similarity * 100).toFixed(0)}% ·
      <span class="tag">${esc(c.status)}</span></li>`).join("");

  const evidence = (d.evidence || []).map(e => `
    <li><span class="type">${esc(e.type)}</span>
      ${e.weight ? `<span class="tag">w=${e.weight}</span>` : ""}
      <div class="muted mono">${esc(JSON.stringify(e.details))}</div></li>`).join("");

  const ai = (d.ai_decisions || []).map(a => `
    <li><b>${esc(a.recommendation)}</b> · ${(a.confidence * 100).toFixed(0)}% ·
      <span class="tag">${esc(a.model_version)}</span>
      <div class="muted mono">${esc(JSON.stringify(a.investigation))}</div></li>`).join("") || "<li class=\"muted\">No AI investigation run.</li>";

  const audit = (d.audit || []).map(a => `
    <li><span class="tag">${time(a.created_at)}</span>
      <b>${esc(a.action)}</b> <span class="muted">by ${esc(a.actor)}</span>
      <div class="muted mono">${esc(JSON.stringify(a.metadata))}</div></li>`).join("");

  const reviewable = ["REVIEW", "ESCALATED"].includes(it.status);

  app.innerHTML = `
    <div class="toolbar"><button class="ghost" onclick="history.back()">← Back</button></div>
    <div class="grid">
      <div class="card">
        <h2>Case #${it.id} ${pill(it.status)}</h2>
        <dl class="kv">
          <dt>Record</dt><dd class="mono">${esc(rec.external_id)} <span class="tag">${esc(rec.kind)}</span>
            ${rec.is_synthetic ? '<span class="tag">synthetic</span>' : '<span class="tag">provider</span>'}</dd>
          <dt>Amount</dt><dd>${money(rec.amount_minor)} <span class="tag">${esc(rec.currency)}</span></dd>
          <dt>Fee / Tax / Net</dt><dd class="mono">${money(rec.fee_minor)} / ${money(rec.tax_minor)} / ${money(rec.net_minor)}</dd>
          <dt>Occurred</dt><dd>${time(rec.occurred_at)}</dd>
          <dt>Matched</dt><dd>${d.match_record ? `<span class="mono">${esc(d.match_record.external_id)}</span> ${money(d.match_record.amount_minor)}` : "—"}</dd>
          <dt>Confidence</dt><dd>${it.confidence == null ? "—" : (it.confidence * 100).toFixed(1) + "%"}</dd>
          <dt>Decision</dt><dd>${esc(it.decision || "—")}</dd>
        </dl>
      </div>

      <div class="card">
        <h3>Candidates</h3>
        <ul class="evidence">${candidates || '<li class="muted">No candidates generated.</li>'}</ul>
      </div>

      <div class="card">
        <h3>Evidence</h3>
        <ul class="evidence">${evidence || '<li class="muted">No evidence recorded.</li>'}</ul>
      </div>

      <div class="card">
        <h3>AI investigation <span class="tag">advisory</span></h3>
        <ul class="evidence">${ai}</ul>
      </div>

      <div class="card">
        <h3>Audit trail</h3>
        <ul class="evidence">${audit || '<li class="muted">No events.</li>'}</ul>
      </div>

      ${reviewable ? `
      <div class="card">
        <h3>Human review</h3>
        <div class="toolbar">
          <button class="ok" onclick="review(${it.id}, 'ACCEPTED_AGENT_MATCH')">Accept match</button>
          <button class="warn" onclick="review(${it.id}, 'REJECTED_CANDIDATE')">Reject candidate</button>
          <button class="danger" onclick="review(${it.id}, 'ESCALATED')">Escalate</button>
          <button class="ghost" onclick="review(${it.id}, 'CONFIRMED_EXCEPTION')">Confirm exception</button>
        </div>
        <div class="toolbar">
          <input id="manual-target" class="mono" style="flex:1" placeholder="target_record_id for manual link" type="number">
          <button onclick="manualLink(${it.id})">Manual link</button>
        </div>
      </div>` : ""}
    </div>`;
}

async function review(itemId, action) {
  await api(`/api/v1/items/${itemId}/review`, {
    method: "POST",
    body: JSON.stringify({ action, actor: "operator@demo" }),
  });
  route();
}

async function manualLink(itemId) {
  const target = $("manual-target").value;
  await api(`/api/v1/items/${itemId}/review`, {
    method: "POST",
    body: JSON.stringify({ action: "MANUALLY_LINKED_RECORDS", actor: "operator@demo", target_record_id: Number(target) }),
  });
  route();
}

/* ---------- records ---------- */
async function renderRecords() {
  const data = await api("/api/v1/records?limit=200");
  app.innerHTML = `
    <div class="card">
      <h2>Records</h2>
      <table>
        <thead><tr><th>ID</th><th>External</th><th>Kind</th><th>Amount</th><th>Fee</th><th>Tax</th><th>Net</th><th>Ref</th><th>Source</th></tr></thead>
        <tbody>
          ${(data.records || []).map(r => `
            <tr><td class="mono">${r.id}</td><td class="mono">${esc(r.external_id)}</td>
              <td>${esc(r.kind)}</td><td>${money(r.amount_minor)}</td><td>${money(r.fee_minor)}</td>
              <td>${money(r.tax_minor)}</td><td>${money(r.net_minor)}</td>
              <td class="mono">${esc(r.ref_external_id || "")}</td>
              <td><span class="tag">${r.is_synthetic ? "synthetic" : "provider"}</span></td></tr>`).join("")}
        </tbody>
      </table>
    </div>`;
}

route();
