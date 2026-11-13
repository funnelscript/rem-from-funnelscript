"use strict";

const $ = id => document.getElementById(id);
const labels = {
  evidence: "Evidence only", anthropic: "Anthropic API", "claude-code": "Claude Code", codex: "Codex",
  website: "Website", seo: "SEO", history: "History", apps: "App stores",
  paid: "Paid media", social: "Organic social", founders: "Founders", community: "Community",
  geo: "AI search", paid_meta: "Meta ads", paid_apple: "Apple ads", paid_google: "Google ads",
  paid_linkedin: "LinkedIn ads", paid_tiktok: "TikTok ads", paid_reddit: "Reddit ads",
  appstore: "App Store", playstore: "Google Play", llms: "LLMs.txt",
};
const defaultAreas = ["website", "seo", "history", "apps", "paid", "social", "founders", "community"];
const panels = ["overview", "report", "evidence", "content", "unknowns"];
const label = key => labels[key] || String(key || "").replaceAll("_", " ").replace(/^./, c => c.toUpperCase());
const date = iso => {
  const d = new Date(iso);
  return iso && Number.isFinite(d.getTime()) ? new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" }).format(d) : "";
};

let current;
let currentJob = "";
let renderEpoch = 0;

function el(tag, text, className) {
  const node = document.createElement(tag);
  if (text != null) node.textContent = text;
  if (className) node.className = className;
  return node;
}

function httpURL(raw) {
  try {
    const url = new URL(raw);
    return (url.protocol === "https:" || url.protocol === "http:") && !url.username && !url.password ? url.href : "";
  } catch {
    return "";
  }
}

function external(raw, text) {
  const url = httpURL(raw);
  if (!url) return null;
  const a = el("a", text);
  a.href = url;
  a.target = "_blank";
  a.rel = "noopener noreferrer";
  return a;
}

function empty(text) {
  return el("p", text, "empty");
}

function notice(text) {
  $("notice").textContent = text || "";
  $("notice").hidden = !text;
}

async function api(path, options) {
  const response = await fetch(path, options);
  const result = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(result.error || `Request failed (${response.status})`);
  return result;
}

function post(path, data) {
  return api(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });
}

function configureAreas(areas) {
  const field = $("areas");
  for (const old of field.querySelectorAll("label")) old.remove();
  for (const area of areas) {
    const item = el("label");
    const input = el("input");
    input.type = "checkbox";
    input.value = area;
    input.checked = true;
    input.disabled = area === "website";
    item.append(input, document.createTextNode(label(area)));
    field.append(item);
  }
}

async function refreshStatus() {
  const info = await api("/api/status");
  const select = $("engine");
  const existing = select.value;
  const engineList = ["evidence", "codex", "claude-code", "anthropic"];
  select.replaceChildren();
  for (const id of engineList) {
    const available = id === "evidence" || !!info.engines?.[id];
    const option = new Option(label(id) + (available ? "" : " · unavailable"), id, false, id === existing && available);
    option.disabled = !available;
    select.add(option);
  }
  if (Array.isArray(info.areas) && info.areas.length) configureAreas(info.areas);
  const list = el("ul");
  list.append(el("li", "Public sources: available"));
  list.append(el("li", `Scrape Creators: ${info.scrapeCreators ? "configured" : "set SCRAPECREATORS_API_KEY to check paid sources"}`));
  for (const id of engineList.slice(1)) list.append(el("li", `${label(id)}: ${info.engines?.[id] ? "available" : "unavailable"}`));
  $("status-details").replaceChildren(list);
}

async function refreshHistory() {
  const target = $("history");
  try {
    const runs = await api("/api/runs");
    if (!runs.length) {
      target.replaceChildren(empty("No saved investigations yet."));
      return;
    }
    const list = el("ul", null, "list");
    for (const run of runs) {
      const a = el("a", null, "history-item");
      a.href = `#/run/${encodeURIComponent(run.id)}`;
      const identity = el("span");
      identity.append(el("strong", run.brand || run.domain || run.id), document.createTextNode(run.domain || ""));
      const meta = el("span", [date(run.generatedAt), `${run.evidenceCount ?? 0} evidence`].filter(Boolean).join(" · "));
      a.append(identity, meta);
      const li = el("li");
      li.append(a);
      list.append(li);
    }
    target.replaceChildren(list);
  } catch (err) {
    target.replaceChildren(empty(err.message));
  }
}

