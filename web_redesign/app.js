/**
 * Resonance Archive · Web Runtime (v2 Redesign)
 * Connects directly to Go backend endpoints:
 *   GET  /health
 *   GET  /api/v1/status
 *   POST /api/v1/chat
 *   POST /api/v1/search
 */

(function () {
  "use strict";

  const RA = (window.RA = {});

  /* ── 0. Base URL Resolver ── */
  const API_BASE = (function () {
    if (typeof window !== "undefined" && window.location) {
      if (window.location.protocol === "http:" || window.location.protocol === "https:") {
        if (window.location.port === "8080") return "";
        return `${window.location.protocol}//${window.location.hostname}:8080`;
      }
    }
    return "http://localhost:8080";
  })();
  RA.API_BASE = API_BASE;

  /* ── 1. Storage Helper ── */
  const Storage = {
    get(key, fallback = null) {
      try {
        const val = localStorage.getItem("ra-" + key);
        return val ? JSON.parse(val) : fallback;
      } catch (e) {
        return fallback;
      }
    },
    set(key, val) {
      try {
        localStorage.setItem("ra-" + key, JSON.stringify(val));
      } catch (e) {}
    },
    remove(key) {
      try {
        localStorage.removeItem("ra-" + key);
      } catch (e) {}
    }
  };
  RA.Storage = Storage;

  /* ── 2. Theme Management ── */
  RA.initTheme = function () {
    const saved = Storage.get("theme", "dark");
    document.documentElement.setAttribute("data-theme", saved);
    RA.updateThemeToggleIcon(saved);

    const toggleBtn = document.getElementById("ra-theme-toggle");
    if (toggleBtn) {
      toggleBtn.addEventListener("click", () => {
        const current = document.documentElement.getAttribute("data-theme") || "dark";
        const next = current === "dark" ? "light" : "dark";
        document.documentElement.setAttribute("data-theme", next);
        Storage.set("theme", next);
        RA.updateThemeToggleIcon(next);
      });
    }
  };

  RA.updateThemeToggleIcon = function (theme) {
    const toggleBtn = document.getElementById("ra-theme-toggle");
    if (!toggleBtn) return;
    if (theme === "dark") {
      toggleBtn.innerHTML = `
        <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="5"></circle>
          <line x1="12" y1="1" x2="12" y2="3"></line>
          <line x1="12" y1="21" x2="12" y2="23"></line>
          <line x1="4.22" y1="4.22" x2="5.64" y2="5.64"></line>
          <line x1="18.36" y1="18.36" x2="19.78" y2="19.78"></line>
          <line x1="1" y1="12" x2="3" y2="12"></line>
          <line x1="21" y1="12" x2="23" y2="12"></line>
          <line x1="4.22" y1="19.78" x2="5.64" y2="18.36"></line>
          <line x1="18.36" y1="5.64" x2="19.78" y2="4.22"></line>
        </svg>`;
      toggleBtn.setAttribute("aria-label", "Switch to light theme");
    } else {
      toggleBtn.innerHTML = `
        <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"></path>
        </svg>`;
      toggleBtn.setAttribute("aria-label", "Switch to dark theme");
    }
  };

  /* ── 3. Health & Status Polling ── */
  RA.initStatus = async function () {
    const pill = document.querySelector(".ra-status-pill");
    try {
      const res = await fetch(API_BASE + "/health");
      if (!res.ok) throw new Error("Health check failed");
      const data = await res.json();
      if (pill) {
        const count = data.indexed_chunks ? ` · ${data.indexed_chunks.toLocaleString()} vectors` : "";
        pill.innerHTML = `<span class="ra-status-dot" aria-hidden="true"></span> index ok${count}`;
      }
      // Update stats cards if present
      const statVec = document.querySelector("[data-stat='vectors']");
      if (statVec && data.indexed_chunks) {
        statVec.textContent = data.indexed_chunks.toLocaleString();
      }
    } catch (err) {
      if (pill) {
        pill.style.backgroundColor = "var(--warning-soft)";
        pill.style.color = "var(--warning)";
        pill.innerHTML = `<span class="ra-status-dot" style="background-color: var(--warning)"></span> connecting…`;
      }
    }
  };

  /* ── 4. Deterministic Speaker Colors ── */
  RA.speakerHue = function (name) {
    let h = 2166136261;
    const str = String(name || "").toLowerCase().trim();
    for (let i = 0; i < str.length; i++) {
      h ^= str.charCodeAt(i);
      h = Math.imul(h, 16777619);
    }
    return Math.abs(h % 360);
  };

  /* ── 5. Simple Markdown Parser ── */
  RA.renderMarkdown = function (md) {
    if (!md) return "";
    let html = md
      // Escaping HTML tags for security
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;");

    // Headers
    html = html.replace(/^### (.*$)/gim, "<h3>$1</h3>");
    html = html.replace(/^## (.*$)/gim, "<h2>$1</h2>");
    html = html.replace(/^# (.*$)/gim, "<h2>$1</h2>");

    // Bold & Italic
    html = html.replace(/\*\*\*(.*?)\*\*\*/gim, "<strong><em>$1</em></strong>");
    html = html.replace(/\*\*(.*?)\*\*/gim, "<strong>$1</strong>");
    html = html.replace(/\*(.*?)\*/gim, "<em>$1</em>");

    // Bullet lists
    html = html.replace(/^\* (.*$)/gim, "<li>$1</li>");
    html = html.replace(/^- (.*$)/gim, "<li>$1</li>");
    html = html.replace(/((?:<li>.*?<\/li>\s*)+)/gi, "<ul>$1</ul>");

    // Paragraph breaks
    html = html.replace(/\n\n+/g, "</p><p>");
    html = "<p>" + html + "</p>";
    html = html.replace(/<p><\/p>/g, "");
    html = html.replace(/<p>(<h[23]>)/g, "$1");
    html = html.replace(/(<\/h[23]>)<\/p>/g, "$1");
    html = html.replace(/<p>(<ul>)/g, "$1");
    html = html.replace(/(<\/ul>)<\/p>/g, "$1");

    // Citations [1], [2], [3] -> clickable badges
    html = html.replace(/\[(\d+)\]/g, (match, p1) => {
      return `<a href="#source-${p1}" class="ra-cite-link" data-cite="${p1}" title="View source ${p1}">[${p1}]</a>`;
    });

    return html;
  };

  /* ── 6. Ask the Archive (Reader Controller) ── */
  RA.initReader = function () {
    const readerLayout = document.querySelector(".ra-reader-layout");
    if (!readerLayout) return;

    let rawThreads = Storage.get("threads", []);
    let threads = Array.isArray(rawThreads) ? rawThreads : [];
    let currentThreadId = Storage.get("active-thread-id", null);
    let previewSources = Storage.get("preview-sources", true);

    const newThreadBtn = document.getElementById("ra-new-thread-btn");
    const threadFilterInput = document.getElementById("ra-thread-filter");
    const historyList = document.getElementById("ra-history-list");
    const clearHistoryBtn = document.getElementById("ra-clear-history-btn");

    const initialView = document.getElementById("ra-initial-view");
    const conversationView = document.getElementById("ra-conversation-view");
    const threadHead = document.getElementById("ra-thread-head-info");
    const exchangeContainer = document.getElementById("ra-exchange-container");

    const initialComposer = document.getElementById("ra-initial-composer");
    const initialTextarea = document.getElementById("ra-question-input");
    const initialCounter = document.getElementById("ra-char-counter");
    const initialAskBtn = document.getElementById("ra-ask-btn");
    const previewToggleBtn = document.getElementById("ra-preview-toggle-btn");

    const dockComposer = document.getElementById("ra-dock-composer");
    const dockTextarea = document.getElementById("ra-dock-input");
    const dockAskBtn = document.getElementById("ra-dock-ask-btn");

    const newTopicBtn = document.getElementById("ra-new-topic-btn");
    const exportBtn = document.getElementById("ra-export-btn");

    // Preview Sources Toggle
    if (previewToggleBtn) {
      previewToggleBtn.setAttribute("aria-pressed", String(previewSources));
      previewToggleBtn.addEventListener("click", () => {
        previewSources = !previewSources;
        Storage.set("preview-sources", previewSources);
        previewToggleBtn.setAttribute("aria-pressed", String(previewSources));
      });
    }

    // Char counter
    if (initialTextarea && initialCounter) {
      initialTextarea.addEventListener("input", () => {
        initialCounter.textContent = `${initialTextarea.value.length} / 1000`;
      });
    }

    // Render Recent Threads
    function renderHistory(filterText = "") {
      if (!historyList) return;
      historyList.innerHTML = "";
      const filtered = threads.filter(t => t.title.toLowerCase().includes(filterText.toLowerCase()));
      if (filtered.length === 0) {
        historyList.innerHTML = `<li style="padding: 12px 16px; font-size: 12px; color: var(--text-muted);">No threads found</li>`;
        return;
      }
      filtered.forEach(t => {
        const li = document.createElement("li");
        const btn = document.createElement("button");
        btn.className = `ra-history-item ${t.id === currentThreadId ? "active" : ""}`;
        btn.innerHTML = `
          <span class="ra-history-item-title">${escapeHTML(t.title)}</span>
          <span class="ra-history-item-meta">${formatRelativeTime(t.updatedAt || t.createdAt)}</span>
        `;
        btn.addEventListener("click", () => loadThread(t.id));
        li.appendChild(btn);
        historyList.appendChild(li);
      });
    }

    // Load Specific Thread
    function loadThread(id) {
      if (!Array.isArray(threads)) threads = [];
      const thread = threads.find(t => t && t.id === id);
      if (!thread) {
        currentThreadId = null;
        Storage.set("active-thread-id", null);
        showInitialView();
        return;
      }
      currentThreadId = id;
      Storage.set("active-thread-id", id);
      renderHistory();

      initialView.style.display = "none";
      conversationView.style.display = "flex";
      dockComposer.style.display = "block";

      const exchangeCount = Math.floor(((thread.messages ? thread.messages.length : 0) + 1) / 2);
      threadHead.textContent = `CONVERSATION · ${exchangeCount} EXCHANGE${exchangeCount > 1 ? "S" : ""}`;
      exchangeContainer.innerHTML = "";

      if (Array.isArray(thread.messages)) {
        thread.messages.forEach(msg => {
          if (!msg) return;
          if (msg.role === "user") {
            exchangeContainer.appendChild(createUserTurnElement(msg.content, msg.timestamp));
          } else if (msg.role === "assistant") {
            exchangeContainer.appendChild(createAssistantTurnElement(msg.content, msg.sources || [], msg.meta || {}));
          }
        });
      }

      // Scroll to bottom
      window.scrollTo({ top: document.body.scrollHeight, behavior: "smooth" });
    }

    function showInitialView() {
      currentThreadId = null;
      Storage.set("active-thread-id", null);
      renderHistory();
      initialView.style.display = "flex";
      conversationView.style.display = "none";
      dockComposer.style.display = "none";
      if (initialTextarea) {
        initialTextarea.value = "";
        initialTextarea.focus();
      }
    }

    // Create New Thread
    if (newThreadBtn) {
      newThreadBtn.addEventListener("click", showInitialView);
    }
    if (newTopicBtn) {
      newTopicBtn.addEventListener("click", showInitialView);
    }

    // Clear History
    if (clearHistoryBtn) {
      clearHistoryBtn.addEventListener("click", () => {
        if (confirm("Clear all conversation history?")) {
          threads = [];
          Storage.set("threads", []);
          showInitialView();
        }
      });
    }

    // Filter threads
    if (threadFilterInput) {
      threadFilterInput.addEventListener("input", (e) => {
        renderHistory(e.target.value);
      });
    }

    // Suggestion pills
    document.querySelectorAll(".ra-suggestion-btn").forEach(btn => {
      btn.addEventListener("click", () => {
        const text = btn.getAttribute("data-prompt") || btn.textContent.trim();
        if (initialTextarea) {
          initialTextarea.value = text;
          submitQuestion(text);
        }
      });
    });

    // Submitting a Question
    async function submitQuestion(questionText) {
      if (!questionText || !questionText.trim()) return;
      const q = questionText.trim();

      // Disable inputs while pending
      if (initialAskBtn) initialAskBtn.disabled = true;
      if (dockAskBtn) dockAskBtn.disabled = true;

      // Determine or create thread
      if (!Array.isArray(threads)) threads = [];
      let thread = threads.find(t => t && t.id === currentThreadId);
      if (!thread) {
        thread = {
          id: "th-" + Date.now().toString(36),
          title: q.slice(0, 48) + (q.length > 48 ? "…" : ""),
          createdAt: Date.now(),
          updatedAt: Date.now(),
          messages: []
        };
        threads.unshift(thread);
        currentThreadId = thread.id;
        Storage.set("threads", threads);
        Storage.set("active-thread-id", thread.id);
      }
      if (!Array.isArray(thread.messages)) thread.messages = [];

      // Add user message
      const userMsg = {
        role: "user",
        content: q,
        timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) + " · " + new Date().toLocaleDateString([], { day: 'numeric', month: 'short', year: 'numeric' })
      };
      thread.messages.push(userMsg);
      thread.updatedAt = Date.now();
      Storage.set("threads", threads);

      // Render conversation view
      initialView.style.display = "none";
      conversationView.style.display = "flex";
      dockComposer.style.display = "block";
      renderHistory();

      // Append user turn
      exchangeContainer.appendChild(createUserTurnElement(userMsg.content, userMsg.timestamp));

      // Append temporary loading state with progress ticker
      const loadingEl = document.createElement("div");
      loadingEl.className = "ra-assistant-turn";
      loadingEl.innerHTML = `
        <div class="ra-answer-header">
          <span class="ra-badge ra-badge-accent">RETRIEVING LORE…</span>
          <span id="ra-loading-status">Searching vector database & synthesizing answer…</span>
        </div>
        <div class="ra-answer-prose" style="padding: 32px; display: flex; align-items: center; gap: 12px; font-family: var(--font-sans); font-size: 14px; color: var(--text-secondary);">
          <div class="ra-status-dot" style="background-color: var(--accent); animation: pulse-dot 1s infinite ease-in-out;"></div>
          <span>Consulting Wuthering Waves records with Reciprocal Rank Fusion…</span>
        </div>
      `;
      exchangeContainer.appendChild(loadingEl);
      window.scrollTo({ top: document.body.scrollHeight, behavior: "smooth" });

      const startTime = Date.now();
      const progressTimer = setInterval(() => {
        const sec = Math.floor((Date.now() - startTime) / 1000);
        const statusEl = loadingEl.querySelector("#ra-loading-status");
        if (statusEl) {
          if (sec > 10) {
            statusEl.textContent = `Synthesizing grounded response (${sec}s elapsed)…`;
          } else if (sec > 3) {
            statusEl.textContent = `Ranking dialogue evidence with hybrid fusion (${sec}s)…`;
          }
        }
      }, 1000);

      try {
        // Build conversation history payload for multi-turn follow-up
        const historyPayload = (thread.messages || []).slice(0, -1)
          .filter(m => m && (m.role === "user" || m.role === "assistant") && m.content)
          .map(m => ({
            role: m.role,
            content: String(m.content)
          }));

        const t0 = performance.now();
        const res = await fetch(API_BASE + "/api/v1/chat", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ question: q, history: historyPayload })
        });

        if (!res.ok) {
          const errText = await res.text();
          let msg = `API responded with status ${res.status}`;
          try {
            const errJson = JSON.parse(errText);
            if (errJson && errJson.error) msg = errJson.error;
          } catch (_) {}
          throw new Error(msg);
        }

        const data = await res.json();
        const latencyMs = Math.round(data.latency_ms || (performance.now() - t0));

        const assistantMsg = {
          role: "assistant",
          content: data.answer || "No response received.",
          sources: Array.isArray(data.sources) ? data.sources : [],
          meta: {
            model: data.model || "wuwa-lore-model",
            latencyMs: latencyMs,
            tokens: data.total_tokens || 0
          }
        };

        thread.messages.push(assistantMsg);
        Storage.set("threads", threads);

        clearInterval(progressTimer);

        // Replace loading state with actual answer
        try {
          const answerNode = createAssistantTurnElement(assistantMsg.content, assistantMsg.sources, assistantMsg.meta);
          loadingEl.replaceWith(answerNode);
        } catch (renderErr) {
          console.error("Error rendering assistant turn:", renderErr);
          loadingEl.innerHTML = `
            <div class="ra-answer-header">
              <span class="ra-badge ra-badge-accent">ANSWER</span>
              <span>${escapeHTML(assistantMsg.meta.model)}</span>
            </div>
            <div class="ra-answer-prose">
              ${RA.renderMarkdown(assistantMsg.content)}
            </div>
          `;
        }

        // Clear input textareas
        if (initialTextarea) initialTextarea.value = "";
        if (dockTextarea) dockTextarea.value = "";

        // Update exchange count
        const exchangeCount = Math.floor((thread.messages.length + 1) / 2);
        threadHead.textContent = `CONVERSATION · ${exchangeCount} EXCHANGE${exchangeCount > 1 ? "S" : ""}`;
      } catch (err) {
        clearInterval(progressTimer);
        console.error("Chat request failed:", err);
        let errorDetail = err.message || "An unexpected error occurred while querying the lore engine.";
        if (err.name === "TypeError" && (err.message.includes("fetch") || err.message.includes("network") || err.message.includes("Failed to fetch"))) {
          errorDetail = `Could not connect to the API server at ${API_BASE || window.location.origin}. Please ensure the backend daemon (bin/api) is running.`;
        }

        loadingEl.innerHTML = `
          <div class="ra-answer-header" style="background-color: var(--danger-soft); color: var(--danger);">
            <span class="ra-badge ra-badge-danger">QUERY ERROR</span>
            <span>Failed to reach knowledge engine</span>
          </div>
          <div class="ra-answer-prose" style="color: var(--danger); font-family: var(--font-sans); font-size: 14px; padding: 24px;">
            <p style="margin-bottom: 16px; line-height: 1.5;">${escapeHTML(errorDetail)}</p>
            <div style="display: flex; gap: 8px;">
              <button class="ra-btn ra-btn-secondary ra-btn-sm" type="button" onclick="window.RA.retryQuestion(${JSON.stringify(q)})">
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l5.67-5.67"></path></svg>
                <span>Retry question</span>
              </button>
            </div>
          </div>
        `;
      } finally {
        clearInterval(progressTimer);
        if (initialAskBtn) initialAskBtn.disabled = false;
        if (dockAskBtn) dockAskBtn.disabled = false;
        window.scrollTo({ top: document.body.scrollHeight, behavior: "smooth" });
      }
    }

    RA.retryQuestion = function (q) {
      if (initialTextarea) initialTextarea.value = q;
      if (dockTextarea) dockTextarea.value = q;
      submitQuestion(q);
    };

    // Initial Ask Button
    if (initialAskBtn && initialTextarea) {
      initialAskBtn.addEventListener("click", () => submitQuestion(initialTextarea.value));
      initialTextarea.addEventListener("keydown", (e) => {
        if (e.key === "Enter" && !e.shiftKey) {
          e.preventDefault();
          submitQuestion(initialTextarea.value);
        }
      });
    }

    // Dock Ask Button
    if (dockAskBtn && dockTextarea) {
      dockAskBtn.addEventListener("click", () => submitQuestion(dockTextarea.value));
      dockTextarea.addEventListener("keydown", (e) => {
        if (e.key === "Enter" && !e.shiftKey) {
          e.preventDefault();
          submitQuestion(dockTextarea.value);
        }
      });
    }

    // Global Cmd+K / Ctrl+K Shortcut
    window.addEventListener("keydown", (e) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        showInitialView();
      }
    });

    // Export Thread
    if (exportBtn) {
      exportBtn.addEventListener("click", () => {
        const thread = threads.find(t => t.id === currentThreadId);
        if (!thread) return;
        let md = `# ${thread.title}\n\n`;
        thread.messages.forEach(m => {
          md += `### ${m.role === "user" ? "You (Rover)" : "Resonance Archive"}\n\n${m.content}\n\n`;
        });
        const blob = new Blob([md], { type: "text/markdown" });
        const url = URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = `resonance-archive-${thread.id}.md`;
        a.click();
        URL.revokeObjectURL(url);
      });
    }

    // Initial Render & URL Query Parameter Support (?q=... / ?question=...)
    renderHistory();
    const urlParams = new URLSearchParams(window.location.search);
    const initialQ = urlParams.get("q") || urlParams.get("question");

    if (initialQ && initialQ.trim()) {
      showInitialView();
      if (initialTextarea) initialTextarea.value = initialQ.trim();
      submitQuestion(initialQ.trim());
    } else if (currentThreadId) {
      loadThread(currentThreadId);
    } else {
      showInitialView();
    }
  };

  /* Helper: Create User Turn DOM Element */
  function createUserTurnElement(text, timestamp) {
    const el = document.createElement("div");
    el.className = "ra-user-turn";
    el.innerHTML = `
      <div class="ra-turn-header">
        <span style="display: flex; align-items: center; gap: 6px;">
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"></path></svg>
          YOU (ROVER)
        </span>
        <span style="color: var(--text-muted); font-size: 11px;">${escapeHTML(timestamp || "")}</span>
      </div>
      <div class="ra-question-text">${escapeHTML(text)}</div>
    `;
    return el;
  }

  /* Helper: Create Assistant Turn DOM Element */
  function createAssistantTurnElement(answerMd, sources = [], meta = {}) {
    const el = document.createElement("div");
    el.className = "ra-assistant-turn";

    const modelName = (meta && meta.model) ? meta.model : "wuwa-lore-model";
    const latency = (meta && meta.latencyMs) ? `${meta.latencyMs.toLocaleString()} ms` : "";
    const tokens = (meta && meta.tokens) ? ` · ${meta.tokens.toLocaleString()} tok` : "";

    const renderedHtml = RA.renderMarkdown(answerMd);

    let sourcesHtml = "";
    if (Array.isArray(sources) && sources.length > 0) {
      let cardsHtml = "";
      sources.forEach((src, idx) => {
        if (!src) return;
        const num = idx + 1;
        const rawScore = Number(src.score != null ? src.score : 0.85);
        const scoreNum = !isNaN(rawScore) ? rawScore : 0.85;
        const scorePct = Math.min(100, Math.max(5, Math.round(scoreNum * 100)));
        const datasetTag = src.dataset_code === "lore-corpus-v1" ? "WORLD LORE" : "MAIN QUEST";
        const tagClass = src.dataset_code === "lore-corpus-v1" ? "ra-badge-accent" : "ra-badge-neutral";

        let speakersHtml = "";
        const speakers = Array.isArray(src.speakers) ? src.speakers : (src.speaker ? [src.speaker] : []);
        if (speakers.length > 0) {
          speakersHtml = `<div style="display: flex; gap: 4px; flex-wrap: wrap; margin-top: 4px;">` +
            speakers.map(sp => {
              const hue = RA.speakerHue(sp);
              return `<span class="ra-badge" style="background-color: hsl(${hue}, 40%, 88%); color: hsl(${hue}, 80%, 25%);">${escapeHTML(sp)}</span>`;
            }).join("") + `</div>`;
        }

        cardsHtml += `
          <div class="ra-source-card-expanded" id="source-${num}">
            <div class="ra-source-head">
              <span class="ra-source-index">${num}</span>
              <div class="ra-source-meta">
                <div style="display: flex; align-items: center; gap: 8px;">
                  <span class="ra-source-title">${escapeHTML(src.title || src.page_title || "Unknown Document")}</span>
                  <span class="ra-badge ${tagClass}">${datasetTag}</span>
                </div>
                <div class="ra-source-sub">${escapeHTML(src.section || src.section_title || "")} ${src.subsection ? `· ${escapeHTML(src.subsection)}` : ""}</div>
                ${speakersHtml}
              </div>
              <div class="ra-source-score">
                <span>${scoreNum.toFixed(2)}</span>
                <span class="ra-badge ra-badge-accent">RRF ${num}</span>
              </div>
            </div>
            <div class="ra-score-bar-bg">
              <div class="ra-score-bar-fill" style="width: ${scorePct}%;"></div>
            </div>
            <div class="ra-source-text-box">${escapeHTML(src.content || src.chunk_text || "")}</div>
          </div>
        `;
      });

      sourcesHtml = `
        <div class="ra-sources-drawer">
          <div class="ra-sources-drawer-head" onclick="this.nextElementSibling.style.display = this.nextElementSibling.style.display === 'none' ? 'flex' : 'none'">
            <span class="ra-eyebrow" style="color: var(--text-primary); font-size: 11px;">EVIDENCE DOCUMENTS (${sources.length} PASSAGES)</span>
            <span style="font-family: var(--font-mono); font-size: 11px; color: var(--text-muted);">Click to toggle ▾</span>
          </div>
          <div class="ra-sources-drawer-body">
            ${cardsHtml}
          </div>
        </div>
      `;
    }

    el.innerHTML = `
      <div class="ra-answer-header">
        <span class="ra-badge ra-badge-accent">ANSWER</span>
        <span>${escapeHTML(modelName)} · ${latency}${tokens}</span>
      </div>
      <div class="ra-answer-prose">
        ${renderedHtml}
      </div>
      ${sourcesHtml}
    `;

    return el;
  }

  /* ── 7. Semantic Search Controller ── */
  RA.initSearch = function () {
    const searchInput = document.getElementById("ra-search-query");
    const searchBtn = document.getElementById("ra-search-btn");
    const limitMinus = document.getElementById("ra-limit-minus");
    const limitPlus = document.getElementById("ra-limit-plus");
    const limitVal = document.getElementById("ra-limit-val");
    const resultsContainer = document.getElementById("ra-search-results");
    const searchMeta = document.getElementById("ra-search-meta");

    if (!searchInput || !resultsContainer) return;

    let limit = 5;

    if (limitMinus && limitPlus && limitVal) {
      limitMinus.addEventListener("click", () => {
        if (limit > 1) {
          limit--;
          limitVal.textContent = limit;
        }
      });
      limitPlus.addEventListener("click", () => {
        if (limit < 20) {
          limit++;
          limitVal.textContent = limit;
        }
      });
    }

    async function executeSearch() {
      const q = searchInput.value.trim();
      if (!q) return;

      if (searchBtn) searchBtn.disabled = true;
      resultsContainer.innerHTML = `
        <div style="padding: 48px; text-align: center; color: var(--text-secondary); font-family: var(--font-mono);">
          <div class="ra-status-dot" style="margin: 0 auto 12px; background-color: var(--accent); animation: pulse-dot 1s infinite ease-in-out;"></div>
          Executing vector search and lexical scroll…
        </div>
      `;

      try {
        const t0 = performance.now();
        const res = await fetch(API_BASE + "/api/v1/search", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ query: q, limit: limit })
        });

        if (!res.ok) throw new Error(`HTTP error ${res.status}`);
        const data = await res.json();
        const latencyMs = Math.round(performance.now() - t0);

        const items = data.results || data.sources || (Array.isArray(data) ? data : []);
        if (searchMeta) {
          searchMeta.textContent = `Found ${items.length} passages in ${latencyMs} ms · Query: "${q}"`;
        }

        if (items.length === 0) {
          resultsContainer.innerHTML = `
            <div class="ra-card" style="text-align: center; padding: 48px;">
              <h3 class="ra-card-title">No matching lore passages found</h3>
              <p class="ra-card-body">Try rephrasing your search query or expanding terms.</p>
            </div>
          `;
          return;
        }

        resultsContainer.innerHTML = "";
        items.forEach((item, idx) => {
          const num = idx + 1;
          const score = item.score ? Number(item.score).toFixed(2) : "0.85";
          const scorePct = Math.round(Number(item.score || 0.85) * 100);
          const datasetTag = item.dataset_code === "lore-corpus-v1" ? "WORLD LORE" : "MAIN QUEST";
          const tagClass = item.dataset_code === "lore-corpus-v1" ? "ra-badge-accent" : "ra-badge-neutral";

          let speakersHtml = "";
          if (item.speakers && item.speakers.length > 0) {
            speakersHtml = `<div style="display: flex; gap: 4px; flex-wrap: wrap; margin-top: 6px;">` +
              item.speakers.map(sp => {
                const hue = RA.speakerHue(sp);
                return `<span class="ra-badge" style="background-color: hsl(${hue}, 40%, 88%); color: hsl(${hue}, 80%, 25%);">${escapeHTML(sp)}</span>`;
              }).join("") + `</div>`;
          }

          const card = document.createElement("div");
          card.className = "ra-search-result-card";
          card.innerHTML = `
            <div class="ra-source-head">
              <span class="ra-source-index">${num}</span>
              <div class="ra-source-meta">
                <div style="display: flex; align-items: center; gap: 8px;">
                  <span class="ra-source-title">${escapeHTML(item.title || item.page_title || "Unknown Document")}</span>
                  <span class="ra-badge ${tagClass}">${datasetTag}</span>
                </div>
                <div class="ra-source-sub">${escapeHTML(item.section || item.section_title || "")} ${item.subsection ? `· ${escapeHTML(item.subsection)}` : ""}</div>
                ${speakersHtml}
              </div>
              <div class="ra-source-score">
                <span>${score}</span>
                <span class="ra-badge ra-badge-accent">RRF ${num}</span>
              </div>
            </div>
            <div class="ra-score-bar-bg">
              <div class="ra-score-bar-fill" style="width: ${scorePct}%;"></div>
            </div>
            <div class="ra-source-text-box" style="max-height: none;">${escapeHTML(item.content || item.chunk_text || "")}</div>
          `;
          resultsContainer.appendChild(card);
        });
      } catch (err) {
        resultsContainer.innerHTML = `
          <div class="ra-card" style="border-color: var(--danger); background-color: var(--danger-soft);">
            <div class="ra-card-title" style="color: var(--danger);">Search Error</div>
            <div class="ra-card-body" style="color: var(--danger);">${escapeHTML(err.message)}</div>
          </div>
        `;
      } finally {
        if (searchBtn) searchBtn.disabled = false;
      }
    }

    if (searchBtn) searchBtn.addEventListener("click", executeSearch);
    searchInput.addEventListener("keydown", (e) => {
      if (e.key === "Enter") executeSearch();
    });

    // If query in URL params (?q=...)
    const urlParams = new URLSearchParams(window.location.search);
    const initialQuery = urlParams.get("q");
    if (initialQuery) {
      searchInput.value = initialQuery;
      executeSearch();
    }
  };

  /* ── 8. Utility Helpers ── */
  function escapeHTML(str) {
    if (!str) return "";
    return String(str)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;")
      .replace(/'/g, "&#039;");
  }

  function formatRelativeTime(ts) {
    if (!ts) return "recently";
    const diffSec = Math.floor((Date.now() - ts) / 1000);
    if (diffSec < 60) return "just now";
    if (diffSec < 3600) return `${Math.floor(diffSec / 60)} min ago`;
    if (diffSec < 86400) return `${Math.floor(diffSec / 3600)} h ago`;
    if (diffSec < 604800) return `${Math.floor(diffSec / 86400)} d ago`;
    return new Date(ts).toLocaleDateString([], { month: "short", day: "numeric" });
  }

  /* ── 9. Global Initialization ── */
  document.addEventListener("DOMContentLoaded", () => {
    RA.initTheme();
    RA.initStatus();
    RA.initReader();
    RA.initSearch();
  });
})();
