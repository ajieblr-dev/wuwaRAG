/* Resonance Archive · shared app runtime (prototype, mock-backed) */
(function () {
  "use strict";
  const OD = (window.OD = {});

  /* ── safe storage (works even when the preview sandbox blocks Web Storage) ── */
  function makeStore(kind) {
    var mem = Object.create(null), s = null;
    try { s = window[kind + "Storage"]; if (!s) throw 0; s.setItem("__od_t", "1"); s.removeItem("__od_t"); } catch (e) { s = null; }
    return {
      live: !!s,
      get: function (k) { if (s) { try { return s.getItem(k); } catch (e) {} } return k in mem ? mem[k] : null; },
      set: function (k, v) { if (s) { try { s.setItem(k, v); return; } catch (e) {} } mem[k] = String(v); },
      remove: function (k) { if (s) { try { s.removeItem(k); } catch (e) {} } delete mem[k]; },
      keys: function () { if (s) { try { return Object.keys(s); } catch (e) {} } return Object.keys(mem); },
      all: function () { var o = {}, self = this; this.keys().forEach(function (k) { o[k] = self.get(k); }); return o; },
      clear: function () { var self = this; this.keys().forEach(function (k) { self.remove(k); }); }
    };
  }
  var LS = makeStore("local"), SS = makeStore("session");
  OD.LS = LS; OD.SS = SS;

  /* ── settings ─────────────────────────────────────────────── */
  const S = {
    get theme() { return LS.get("od-theme") || "dark"; },
    set theme(v) { LS.set("od-theme", v); OD.applyPrefs(); },
    get density() { return LS.get("od-density") || "comfortable"; },
    set density(v) { LS.set("od-density", v); OD.applyPrefs(); },
    get spoiler() { return LS.get("od-spoiler") || "off"; },
    set spoiler(v) { LS.set("od-spoiler", v); OD.applyPrefs(); },
    get reduced() { return LS.get("od-reduced") === "true"; },
    set reduced(v) { LS.set("od-reduced", String(v)); OD.applyPrefs(); },
    get previewSources() { return (LS.get("od-preview-sources") ?? "true") === "true"; },
    set previewSources(v) { LS.set("od-preview-sources", String(v)); },
    get mock() { return (LS.get("od-mock") ?? "true") === "true"; },
    set mock(v) { LS.set("od-mock", String(v)); OD.applyPrefs(); },
    get base() { return LS.get("od-base") || "http://localhost:8080"; },
    set base(v) { LS.set("od-base", v.replace(/\/+$/, "")); }
  };
  OD.S = S;

  OD.applyPrefs = function () {
    const root = document.documentElement;
    const t = S.theme;
    if (t === "system") {
      root.setAttribute("data-theme", "system");
    } else {
      root.setAttribute("data-theme", t);
    }
    document.body.dataset.density = S.density;
    document.body.dataset.spoiler = S.spoiler;
    document.body.dataset.reduced = String(S.reduced || window.matchMedia("(prefers-reduced-motion: reduce)").matches);
    document.querySelectorAll("[data-theme-select]").forEach((el) => {
      el.querySelectorAll("button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.themeVal === t)));
    });
    document.querySelectorAll("[data-mock-badge]").forEach((el) => (el.hidden = !S.mock));
    const tgl = document.getElementById("theme-toggle");
    if (tgl) tgl.setAttribute("aria-label", "Switch to " + (t === "dark" ? "light" : "dark") + " theme");
  };

  /* ── stable speaker hue (unit-testable) ───────────────────── */
  OD.hashString = function (str) {
    let h = 2166136261;
    for (let i = 0; i < str.length; i++) {
      h ^= str.charCodeAt(i);
      h = Math.imul(h, 16777619);
    }
    return h >>> 0;
  };
  OD.speakerHue = function (name) {
    return OD.hashString(String(name).toLowerCase().trim()) % 360;
  };
  OD.uid = function () {
    try { if (window.crypto && crypto.randomUUID) return crypto.randomUUID(); } catch (e) {}
    return "id-" + Date.now().toString(36) + "-" + Math.random().toString(36).slice(2, 8);
  };
  OD.initials = function (name) {
    const parts = String(name).trim().split(/\s+/);
    if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
    return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
  };
  OD.speakerChip = function (name) {
    return `<span class="spk" style="--h:${OD.speakerHue(name)}"><span class="ini">${OD.initials(name)}</span>${name}</span>`;
  };

  /* ── icons ────────────────────────────────────────────────── */
  const I = (p, sw) => `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="${sw || 1.6}" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${p}</svg>`;
  OD.icons = {
    ask: I('<path d="M4 5h16v11H9l-5 4z"/><path d="M9 9h7M9 12h4"/>'),
    search: I('<circle cx="11" cy="11" r="7"/><path d="M20 20l-3.2-3.2"/>'),
    about: I('<circle cx="12" cy="12" r="9"/><path d="M12 11v5M12 8h.01"/>'),
    admin: I('<path d="M12 3l7 3v5c0 4.4-3 8-7 10-4-2-7-5.6-7-10V6z"/><path d="M9.5 12l1.8 1.8 3.4-3.8"/>'),
    sun: I('<circle cx="12" cy="12" r="4.2"/><path d="M12 2v2M12 20v2M2 12h2M20 12h2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M19.1 4.9l-1.4 1.4M6.3 17.7l-1.4 1.4"/>'),
    moon: I('<path d="M20 14.5A8 8 0 109.5 4a6.5 6.5 0 1010.5 10.5z"/>'),
    wave: I('<path d="M2 14c2-6 4-6 6 0s4 6 6 0 4-6 6 0"/>', 1.4),
    cp: I('<rect x="9" y="9" width="11" height="11" rx="2"/><path d="M5 15V6a2 2 0 012-2h9"/>'),
    check: I('<path d="M4 12l5 5L20 6"/>', 2),
    x: I('<path d="M6 6l12 12M18 6L6 18"/>'),
    send: I('<path d="M4 12l16-8-6 16-3-6z"/>'),
    arrow: I('<path d="M5 12h14M13 6l6 6-6 6"/>'),
    expand: I('<path d="M8 3H5a2 2 0 00-2 2v3M16 3h3a2 2 0 012 2v3M8 21H5a2 2 0 01-2-2v-3M16 21h3a2 2 0 002-2v-3"/>'),
    download: I('<path d="M12 3v12M7 10l5 5 5-5M5 21h14"/>'),
    refresh: I('<path d="M20 11a8 8 0 10-1.5 5.5M20 5v6h-6"/>'),
    play: I('<path d="M7 4l12 8-12 8z"/>'),
    pause: I('<path d="M8 5v14M16 5v14"/>', 2),
    alert: I('<path d="M12 3l9 16H3z"/><path d="M12 10v4M12 17h.01"/>'),
    info: I('<circle cx="12" cy="12" r="9"/><path d="M12 11v5M12 8h.01"/>'),
    lock: I('<rect x="4" y="10" width="16" height="10" rx="2"/><path d="M8 10V7a4 4 0 018 0v3"/>'),
    db: I('<ellipse cx="12" cy="6" rx="7" ry="3"/><path d="M5 6v12c0 1.7 3.1 3 7 3s7-1.3 7-3V6M5 12c0 1.7 3.1 3 7 3s7-1.3 7-3"/>'),
    route: I('<circle cx="6" cy="6" r="2.4"/><circle cx="18" cy="18" r="2.4"/><path d="M8.4 6H15a3 3 0 010 6H9a3 3 0 000 6h6.6"/>'),
    list: I('<path d="M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01"/>'),
    cog: I('<circle cx="12" cy="12" r="3"/><path d="M19 12a7 7 0 00-.1-1.2l2-1.6-2-3.4-2.4 1a7 7 0 00-2-1.2L14 2h-4l-.5 2.6a7 7 0 00-2 1.2l-2.4-1-2 3.4 2 1.6A7 7 0 005 12a7 7 0 00.1 1.2l-2 1.6 2 3.4 2.4-1a7 7 0 002 1.2L10 22h4l.5-2.6a7 7 0 002-1.2l2.4 1 2-3.4-2-1.6A7 7 0 0019 12z"/>'),
    kbd: I('<rect x="2" y="6" width="20" height="12" rx="2"/><path d="M7 10h.01M11 10h.01M15 10h.01" stroke-width="2.4"/>'),
    copy: I('<rect x="9" y="9" width="11" height="11" rx="2"/><path d="M5 15V6a2 2 0 012-2h9"/>'),
    filter: I('<path d="M3 5h18l-7 8v6l-4-2v-4z"/>'),
    back: I('<path d="M15 5l-7 7 7 7"/>'),
    external: I('<path d="M14 4h6v6M20 4l-9 9M18 14v5a1 1 0 01-1 1H5a1 1 0 01-1-1V7a1 1 0 011-1h5"/>'),
    clock: I('<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>'),
    spark: I('<path d="M12 3v5M12 16v5M3 12h5M16 12h5"/>'),
    eye: I('<path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7-10-7-10-7z"/><circle cx="12" cy="12" r="3"/>'),
    hash: I('<path d="M5 9h14M5 15h14M10 4l-2 16M16 4l-2 16"/>')
  };

  /* ── toast ────────────────────────────────────────────────── */
  let toastHost;
  OD.toast = function (msg, kind) {
    kind = kind || "info";
    if (!toastHost) {
      toastHost = document.createElement("div");
      toastHost.className = "toasts";
      toastHost.setAttribute("role", "status");
      toastHost.setAttribute("aria-live", "polite");
      document.body.appendChild(toastHost);
    }
    const ic = { ok: OD.icons.check, err: OD.icons.alert, warn: OD.icons.alert, info: OD.icons.info }[kind];
    const el = document.createElement("div");
    el.className = "toast " + kind;
    el.innerHTML = ic.replace("<svg", '<svg style="width:16px;height:16px"') + "<span></span>";
    el.querySelector("span").textContent = msg;
    toastHost.appendChild(el);
    setTimeout(() => el.remove(), 4200);
  };

  OD.copy = async function (text, label) {
    try {
      await navigator.clipboard.writeText(text);
      OD.toast((label || "Copied") + " to clipboard", "ok");
    } catch (e) {
      OD.toast("Copy blocked by browser", "err");
    }
  };

  /* ── request log (client-side, session only) ──────────────── */
  OD.log = {
    all() { try { return JSON.parse(SS.get("od-reqlog") || "[]"); } catch (e) { return []; } },
    add(entry) {
      const rows = OD.log.all();
      rows.unshift(entry);
      SS.set("od-reqlog", JSON.stringify(rows.slice(0, 200)));
      window.dispatchEvent(new CustomEvent("od-log", { detail: entry }));
    },
    clear() { SS.remove("od-reqlog"); window.dispatchEvent(new CustomEvent("od-log")); }
  };

  /* ── mock corpus (real page facts; [mock] prefixed placeholders) ── */
  OD.corpus = [
    { title: "Simulacrum of the Heart", chars: 168893, chunks: 0, real: true },
    { title: "Hsin", chars: 3167, chunks: 0 },
    { title: "Suoming", chars: 1861, chunks: 0 },
    { title: "Mengzhou", chars: 5179, chunks: 0 },
    { title: "Xuanfang Hold", chars: 4028, chunks: 0 }
  ];
  OD.speakers = ["Rover", "Suoming", "Hsin", "Abby", "Tangning", "Qingxiao", "Jiu", "Ninth Seal"];
  OD.suggested = [
    "What is the Waking Moon Festival and what role does Sentinel Hsin play?",
    "What are Moon Fox Rice Cakes?",
    "Who is Intendant Suoming?",
    "What is Xuanfang Hold?"
  ];
  OD.wikiUrl = (t) => "https://wutheringwaves.fandom.com/wiki/" + encodeURIComponent(String(t).replace(/ /g, "_"));

  const MOCK_ANSWER =
    "[mock] Grounded answer preview. In production this block is written only from the passages the retriever returns, " +
    "with a citation chip after each claim [1]. This prototype has no live gateway behind it, so the text here describes " +
    "the shape of a real answer rather than any lore content [2]. Selecting a chip highlights the matching source card; " +
    "every claim must resolve to a passage, and passages outside the indexed scope produce a thinner answer.";
  OD.mockAnswer = MOCK_ANSWER;

  OD.mockSources = function () {
    const specs = [
      { page: "Simulacrum of the Heart", section: "[mock] Overview", spk: ["Rover", "Suoming"], score: 0.82 },
      { page: "Hsin", section: "[mock] Sentinel record", spk: ["Hsin"], score: 0.74 },
      { page: "Xuanfang Hold", section: "[mock] Region notes", spk: ["Tangning", "Qingxiao"], score: 0.61 },
      { page: "Suoming", section: "[mock] Character profile", spk: ["Suoming", "Jiu"], score: 0.55 },
      { page: "Mengzhou", section: "[mock] Location", spk: ["Ninth Seal"], score: 0.44 }
    ];
    return specs.map((s, i) => OD.normalizeSource({
      index: i + 1,
      page_title: s.page,
      section_title: s.section,
      speakers: s.spk,
      score: s.score,
      chunk_text:
        "[mock] Placeholder passage. This stands in for a retrieved chunk from \u201C" + s.page +
        "\u201D so the sources panel and citation chips can be reviewed before the backend lists real chunks."
    }, i));
  };

  /* ── source normalizer (tolerant to BOTH shapes) ─────────────
     Mock shape:    { index, page_title, section_title, speakers, score, chunk_text }
     Backend shape: { id, title, section, subsection, speakers, content, score }
                    (+ chunk_index carried through when present)
     Returns a canonical object carrying BOTH aliases so every
     renderer can read .title/.page_title, .section/.section_title,
     .content/.chunk_text/.text interchangeably. Always use
     OD.normalizeSource(s, i) / OD.normalizeSources(list) before render. */
  OD.normalizeSource = function (raw, pos) {
    if (!raw || typeof raw !== "object") raw = {};
    var i = (typeof pos === "number" ? pos : 0);
    var n = Number(raw.index != null ? raw.index : (raw.chunk_index != null ? raw.chunk_index : i + 1));
    if (!isFinite(n) || n <= 0) n = i + 1;
    var title = raw.page_title != null ? raw.page_title : (raw.page != null ? raw.page : (raw.title != null ? raw.title : "[page title not exposed]"));
    var section = raw.section_title != null ? raw.section_title : (raw.section != null ? raw.section : "");
    var subsection = raw.subsection != null ? raw.subsection : "";
    var speakers = Array.isArray(raw.speakers) ? raw.speakers.slice() : (raw.speaker ? [raw.speaker] : []);
    var text = raw.chunk_text != null ? raw.chunk_text : (raw.content != null ? raw.content : (raw.text != null ? raw.text : ""));
    var score = (typeof raw.score === "number" && isFinite(raw.score)) ? raw.score : null;
    var id = raw.id != null ? String(raw.id) : String(n);
    var sTitle = String(title);
    var sSection = String(section);
    var sSub = String(subsection || "");
    var sText = String(text);
    return {
      index: n,
      id: id,
      chunk_index: (raw.chunk_index != null ? raw.chunk_index : n),
      title: sTitle, page_title: sTitle, page: sTitle,
      section: sSection, section_title: sSection,
      subsection: sSub,
      speakers: speakers, speaker: speakers[0] || "",
      content: sText, chunk_text: sText, text: sText,
      score: score
    };
  };

  OD.normalizeSources = function (list) {
    if (!Array.isArray(list)) return [];
    return list.map(function (s, i) { return OD.normalizeSource(s, i); });
  };

  /* Normalize search/chat envelopes so callers always see
     { query|question, limit|count, results|sources(normalized) }. */
  OD.normalizeSearch = function (res, fallbackQuery, fallbackLimit) {
    res = (res && typeof res === "object") ? res : {};
    var results = OD.normalizeSources(res.results || res.snippets || res.sources || []);
    var limit = (res.limit != null ? res.limit : (res.count != null ? res.count : fallbackLimit));
    if (limit == null) limit = results.length;
    return {
      query: (res.query != null ? res.query : (fallbackQuery || "")),
      limit: limit,
      count: (res.count != null ? res.count : results.length),
      results: results
    };
  };

  OD.normalizeChat = function (res) {
    res = (res && typeof res === "object") ? res : {};
    return {
      question: res.question || "",
      answer: res.answer || "",
      sources: OD.normalizeSources(res.sources || res.results || []),
      model: res.model || "",
      latency_ms: (res.latency_ms != null ? res.latency_ms : null),
      total_tokens: (res.total_tokens != null ? res.total_tokens : null)
    };
  };

  /* ── API layer (mock + real, tolerant shapes) ─────────────── */
  const wait = (ms) => new Promise((r) => setTimeout(r, ms));

  OD.api = {
    async health() {
      if (S.mock) {
        await wait(220);
        return { status: "ok", service: "wuwa-lore-api", qdrant_connected: true, collection: "wuwa_lore", indexed_chunks: 222, timestamp: new Date().toISOString() };
      }
      return OD.request("GET", "/health");
    },
    async status() {
      if (S.mock) {
        await wait(180);
        return { collection: "wuwa_lore", status: "green", points_count: 222, vectors_count: 222 };
      }
      return OD.request("GET", "/api/v1/status");
    },
    async search(query, limit) {
      if (S.mock) {
        const t0 = Date.now();
        await wait(340);
        const results = OD.mockSources().slice(0, limit || 5).map((s) => ({ ...s, score: Math.min(0.97, s.score + (Math.random() - 0.5) * 0.06) }));
        OD.log.add({ id: OD.uid(), ts: Date.now(), method: "POST", path: "/api/v1/search", status: 200, ms: Date.now() - t0, tokens: 0, err: null, mock: true, note: query });
        return OD.normalizeSearch({ query, limit: limit || 5, results }, query, limit || 5);
      }
      const res = await OD.request("POST", "/api/v1/search", { query, limit: limit || 5 });
      return OD.normalizeSearch(res, query, limit || 5);
    },
    async chat(question, signal) {
      if (S.mock) {
        const t0 = Date.now();
        await wait(4200);
        if (signal && signal.aborted) throw Object.assign(new Error("aborted"), { name: "AbortError" });
        const tokens = 2680 + Math.floor(Math.random() * 420);
        const ms = Date.now() - t0;
        OD.log.add({ id: OD.uid(), ts: Date.now(), method: "POST", path: "/api/v1/chat", status: 200, ms, tokens, err: null, mock: true, note: question.slice(0, 60) });
        return OD.normalizeChat({ question, model: "wuwa-lore-model", latency_ms: ms, total_tokens: tokens, answer: MOCK_ANSWER, sources: OD.mockSources() });
      }
      const res = await OD.request("POST", "/api/v1/chat", { question }, signal);
      return OD.normalizeChat(res);
    },
    async ingest(payload) {
      if (S.mock) {
        const t0 = Date.now();
        await wait(2600);
        return { mock: true, accepted: payload, before: 222, after: 222, message: "[mock] Re-ingestion upserts deterministic IDs; chunk count unchanged in this placeholder." };
      }
      return OD.request("POST", "/api/v1/ingest", payload);
    }
  };

  OD.request = async function (method, path, body, signal) {
    const t0 = Date.now();
    const entry = { id: OD.uid(), ts: Date.now(), method, path, status: null, ms: 0, tokens: 0, err: null };
    try {
      const res = await fetch(S.base + path, {
        method,
        headers: Object.assign({ "Content-Type": "application/json" }, OD.token ? { Authorization: "Bearer " + OD.token } : {}),
        body: body ? JSON.stringify(body) : undefined,
        signal
      });
      entry.status = res.status;
      const text = await res.text();
      let data = {};
      try { data = text ? JSON.parse(text) : {}; } catch (e) { data = { raw: text }; }
      entry.ms = Date.now() - t0;
      if (res.status === 401 || res.status === 403) { entry.err = "auth"; OD.log.add(entry); throw Object.assign(new Error("auth"), { name: "AuthError", status: res.status }); }
      if (!res.ok) { entry.err = "http"; OD.log.add(entry); throw Object.assign(new Error("http"), { name: "HttpError", status: res.status, body: text.slice(0, 300) }); }
      OD.log.add(entry);
      return data;
    } catch (e) {
      if (e.name === "AbortError" || e.message === "aborted") { e.name = "AbortError"; entry.err = "aborted"; entry.ms = Date.now() - t0; OD.log.add(entry); throw e; }
      if (e.name === "AuthError" || e.name === "HttpError") throw e;
      entry.err = "network"; entry.ms = Date.now() - t0; OD.log.add(entry);
      throw Object.assign(new Error("network"), { name: "NetworkError" });
    }
  };

  OD.errorClass = function (e) {
    if (!e) return "network";
    if (e.name === "AbortError") return "aborted";
    if (e.name === "AuthError") return "401/403";
    if (e.name === "NetworkError" || e.name === "TypeError") return "network";
    if (e.name === "HttpError") return "http " + e.status;
    return "error";
  };

  /* ── shell ────────────────────────────────────────────────── */
  const NAV = [
    { key: "ask", href: "ask.html", label: "Ask", icon: "ask" },
    { key: "search", href: "search.html", label: "Search", icon: "search" },
    { key: "about", href: "about.html", label: "About", icon: "about" }
  ];
  const ADMIN_NAV = [
    { key: "admin-overview", href: "admin-overview.html", label: "Overview", icon: "admin" },
    { key: "admin-ingest", href: "admin-ingest.html", label: "Ingest", icon: "refresh" },
    { key: "admin-corpus", href: "admin-corpus.html", label: "Corpus", icon: "db" },
    { key: "admin-gateway", href: "admin-gateway.html", label: "Gateway", icon: "route" },
    { key: "admin-logs", href: "admin-logs.html", label: "Logs", icon: "list" },
    { key: "admin-settings", href: "admin-settings.html", label: "Settings", icon: "cog" }
  ];

  OD.mount = function (active) {
    try {
    OD.applyPrefs();
    const isAdmin = String(active).startsWith("admin");
    const tabs = NAV.map((n) => `<a href="${n.href}" ${active === n.key ? 'aria-current="page"' : ""}>${OD.icons[n.icon]}${n.label}</a>`).join("");
    const bar = document.createElement("header");
    bar.className = "topbar";
    bar.innerHTML = `
      <a class="skip-link" href="#content">Skip to content</a>
      <div class="topbar-inner">
        <a class="brand" href="index.html" aria-label="Resonance Archive home">
          <span class="mark">${OD.icons.wave}</span> Resonance Archive
        </a>
        <nav class="tabs hide-sm" aria-label="Primary">${tabs}</nav>
        <div class="tools">
          <span class="mock-badge" data-mock-badge hidden>MOCK</span>
          <button class="health-dot hide-sm" id="health-dot" data-state="unknown" type="button" title="Backend health">
            <span class="dot"></span><span id="health-text">checking</span>
          </button>
          <button class="btn-icon btn btn-ghost hide-sm" id="palette-btn" type="button" aria-label="Open command palette" title="Command palette (Ctrl/Cmd K)">${OD.icons.kbd}</button>
          <button class="btn-icon btn btn-ghost" id="theme-toggle" type="button" aria-label="Toggle theme">${OD.icons.moon}</button>
          <a class="btn btn-secondary btn-sm hide-sm" href="${isAdmin ? "ask.html" : "admin.html"}">${isAdmin ? "Reader" : "Admin"}</a>
        </div>
      </div>`;
    document.body.prepend(bar);

    const bn = document.createElement("nav");
    bn.className = "bottomnav";
    bn.setAttribute("aria-label", "Primary mobile");
    bn.innerHTML = NAV.concat([{ key: "admin", href: isAdmin ? "admin-overview.html" : "admin.html", label: "Admin", icon: "admin" }])
      .map((n) => `<a href="${n.href}" ${active === n.key ? 'aria-current="page"' : ""}>${OD.icons[n.icon]}${n.label}</a>`).join("");
    document.body.appendChild(bn);

    const themeBtn = document.getElementById("theme-toggle");
    themeBtn.innerHTML = S.theme === "light" ? OD.icons.sun : OD.icons.moon;
    themeBtn.addEventListener("click", () => {
      S.theme = S.theme === "light" ? "dark" : "light";
      themeBtn.innerHTML = S.theme === "light" ? OD.icons.sun : OD.icons.moon;
    });
    window.matchMedia("(prefers-color-scheme: light)").addEventListener("change", OD.applyPrefs);

    if (isAdmin) OD.mountAdminNav(active);
    OD.healthPoll();
    OD.palette(active);
    OD.shortcuts();
    OD.applyPrefs();
    } catch (e) { try { console.error("OD.mount failed:", e); } catch (_) {} }
  };

  OD.mountAdminNav = function (active) {
    const host = document.getElementById("admin-nav");
    if (!host) return;
    host.innerHTML = ADMIN_NAV.map((n) => `<a href="${n.href}" ${active === n.key ? 'aria-current="page"' : ""}>${OD.icons[n.icon]}${n.label}</a>`).join("");
  };

  OD.healthPoll = function () {
    const dot = document.getElementById("health-dot");
    const txt = document.getElementById("health-text");
    if (!dot) return;
    async function tick() {
      try {
        const h = await OD.api.health();
        const ok = h.qdrant_connected !== false && h.status !== "down";
        dot.dataset.state = ok ? "ok" : "warn";
        txt.textContent = ok ? (h.qdrant_connected ? "index ok" : "index down") : "degraded";
        dot.title = `service ${h.service || "?"} · collection ${h.collection || "?"} · ${h.indexed_chunks ?? "?"} chunks`;
      } catch (e) {
        dot.dataset.state = "down";
        txt.textContent = "offline";
        dot.title = "Cannot reach " + S.base;
      }
    }
    tick();
    if (OD._ht) clearInterval(OD._ht);
    OD._ht = setInterval(tick, 30000);
  };

  /* ── overlay focus management ──────────────────────────────────
     Every dialog in the prototype (palette, spoiler gate, shortcuts, sources sheet)
     opened without moving focus into it or giving focus back on close. Keyboard and
     screen-reader users were left on the trigger with the dialog open. */
  const FOCUSABLE =
    'a[href],button:not([disabled]),input:not([disabled]),select:not([disabled]),' +
    'textarea:not([disabled]),[tabindex]:not([tabindex="-1"])';
  let lastFocused = null;

  OD.focusables = (root) =>
    Array.prototype.slice.call(root.querySelectorAll(FOCUSABLE)).filter((el) => el.getClientRects().length > 0);

  OD.openOverlay = function (node, opts) {
    opts = opts || {};
    if (!node || !node.hidden) return;
    lastFocused = document.activeElement;
    node.hidden = false;
    const target = opts.initialFocus || OD.focusables(node)[0] || node;
    if (target === node && !node.hasAttribute("tabindex")) node.setAttribute("tabindex", "-1");
    try { target.focus({ preventScroll: true }); } catch (e) { /* focus is best-effort */ }
    if (opts.onOpen) opts.onOpen();
  };

  OD.closeOverlay = function (node, opts) {
    opts = opts || {};
    if (!node || node.hidden) return;
    node.hidden = true;
    if (opts.onClose) opts.onClose();
    const back = lastFocused;
    lastFocused = null;
    if (back && document.contains(back)) { try { back.focus({ preventScroll: true }); } catch (e) {} }
  };

  /* Trap Tab inside whichever overlay is topmost. */
  document.addEventListener("keydown", (e) => {
    if (e.key !== "Tab") return;
    const open = Array.prototype.slice
      .call(document.querySelectorAll(".scrim, .palette"))
      .filter((n) => !n.hidden);
    if (!open.length) return;
    const layer = open[open.length - 1];
    const box = layer.querySelector('[role="dialog"]') || layer;
    const f = OD.focusables(box);
    if (!f.length) return;
    const first = f[0], last = f[f.length - 1];
    if (!box.contains(document.activeElement)) { first.focus(); e.preventDefault(); return; }
    if (e.shiftKey && document.activeElement === first) { last.focus(); e.preventDefault(); }
    else if (!e.shiftKey && document.activeElement === last) { first.focus(); e.preventDefault(); }
  }, true);

  /* ── command palette ──────────────────────────────────────── */
  OD.palette = function (active) {
    const items = [
      { grp: "Navigate", label: "Ask the archive", href: "ask.html", k: "G A", icon: "ask" },
      { grp: "Navigate", label: "Semantic search", href: "search.html", k: "G S", icon: "search" },
      { grp: "Navigate", label: "About & attribution", href: "about.html", k: "", icon: "about" },
      { grp: "Navigate", label: "Design system", href: "design-system.html", k: "", icon: "spark" },
      { grp: "Admin", label: "Admin overview", href: "admin-overview.html", k: "", icon: "admin" },
      { grp: "Admin", label: "Re-ingestion", href: "admin-ingest.html", k: "", icon: "refresh" },
      { grp: "Admin", label: "Corpus explorer", href: "admin-corpus.html", k: "", icon: "db" },
      { grp: "Admin", label: "Gateway routing", href: "admin-gateway.html", k: "", icon: "route" },
      { grp: "Admin", label: "Request log", href: "admin-logs.html", k: "", icon: "list" },
      { grp: "Admin", label: "Settings", href: "admin-settings.html", k: "", icon: "cog" },
      { grp: "Actions", label: "Toggle theme", action: () => (S.theme = S.theme === "light" ? "dark" : "light"), k: "", icon: "sun" },
      { grp: "Actions", label: "New question", href: "ask.html#new", k: "/", icon: "send" }
    ];
    const host = document.createElement("div");
    host.className = "palette";
    host.id = "palette";
    host.hidden = true;
    host.innerHTML = `<div class="palette-box" role="dialog" aria-modal="true" aria-label="Command palette">
      <input type="text" id="palette-input" placeholder="Search commands and pages…" aria-label="Command palette search" />
      <div class="palette-list" id="palette-list" role="listbox"></div></div>`;
    document.body.appendChild(host);

    const input = host.querySelector("#palette-input");
    const list = host.querySelector("#palette-list");
    let sel = 0, filtered = items;

    function render() {
      const q = input.value.toLowerCase().trim();
      filtered = items.filter((i) => i.label.toLowerCase().includes(q) || i.grp.toLowerCase().includes(q));
      sel = Math.min(sel, Math.max(0, filtered.length - 1));
      let html = "", last = "";
      filtered.forEach((it, idx) => {
        if (it.grp !== last) { html += `<div class="grp">${it.grp}</div>`; last = it.grp; }
        html += `<button class="palette-item" data-i="${idx}" aria-selected="${idx === sel}" role="option">${OD.icons[it.icon]}<span>${it.label}</span>${it.k ? `<span class="pk">${it.k}</span>` : ""}</button>`;
      });
      list.innerHTML = html || `<div class="grp">No matches</div>`;
      list.querySelectorAll(".palette-item").forEach((b) => {
        b.addEventListener("click", () => run(filtered[+b.dataset.i]));
        b.addEventListener("mousemove", () => { sel = +b.dataset.i; render(); });
      });
    }
    function run(it) { close(); if (it.href) location.href = it.href; else if (it.action) it.action(); }
    function open() { OD.openOverlay(host, { initialFocus: input }); input.value = ""; sel = 0; render(); }
    function close() { OD.closeOverlay(host); }

    input.addEventListener("input", render);
    input.addEventListener("keydown", (e) => {
      if (e.key === "ArrowDown") { sel = Math.min(sel + 1, filtered.length - 1); render(); e.preventDefault(); }
      else if (e.key === "ArrowUp") { sel = Math.max(sel - 1, 0); render(); e.preventDefault(); }
      else if (e.key === "Enter" && filtered[sel]) run(filtered[sel]);
      else if (e.key === "Escape") close();
    });
    host.addEventListener("click", (e) => { if (e.target === host) close(); });
    document.getElementById("palette-btn")?.addEventListener("click", open);
    window.addEventListener("keydown", (e) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "k") { e.preventDefault(); host.hidden ? open() : close(); }
    });
    OD.openPalette = open;
  };

  OD.shortcuts = function () {
    window.addEventListener("keydown", (e) => {
      const tag = (document.activeElement && document.activeElement.tagName) || "";
      const typing = tag === "INPUT" || tag === "TEXTAREA" || document.activeElement?.isContentEditable;

      /* close topmost only, and hand focus back to whatever opened it */
      const open = () =>
        Array.prototype.slice
          .call(document.querySelectorAll(".scrim, .palette"))
          .filter((n) => !n.hidden);

      if (e.key === "Escape") {
        const layers = open();
        if (!layers.length) return;
        OD.closeOverlay(layers[layers.length - 1]);
        return;
      }

      /* A modal is up: single-key shortcuts must not reach through it and move
         focus to a control behind the dialog. */
      if (open().length) return;

      if (e.key === "/" && !typing) {
        const c = document.querySelector("[data-focus-composer]");
        if (c) { e.preventDefault(); c.focus(); }
      } else if (e.key === "?" && !typing) {
        const s = document.getElementById("shortcut-modal");
        if (!s) return;
        e.preventDefault();
        OD.openOverlay(s);
      }
    });
  };

  OD.enterLabel = () => "Enter";
})();