function getHandles(raw) {
  const result = Object.create(null);
  for (const part of raw.split(",")) {
    if (!part.trim()) continue;
    const pair = part.match(/^\s*(tiktok|instagram|youtube|x|linkedin|reddit|threads|producthunt|facebook)\s*=\s*([^=,]+?)\s*$/);
    if (!pair) throw new Error("Use platform=handle for each social handle.");
    result[pair[1]] = pair[2];
  }
  return result;
}

async function begin(event) {
  event.preventDefault();
  if (currentJob || $("start").disabled) return;
  notice("");
  const url = $("target").value.trim();
  const selected = [...$("areas").querySelectorAll("input:checked")].map(input => input.value);
  const body = { url, engine: $("engine").value };
  if (selected.length) body.only = selected;
  const credits = $("credits").value;
  if (credits !== "") body.maxCredits = Number(credits);
  const model = $("model").value.trim();
  if (model && body.engine !== "evidence") body.model = model;
  try {
    body.handles = getHandles($("handles").value);
    $("start").disabled = true;
    const { jobId } = await post("/api/runs", body);
    currentJob = jobId;
    $("progress").hidden = false;
    $("progress-title").textContent = "Investigation in progress";
    $("progress-message").textContent = url;
    $("cancel").disabled = false;
    $("stages").replaceChildren();
    await poll(jobId);
  } catch (err) {
    notice(err.message);
  } finally {
    currentJob = "";
    $("start").disabled = false;
    $("progress").hidden = true;
  }
}

async function poll(job) {
  for (;;) {
    const state = await api(`/api/jobs/${encodeURIComponent(job)}`);
    const stages = Object.entries(state.stages || {}).map(([name, value]) => el("li", `${label(name)} · ${label(value)}`));
    $("stages").replaceChildren(...stages);
    if (state.state === "done" && state.runId) {
      const path = `#/run/${encodeURIComponent(state.runId)}`;
      if (location.hash === path) await route();
      else location.hash = path;
      return;
    }
    if (state.state === "cancelled") {
      notice("Investigation cancelled.");
      return;
    }
    if (state.state !== "running") throw new Error(state.error || "Investigation did not finish.");
    await new Promise(resolve => setTimeout(resolve, 700));
  }
}

function setPanel(panel) {
  for (const name of panels) {
    $("panel-" + name).hidden = name !== panel;
  }
  for (const button of $("tabs").querySelectorAll("button")) {
    button.setAttribute("aria-pressed", String(button.dataset.panel === panel));
  }
  if (panel === "content") showContent();
}

function citation(ids) {
  const valid = ids.filter(id => current?.evidence.some(e => e.id === id));
  if (!valid.length) return el("span", ids.join(", "));
  const btn = el("button", valid.join(", "), "cite");
  btn.type = "button";
  btn.setAttribute("aria-label", `View evidence ${valid.join(", ")}`);
  btn.addEventListener("click", () => openEvidence(valid));
  return btn;
}

function openEvidence(ids) {
  const details = $("evidence-details");
  details.replaceChildren();
  for (const item of current.evidence.filter(e => ids.includes(e.id))) {
    const block = el("section");
    const metadata = [item.id, label(item.source), label(item.kind), date(item.observedAt)].filter(Boolean).join(" · ");
    block.append(el("h3", item.title || item.id), el("p", metadata, "subtle"));
    const a = external(item.url, "Open source");
    if (a) block.append(a);
    block.append(el("pre", JSON.stringify(item.data, null, 2) ?? ""));
    details.append(block);
  }
  if (!$("evidence-dialog").open) $("evidence-dialog").showModal();
}

function makeTable(headers, rows) {
  const container = el("div", null, "table-wrap");
  const table = el("table");
  const head = table.createTHead().insertRow();
  for (const value of headers) {
    const th = el("th", value);
    th.scope = "col";
    head.append(th);
  }
  const body = table.createTBody();
  for (const values of rows) {
    const row = body.insertRow();
    for (const value of values) {
      const cell = row.insertCell();
      if (value instanceof Node) cell.append(value);
      else cell.textContent = value == null ? "" : value;
    }
  }
  container.append(table);
  return container;
}

function showOverview(bundle) {
  const rows = (bundle.scorecard || []).map(row => {
    const ids = row.evidence || row.evidenceIds || [];
    const status = el("span", label(row.status), row.status === "unknown" ? "tag unknown" : "tag");
    return [label(row.channel), status, row.signal || "", ids.length ? citation(ids) : ""];
  });
  $("scorecard").replaceChildren(rows.length ? makeTable(["Channel", "Status", "Signal", "Evidence"], rows) : empty("No scorecard for this run."));
  const sources = (bundle.sources || []).map(source => [label(source.source), label(source.status), source.reason || ""]);
  $("source-runs").replaceChildren(sources.length ? makeTable(["Source", "Status", "Details"], sources) : empty("No source runs recorded."));
  const people = bundle.identity?.founders || [];
  $("founders-section").hidden = !people.length;
  if (people.length) {
    $("founders").replaceChildren(makeTable(["Name", "Role"], people.map(person => [
      external(person.url || person.linkedin?.url, person.name) || person.name, person.role,
    ])));
  } else {
    $("founders").replaceChildren();
  }
}

function showEvidence() {
  const search = $("evidence-search").value.trim().toLowerCase();
  const matches = current.evidence.filter(e => [e.id, e.title, e.source, e.kind].some(field => String(field || "").toLowerCase().includes(search)));
  if (!matches.length) {
    $("evidence-list").replaceChildren(empty(search ? "No evidence matches the search." : "No evidence collected."));
    return;
  }
  const list = el("ul", null, "list");
  for (const item of matches) {
    const row = el("li");
    const btn = el("button", null, "evidence-item");
    btn.type = "button";
    const title = el("span");
    title.append(el("strong", item.title || item.id), document.createTextNode(label(item.source)));
    btn.append(title, el("span", item.id));
    btn.addEventListener("click", () => openEvidence([item.id]));
    row.append(btn);
    list.append(row);
  }
  $("evidence-list").replaceChildren(list);
}

function showUnknowns(bundle) {
  const rows = (bundle.unknowns || []).map(item => {
    const card = el("article", null, "note-item");
    card.append(el("h3", label(item.channel)), el("p", item.reason));
    const check = external(item.manualCheck, "Check this source");
    if (check) card.append(check);
    return card;
  });
  $("unknown-list").replaceChildren(...(rows.length ? rows : [empty("No unknowns were recorded.")]));
}

function setFilter(select, values, placeholder) {
  select.replaceChildren(new Option(placeholder, ""));
  for (const value of [...new Set(values.filter(Boolean))].sort()) {
    select.add(new Option(label(value), value));
  }
}

function showContent() {
  const platform = $("platform").value;
  const kind = $("kind").value;
  const sort = $("sort-content").value;
  const items = current.content.filter(item => (!platform || item.platform === platform) && (!kind || item.kind === kind));
  const key = sort === "running" ? "runningDays" : sort;
  const score = item => sort === "latest" ? Date.parse(item.date) || 0 : item.metrics?.[key] ?? 0;
  items.sort((a, b) => score(b) - score(a));
  const root = $("content-list");
  if (!items.length) {
    root.replaceChildren(empty(current.content.length ? "No content matches these filters." : "No ads or posts collected. Check Unknowns for source availability."));
    return;
  }
  root.replaceChildren(...items.map(item => {
    const card = el("article", null, "content-card");
    const tags = [label(item.platform), label(item.kind), label(item.status), item.date].filter(Boolean).join(" · ");
    card.append(el("p", tags, "subtle"), el("h3", item.title || label(item.platform)));
    const thumb = httpURL(item.thumb);
    if (thumb) {
      const img = el("img");
      img.src = thumb;
      img.alt = "";
      img.loading = "lazy";
      img.referrerPolicy = "no-referrer";
      img.addEventListener("error", () => img.remove(), { once: true });
      card.append(img);
    }
    const videoURL = httpURL(item.video);
    if (videoURL && /\.mp4(?:$|[?#])/i.test(videoURL)) {
      const video = el("video");
      video.controls = true;
      video.preload = "none";
      video.playsInline = true;
      video.src = videoURL;
      video.setAttribute("aria-label", item.title || "Ad or post video");
      card.append(video);
    }
    if (item.text) card.append(el("p", item.text));
    if (item.metrics) {
      const metrics = Object.entries(item.metrics).filter(([, v]) => Number.isFinite(v))
        .map(([metric, value]) => `${value.toLocaleString()} ${metric === "runningDays" ? "days running" : metric}`);
      if (metrics.length) card.append(el("p", metrics.join(" · "), "subtle"));
    }
    const footer = el("footer");
    for (const link of [external(item.url, "Original"), external(item.video, "Video")]) {
      if (link) footer.append(link);
    }
    if (item.evidence) footer.append(citation([item.evidence]));
    card.append(footer);
    return card;
  }));
}

function inline(parent, line) {
  const tokens = /\[((?:E\d+)(?:\s*[, ]\s*E\d+)*)\]|\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)|\*\*([^*]+)\*\*|`([^`]+)`|\b(E\d+)\b/g;
  let cursor = 0;
  for (const match of line.matchAll(tokens)) {
    parent.append(document.createTextNode(line.slice(cursor, match.index)));
    if (match[1]) parent.append(citation(match[1].match(/E\d+/g)));
    else if (match[2]) parent.append(external(match[3], match[2]) || document.createTextNode(match[2]));
    else if (match[4]) {
      const strong = el("strong");
      inline(strong, match[4]);
      parent.append(strong);
    } else if (match[5]) parent.append(el("code", match[5]));
    else parent.append(citation([match[6]]));
    cursor = match.index + match[0].length;
  }
  parent.append(document.createTextNode(line.slice(cursor)));
}

function tableLine(line) {
  return line.trim().replace(/^\||\|$/g, "").split(/(?<!\\)\|/).map(value => value.trim().replaceAll("\\|", "|"));
}

function showReport(markdown) {
  const root = $("panel-report");
  root.replaceChildren();
  if (!markdown) {
    root.append(empty("This saved run has no written report."));
    return;
  }
  const lines = markdown.split(/\r?\n/);
  let list;
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    if (!line.trim()) {
      list = undefined;
      continue;
    }
    if (line.startsWith("|") && /^\|?\s*:?-{3,}/.test(lines[i + 1] || "")) {
      const heads = tableLine(line);
      const rows = [];
      i += 2;
      while (i < lines.length && lines[i].startsWith("|")) {
        rows.push(tableLine(lines[i]).map(text => {
          const cell = el("span");
          inline(cell, text);
          return cell;
        }));
        i++;
      }
      root.append(makeTable(heads, rows));
      i--;
      list = undefined;
      continue;
    }
    if (/^```/.test(line)) {
      const code = [];
      i++;
      while (i < lines.length && !/^```/.test(lines[i])) code.push(lines[i++]);
      root.append(el("pre", code.join("\n")));
      list = undefined;
      continue;
    }
    const item = line.match(/^\s*([-*]|\d+[.)])\s+(.+)/);
    if (item) {
      const type = /\d/.test(item[1]) ? "ol" : "ul";
      if (list?.localName !== type) {
        list = el(type);
        root.append(list);
      }
      const li = el("li");
      inline(li, item[2]);
      list.append(li);
      continue;
    }
    list = undefined;
    const heading = line.match(/^(#{1,6})\s+(.+)/);
    const quote = line.match(/^>\s*(.*)/);
    if (/^---+$/.test(line)) {
      root.append(el("hr"));
      continue;
    }
    const node = el(heading ? `h${Math.min(heading[1].length + 1, 6)}` : quote ? "blockquote" : "p");
    inline(node, heading?.[2] ?? quote?.[1] ?? line);
    root.append(node);
  }
}

function displayRun(id, data) {
  const bundle = data.bundle;
  current = { ...bundle, evidence: bundle.evidence || [], content: bundle.content || [] };
  $("brand").textContent = bundle.brand || bundle.target?.domain || "Investigation";
  $("target-meta").textContent = [bundle.target?.domain || bundle.target?.input, date(bundle.generatedAt)].filter(Boolean).join(" · ");
  $("compare").hidden = !bundle.target?.domain;
  const path = `/api/runs/${encodeURIComponent(id)}`;
  $("markdown-link").href = `${path}/report.md`;
  $("markdown-link").hidden = !data.report;
  const copyBtn = $("copy-report-btn");
  if (copyBtn) {
    copyBtn.hidden = !data.report;
    copyBtn.onclick = async () => {
      try {
        await navigator.clipboard.writeText(data.report);
        const originalText = copyBtn.textContent;
        copyBtn.textContent = "Copied!";
        setTimeout(() => { copyBtn.textContent = originalText; }, 2000);
      } catch (e) {
        copyBtn.textContent = "Copy failed";
        setTimeout(() => { copyBtn.textContent = "Copy report"; }, 2000);
      }
    };
  }
  $("evidence-link").href = `${path}/evidence.json`;
  $("metrics").replaceChildren(...[
    [current.evidence.length, "Evidence items"], [current.content.length, "Ads & posts"], [bundle.unknowns?.length || 0, "Unknowns"],
  ].map(([number, name]) => {
    const metric = el("div", null, "metric");
    metric.append(el("strong", number), el("span", name));
    return metric;
  }));
  $("report-warning").textContent = bundle.reportWarning || "";
  $("report-warning").hidden = !bundle.reportWarning;
  $("evidence-search").value = "";
  setFilter($("platform"), current.content.map(item => item.platform), "All platforms");
  setFilter($("kind"), current.content.map(item => item.kind), "All types");
  $("sort-content").value = "latest";
  showOverview(bundle);
  showReport(data.report);
  showEvidence();
  showUnknowns(bundle);
  setPanel("overview");
  $("run").hidden = false;
}

async function compareRuns() {
  const domain = current?.target?.domain;
  if (!domain) return;
  const dialog = $("compare-dialog");
  const details = $("compare-details");
  details.replaceChildren(el("p", "Reading saved runs…"));
  dialog.showModal();
  try {
    const d = await api(`/api/compare?domain=${encodeURIComponent(domain)}`);
    const pretty = value => typeof value === "number" ? value.toLocaleString() : value || "Unknown";
    const parts = [el("p", [date(d.from), date(d.to)].filter(Boolean).join(" → "), "subtle")];
    if (d.positioning) {
      parts.push(el("h3", "Positioning"), makeTable(["Before", "After"], [[d.positioning.before, d.positioning.after]]));
    }
    if (d.metrics?.length) {
      parts.push(el("h3", "Metrics"), makeTable(["Metric", "Before", "After", "Change"],
        d.metrics.map(m => [m.name, pretty(m.before), pretty(m.after), pretty(m.delta)])));
    }
    if (d.channels?.length) {
      parts.push(el("h3", "Channel changes"), makeTable(["Channel", "Before", "After"],
        d.channels.map(c => [label(c.channel), pretty(c.before), pretty(c.after)])));
    }
    for (const [key, heading] of [["added", "Newly observed"], ["noLongerSeen", "No longer seen in a checked sample"]]) {
      if (!d[key]?.length) continue;
      const list = el("ul");
      for (const item of d[key]) {
        const li = el("li");
        const title = item.title || label(item.platform);
        li.append(external(item.url, title) || document.createTextNode(title));
        list.append(li);
      }
      parts.push(el("h3", heading), list);
    }
    if (parts.length === 1) parts.push(empty("No changes were recorded between these runs."));
    details.replaceChildren(...parts);
  } catch (err) {
    details.replaceChildren(empty(err.message));
  }
}

async function route() {
  const epoch = ++renderEpoch;
  const match = location.hash.match(/^#\/run\/([^/]+)$/);
  $("home").hidden = !!match;
  $("run").hidden = true;
  notice("");
  if (!match) {
    await refreshHistory();
    return;
  }
  try {
    const id = decodeURIComponent(match[1]);
    const data = await api(`/api/runs/${encodeURIComponent(id)}`);
    if (epoch === renderEpoch) displayRun(id, data);
  } catch (err) {
    if (epoch !== renderEpoch) return;
    $("home").hidden = false;
    notice(err.message);
    await refreshHistory();
  }
}

$("investigate").addEventListener("submit", begin);
$("cancel").addEventListener("click", async () => {
  if (!currentJob) return;
  $("cancel").disabled = true;
  try {
    await post(`/api/jobs/${encodeURIComponent(currentJob)}/cancel`, {});
    $("progress-title").textContent = "Cancelling investigation";
  } catch (err) {
    $("cancel").disabled = false;
    notice(err.message);
  }
});
$("refresh").addEventListener("click", refreshHistory);
$("compare").addEventListener("click", compareRuns);
$("rerun").addEventListener("click", () => {
  $("target").value = current?.target?.input || current?.target?.url || "";
  location.hash = "#/";
  $("home").hidden = false;
  $("target").focus();
});
$("tabs").addEventListener("click", event => {
  const button = event.target.closest("button[data-panel]");
  if (button) setPanel(button.dataset.panel);
});
$("evidence-search").addEventListener("input", showEvidence);
for (const id of ["platform", "kind", "sort-content"]) $(id).addEventListener("change", showContent);
$("status-button").addEventListener("click", () => {
  $("status-dialog").showModal();
  refreshStatus().catch(err => $("status-details").replaceChildren(empty(err.message)));
});
window.addEventListener("hashchange", route);
configureAreas(defaultAreas);
refreshStatus().catch(err => $("status-details").replaceChildren(empty(err.message)));
route();
