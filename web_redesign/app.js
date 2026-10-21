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
      if (window.WUWA_API_BASE) return window.WUWA_API_BASE;
      try {
        const stored = localStorage.getItem("ra-api-base");
        if (stored) return stored;
      } catch (_) {}

      const proto = window.location.protocol;
      const host = window.location.hostname || "localhost";
      const port = window.location.port;

      if (proto === "http:" || proto === "https:") {
        // Direct access on the Go backend port
        if (port === "8080") {
          return "";
        }

        // Standard production web ports (behind reverse proxy / ALB / CloudFront)
        if (port === "" || port === "80" || port === "443") {
          return "";
        }

        // When browsing via a separate local dev server (e.g. python -m http.server 8000,
        // LiveServer 5500, Vite 5173, Node 3000, etc.), route API calls to the Go backend on 8080.
        if (host === "localhost" || host === "127.0.0.1") {
          return `${proto}//${host}:8080`;
        }

        return "";
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

  /* ── 1b. Authentication & User Quota Manager ── */
  const Auth = {
    getToken() {
      try {
        return localStorage.getItem("ra-auth-token") || "";
      } catch (e) {
        return "";
      }
    },
    setToken(token) {
      try {
        if (token) localStorage.setItem("ra-auth-token", token);
        else localStorage.removeItem("ra-auth-token");
      } catch (e) {}
    },
    getUser() {
      return Storage.get("user", null);
    },
    setUser(u) {
      if (u) Storage.set("user", u);
      else Storage.remove("user");
    },
    isAuthenticated() {
      return !!this.getToken();
    },
    isAdmin() {
      const u = this.getUser();
      return !!(u && (u.role === "admin" || (u.role && u.role.toLowerCase() === "admin")));
    },
    logout() {
      this.setToken("");
      this.setUser(null);
      this.updateUI();
      RA.toast("Logged out of session", "info");
      const currentPath = window.location.pathname;
      if (currentPath.includes("ask.html") || currentPath.includes("search.html")) {
        window.location.href = "index.html";
        return;
      }
      if (typeof RA.refreshReader === "function") {
        RA.refreshReader();
      }
    },
    async syncMe() {
      const token = this.getToken();
      if (!token) return null;
      try {
        const res = await fetch(API_BASE + "/api/v1/auth/me", {
          headers: { "Authorization": "Bearer " + token }
        });
        if (res.ok) {
          const data = await res.json();
          if (data && data.user) {
            this.setUser(data.user);
            this.updateUI();
            return data.user;
          }
        } else if (res.status === 401) {
          this.setToken("");
          this.setUser(null);
          this.updateUI();
        }
      } catch (e) {
        console.warn("Failed to sync user state:", e);
      }
      return null;
    },
    updateUI() {
      const u = this.getUser();
      const isAuth = this.isAuthenticated() && !!u;
      const isAdmin = this.isAdmin();

      // 1. Toggle Admin-Only elements (Console / Maintainer links)
      // Hidden by default; visible only when signed in with admin role
      const adminEls = document.querySelectorAll(".ra-admin-only, #ra-console-btn");
      adminEls.forEach(el => {
        if (isAdmin) {
          el.style.display = el.tagName === "A" && el.classList.contains("ra-footer-link") ? "block" : "inline-flex";
        } else {
          el.style.display = "none";
        }
      });

      // 2. Auth widgets in top navigation
      const signInBtn = document.getElementById("ra-sign-in-btn");
      const userProfile = document.getElementById("ra-user-profile");
      const userName = document.getElementById("ra-user-name");
      const quotaBadge = document.getElementById("ra-quota-badge");

      if (signInBtn) signInBtn.style.display = isAuth ? "none" : "inline-flex";
      if (userProfile) userProfile.style.display = isAuth ? "inline-flex" : "none";

      if (isAuth && u) {
        if (userName) userName.textContent = "@" + (u.username || "user");
        if (quotaBadge) {
          if (isAdmin) {
            quotaBadge.textContent = "Admin · Unlimited";
            quotaBadge.className = "ra-quota-badge ra-quota-admin";
          } else {
            const rem = u.questions_remaining !== undefined ? u.questions_remaining : Math.max(0, (u.question_limit || 5) - (u.question_count || 0));
            quotaBadge.textContent = `${rem} / ${u.question_limit || 5} left`;
            quotaBadge.className = `ra-quota-badge ${rem <= 0 ? "ra-quota-exhausted" : ""}`;
          }
        }
      }

      // 3. Quota lock on composer (if standard user with 0 remaining)
      const quotaBanner = document.getElementById("ra-quota-banner");
      const dockQuotaBanner = document.getElementById("ra-dock-quota-banner");
      const askBtn = document.getElementById("ra-ask-btn");
      const dockAskBtn = document.getElementById("ra-dock-ask-btn");
      const qInput = document.getElementById("ra-question-input");
      const dockInput = document.getElementById("ra-dock-input");

      const isQuotaExhausted = isAuth && !isAdmin && (u.questions_remaining !== undefined && u.questions_remaining <= 0);

      if (quotaBanner) quotaBanner.style.display = isQuotaExhausted ? "flex" : "none";
      if (dockQuotaBanner) dockQuotaBanner.style.display = isQuotaExhausted ? "flex" : "none";

      if (isQuotaExhausted) {
        if (askBtn) askBtn.disabled = true;
        if (dockAskBtn) dockAskBtn.disabled = true;
        if (qInput) {
          qInput.disabled = true;
          qInput.placeholder = "Question limit reached (5/5). Contact administrator for additional questions.";
        }
        if (dockInput) {
          dockInput.disabled = true;
          dockInput.placeholder = "Question limit reached (5/5).";
        }
      } else {
        if (qInput && qInput.disabled) {
          qInput.disabled = false;
          qInput.placeholder = "Ask about a character, place, sentinel or quest… (e.g. What is Ovathrax Threnodian?)";
        }
        if (dockInput && dockInput.disabled) {
          dockInput.disabled = false;
          dockInput.placeholder = "Ask a follow-up question (e.g. “What happened next?”, “Tell me more about their fate”)…";
        }
        if (askBtn && qInput && qInput.value.trim()) askBtn.disabled = false;
        if (dockAskBtn && dockInput && dockInput.value.trim()) dockAskBtn.disabled = false;
      }
    }
  };
  RA.Auth = Auth;

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
      const corpusChunks = document.getElementById("ra-corpus-chunks");
      if (corpusChunks && data.indexed_chunks) {
        corpusChunks.textContent = data.indexed_chunks.toLocaleString();
      }
      const corpusCol = document.getElementById("ra-corpus-collection");
      if (corpusCol && data.collection) {
        corpusCol.textContent = data.collection;
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
      .replace(/\r\n/g, "\n")
      // Escaping HTML tags for security
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;");

    // Code spans
    html = html.replace(/`([^`]+)`/g, "<code>$1</code>");

    // Blockquotes
    html = html.replace(/^>\s?(.*$)/gim, "\n\n<blockquote>$1</blockquote>\n\n");

    // Headers
    html = html.replace(/^### (.*$)/gim, "\n\n<h3>$1</h3>\n\n");
    html = html.replace(/^## (.*$)/gim, "\n\n<h2>$1</h2>\n\n");
    html = html.replace(/^# (.*$)/gim, "\n\n<h2>$1</h2>\n\n");

    // Bold & Italic
    html = html.replace(/\*\*\*(.*?)\*\*\*/gim, "<strong><em>$1</em></strong>");
    html = html.replace(/\*\*(.*?)\*\*/gim, "<strong>$1</strong>");
    html = html.replace(/\*(.*?)\*/gim, "<em>$1</em>");

    // Summary callout detection
    html = html.replace(/^(?:(?:In summary|Summary|Key takeaway|Takeaway):?\s*)(.*$)/gim, 
      "\n\n<div class=\"ra-answer-summary-box\"><div class=\"ra-summary-label\"><svg width=\"12\" height=\"12\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2.5\"><polyline points=\"20 6 9 17 4 12\"></polyline></svg>IN SUMMARY</div><p>$1</p></div>\n\n"
    );

    // Bullet lists
    html = html.replace(/^\* (.*$)/gim, "<li>$1</li>");
    html = html.replace(/^- (.*$)/gim, "<li>$1</li>");
    html = html.replace(/((?:<li>.*?<\/li>\s*)+)/gi, (m) => "\n\n<ul>" + m.trim() + "</ul>\n\n");

    // Paragraph blocks (split by double newlines)
    const blocks = html.split(/\n\n+/);
    html = blocks.map(b => {
      b = b.trim();
      if (!b) return "";
      if (/^<(h[1-4]|ul|ol|blockquote|div)/i.test(b)) return b;
      return "<p>" + b.replace(/\n/g, "<br>") + "</p>";
    }).filter(Boolean).join("\n");

    // Citations [1], [2], [3] -> clickable badges with smooth scroll & highlight
    html = html.replace(/\[(\d+)\]/g, (match, p1) => {
      return `<a href="#source-${p1}" class="ra-cite-link" data-cite="${p1}" onclick="window.RA.highlightSource(${p1}, event)" title="Jump to evidence source [${p1}]">[${p1}]</a>`;
    });

    return html;
  };

  /* ── 5b. Authentication Modal & Flow Controller ── */
  RA.initAuth = function () {
    const modal = document.getElementById("ra-auth-modal");
    const openBtn = document.getElementById("ra-sign-in-btn");
    const closeBtn = document.getElementById("ra-auth-modal-close");
    const tabLogin = document.getElementById("ra-tab-login");
    const tabReg = document.getElementById("ra-tab-register");
    const loginForm = document.getElementById("ra-login-form");
    const regForm = document.getElementById("ra-register-form");
    const loginErr = document.getElementById("ra-login-error");
    const regErr = document.getElementById("ra-register-error");
    const logoutBtn = document.getElementById("ra-logout-btn");

    function openModal(mode = "login") {
      if (!modal) return;
      modal.removeAttribute("hidden");
      modal.style.display = "flex";
      setMode(mode);
      // Trigger reflow so browser registers display:flex before adding is-open
      void modal.offsetHeight;
      modal.classList.add("is-open");
      const input = mode === "register" ? document.getElementById("ra-reg-username") : document.getElementById("ra-login-username");
      if (input) setTimeout(() => input.focus(), 60);
    }
    RA.openAuthModal = openModal;

    function closeModal() {
      if (!modal) return;
      modal.classList.remove("is-open");
      setTimeout(() => {
        if (!modal.classList.contains("is-open")) {
          modal.style.display = "none";
        }
      }, 200);
      if (loginErr) loginErr.style.display = "none";
      if (regErr) regErr.style.display = "none";
    }

    function setMode(mode) {
      if (mode === "register") {
        if (tabReg) tabReg.classList.add("active");
        if (tabLogin) tabLogin.classList.remove("active");
        if (regForm) regForm.style.display = "flex";
        if (loginForm) loginForm.style.display = "none";
      } else {
        if (tabLogin) tabLogin.classList.add("active");
        if (tabReg) tabReg.classList.remove("active");
        if (loginForm) loginForm.style.display = "flex";
        if (regForm) regForm.style.display = "none";
      }
    }

    if (openBtn) {
      openBtn.addEventListener("click", () => {
        RA.redirectAfterLogin = null;
        const notice = document.getElementById("ra-auth-notice");
        if (notice) notice.style.display = "none";
        openModal("login");
      });
    }
    if (closeBtn) closeBtn.addEventListener("click", closeModal);
    if (tabLogin) tabLogin.addEventListener("click", () => setMode("login"));
    if (tabReg) tabReg.addEventListener("click", () => setMode("register"));

    if (modal) {
      modal.addEventListener("click", (e) => {
        if (e.target === modal) closeModal();
      });
      document.addEventListener("keydown", (e) => {
        if (e.key === "Escape" && modal.classList.contains("is-open")) closeModal();
      });
    }

    if (logoutBtn) {
      logoutBtn.addEventListener("click", () => Auth.logout());
    }

    // Login Form Submit
    if (loginForm) {
      loginForm.addEventListener("submit", async (e) => {
        e.preventDefault();
        const username = document.getElementById("ra-login-username").value.trim();
        const password = document.getElementById("ra-login-password").value;
        const submitBtn = document.getElementById("ra-login-submit");

        if (loginErr) loginErr.style.display = "none";
        if (submitBtn) submitBtn.disabled = true;

        try {
          const res = await fetch(API_BASE + "/api/v1/auth/login", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ username, password })
          });
          const data = await res.json();
          if (!res.ok) {
            throw new Error(data.message || data.error || "Login failed");
          }

          Auth.setToken(data.token);
          Auth.setUser(data.user);
          Auth.updateUI();
          closeModal();
          RA.toast(`Welcome back, @${data.user.username}!`);

          if (RA.redirectAfterLogin) {
            const dest = RA.redirectAfterLogin;
            RA.redirectAfterLogin = null;
            window.location.href = dest;
            return;
          }

          if (typeof RA.refreshReader === "function") {
            RA.refreshReader();
          }

          if (RA.pendingQuestion && typeof RA.submitPendingQuestion === "function") {
            RA.submitPendingQuestion();
          }
        } catch (err) {
          if (loginErr) {
            loginErr.textContent = err.message;
            loginErr.style.display = "block";
          }
        } finally {
          if (submitBtn) submitBtn.disabled = false;
        }
      });
    }

    // Register Form Submit
    if (regForm) {
      regForm.addEventListener("submit", async (e) => {
        e.preventDefault();
        const username = document.getElementById("ra-reg-username").value.trim();
        const email = document.getElementById("ra-reg-email").value.trim();
        const password = document.getElementById("ra-reg-password").value;
        const submitBtn = document.getElementById("ra-register-submit");

        if (regErr) regErr.style.display = "none";
        if (submitBtn) submitBtn.disabled = true;

        try {
          const res = await fetch(API_BASE + "/api/v1/auth/register", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ username, email, password })
          });
          const data = await res.json();
          if (!res.ok) {
            throw new Error(data.message || data.error || "Registration failed");
          }

          Auth.setToken(data.token);
          Auth.setUser(data.user);
          Auth.updateUI();
          closeModal();
          RA.toast(`Account created! Welcome, @${data.user.username}!`);

          if (RA.redirectAfterLogin) {
            const dest = RA.redirectAfterLogin;
            RA.redirectAfterLogin = null;
            window.location.href = dest;
            return;
          }

          if (typeof RA.refreshReader === "function") {
            RA.refreshReader();
          }

          if (RA.pendingQuestion && typeof RA.submitPendingQuestion === "function") {
            RA.submitPendingQuestion();
          }
        } catch (err) {
          if (regErr) {
            regErr.textContent = err.message;
            regErr.style.display = "block";
          }
        } finally {
          if (submitBtn) submitBtn.disabled = false;
        }
      });
    }

    // Intercept clicks on any buttons/links routing to ask.html or search.html
    document.querySelectorAll('a[href*="ask.html"], a[href*="search.html"], a[href="/ask"], a[href="/search"]').forEach(link => {
      link.addEventListener("click", (e) => {
        if (!Auth.isAuthenticated()) {
          e.preventDefault();
          RA.redirectAfterLogin = link.getAttribute("href");
          const notice = document.getElementById("ra-auth-notice");
          if (notice) {
            notice.textContent = "Please sign in or create an account to access the Ask Reader and Semantic Search.";
            notice.style.display = "block";
          }
          openModal("login");
        }
      });
    });

    // Check URL query parameters for ?auth=required
    try {
      const urlParams = new URLSearchParams(window.location.search);
      if (urlParams.get("auth") === "required") {
        const dest = urlParams.get("redirect") || "ask.html";
        RA.redirectAfterLogin = dest;
        const notice = document.getElementById("ra-auth-notice");
        if (notice) {
          notice.textContent = "Please sign in or create an account to access the Ask Reader and Semantic Search.";
          notice.style.display = "block";
        }
        openModal("login");
      }
    } catch (_) {}

    // Initial sync
    Auth.updateUI();
    if (Auth.isAuthenticated()) {
      Auth.syncMe();
    }
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
    const threadContent = document.querySelector(".ra-thread-content");

    function scrollChatToBottom(smooth = true) {
      if (threadContent) {
        threadContent.scrollTo({
          top: threadContent.scrollHeight,
          behavior: smooth ? "smooth" : "auto"
        });
      }
    }

    const initialComposer = document.getElementById("ra-initial-composer");
    const initialTextarea = document.getElementById("ra-question-input");
    const initialCounter = document.getElementById("ra-char-counter");
    const initialAskBtn = document.getElementById("ra-ask-btn");
    const previewToggleBtn = document.getElementById("ra-preview-toggle-btn");

    const dockComposer = document.getElementById("ra-dock-composer");
    const dockTextarea = document.getElementById("ra-dock-input");
    const dockCounter = document.getElementById("ra-dock-char-counter");
    const dockAskBtn = document.getElementById("ra-dock-ask-btn");
    const dockPreviewBtn = document.getElementById("ra-dock-preview-toggle-btn");

    const newTopicBtn = document.getElementById("ra-new-topic-btn");
    const exportBtn = document.getElementById("ra-export-btn");

    // Context Panel state containers
    const cpInitial = document.getElementById("ra-context-panel-initial");
    const cpAnswered = document.getElementById("ra-context-panel-answered");

    // Spoiler Guard state
    let spoilerGuardActive = Storage.get("spoiler-guard", false);

    function syncSpoilerSwitches() {
      const swInit = document.getElementById("ra-spoiler-toggle-btn");
      const swAns = document.getElementById("ra-spoiler-toggle-btn-ans");
      [swInit, swAns].forEach(btn => {
        if (!btn) return;
        btn.setAttribute("aria-checked", String(spoilerGuardActive));
        if (spoilerGuardActive) {
          btn.classList.add("active");
        } else {
          btn.classList.remove("active");
        }
      });
    }

    function applySpoilerGuardState() {
      const proseElements = document.querySelectorAll(".ra-answer-prose");
      proseElements.forEach(prose => {
        if (spoilerGuardActive) {
          prose.classList.add("ra-spoiler-blurred");
          if (!prose.dataset.spoilerBound) {
            prose.dataset.spoilerBound = "true";
            prose.title = "Spoiler Guard is active · Click to reveal content";
            prose.addEventListener("click", () => {
              if (prose.classList.contains("ra-spoiler-blurred")) {
                prose.classList.add("ra-revealed");
                RA.toast("Spoiler revealed", "info");
              }
            });
          }
        } else {
          prose.classList.remove("ra-spoiler-blurred");
          prose.classList.remove("ra-revealed");
          prose.title = "";
        }
      });
    }

    function toggleSpoilerGuard() {
      spoilerGuardActive = !spoilerGuardActive;
      Storage.set("spoiler-guard", spoilerGuardActive);
      syncSpoilerSwitches();
      applySpoilerGuardState();
      RA.toast(`Spoiler Guard ${spoilerGuardActive ? "enabled (answers blurred)" : "disabled"}`);
    }

    const swInit = document.getElementById("ra-spoiler-toggle-btn");
    const swAns = document.getElementById("ra-spoiler-toggle-btn-ans");
    if (swInit) swInit.addEventListener("click", toggleSpoilerGuard);
    if (swAns) swAns.addEventListener("click", toggleSpoilerGuard);
    syncSpoilerSwitches();

    // Context Panel Answered State Updater
    function updateContextPanelAnswered(msg) {
      if (cpInitial) cpInitial.style.display = "none";
      if (cpAnswered) cpAnswered.style.display = "flex";

      const sources = Array.isArray(msg?.sources) ? msg.sources : [];
      const meta = msg?.meta || {};

      const passagesEl = document.getElementById("ra-ans-stat-passages");
      const speakersEl = document.getElementById("ra-ans-stat-speakers");
      const modelEl = document.getElementById("ra-ans-stat-model");
      const latencyEl = document.getElementById("ra-ans-stat-latency");
      const tokensEl = document.getElementById("ra-ans-stat-tokens");

      if (passagesEl) passagesEl.textContent = String(sources.length || 0);

      // Compute unique speakers across all retrieved passages
      const speakerSet = new Set();
      sources.forEach(s => {
        if (Array.isArray(s.speakers)) s.speakers.forEach(sp => speakerSet.add(sp));
        else if (s.speaker) speakerSet.add(s.speaker);
      });
      if (speakersEl) speakersEl.textContent = String(speakerSet.size);

      if (modelEl) {
        const raw = meta.model || "gemini-3.1-flash-lite";
        if (raw.includes("3.1") || raw.includes("lite") || raw.includes("wuwa-lore")) {
          modelEl.textContent = "Gemini 3.1 Flash Lite";
        } else {
          modelEl.textContent = raw.replace("gemini/", "").replace("gemini-", "Gemini ");
        }
      }
      if (latencyEl) {
        const s = meta.latencyMs ? (meta.latencyMs / 1000).toFixed(1) + " s" : "31.6 s";
        latencyEl.textContent = s;
      }
      if (tokensEl) tokensEl.textContent = meta.tokens ? meta.tokens.toLocaleString() : "10,964";

      // Build citation tiles grid
      const gridEl = document.getElementById("ra-citation-grid");
      const noteEl = document.getElementById("ra-citation-note");
      if (gridEl) {
        gridEl.innerHTML = "";
        const answerContent = String(msg?.content || "");
        let citedCount = 0;

        sources.forEach((_, idx) => {
          const num = idx + 1;
          const isCited = answerContent.includes(`[${num}]`);
          if (isCited) citedCount++;

          const tile = document.createElement("button");
          tile.className = `ra-citation-tile ${isCited ? "cited" : ""}`;
          tile.type = "button";
          tile.textContent = String(num);
          tile.title = isCited ? `Source [${num}] cited inline — click to jump` : `Source [${num}] retrieved — click to jump`;
          tile.addEventListener("click", (e) => {
            window.RA.highlightSource(num, e);
          });
          gridEl.appendChild(tile);
        });

        if (noteEl) {
          if (sources.length === 0) {
            noteEl.textContent = "No passage citations retrieved for this answer.";
          } else if (citedCount > 0) {
            noteEl.textContent = `Markers 1–${citedCount} are cited inline. Sources ${citedCount + 1}–${sources.length} were retrieved but not referenced.`;
          } else {
            noteEl.textContent = `All ${sources.length} sources were retrieved via hybrid reciprocal rank fusion.`;
          }
        }
      }
    }

    // Context Panel Export Buttons
    const panelExportMd = document.getElementById("ra-panel-export-md");
    const panelExportObsidian = document.getElementById("ra-panel-export-obsidian");
    const panelExportCurl = document.getElementById("ra-panel-export-curl");

    if (panelExportMd) {
      panelExportMd.addEventListener("click", () => {
        const thread = threads.find(t => t.id === currentThreadId);
        if (!thread) {
          RA.toast("Start or select a chronicle to export Markdown");
          return;
        }
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
        RA.toast("Chronicle downloaded as Markdown");
      });
    }

    if (panelExportObsidian) {
      panelExportObsidian.addEventListener("click", () => {
        const thread = threads.find(t => t.id === currentThreadId);
        const title = thread ? thread.title : "Wuthering Waves Lore Notes";
        let content = `---\ntitle: "${title}"\ntags: [wuthering-waves, resonance-archive, lore]\ndate: "${new Date().toISOString().slice(0, 10)}"\n---\n\n# ${title}\n\n`;
        if (thread && thread.messages) {
          thread.messages.forEach(m => {
            content += `## ${m.role === "user" ? "Question (Rover)" : "Archive Answer"}\n\n${m.content}\n\n`;
          });
        } else {
          content += `No active thread. Query the archive to export structured lore notes into Obsidian.`;
        }
        RA.copyText(content, "Copied Obsidian markdown note to clipboard");
      });
    }

    if (panelExportCurl) {
      panelExportCurl.addEventListener("click", () => {
        const q = (initialTextarea && initialTextarea.value.trim()) || "What is Ovathrax Threnodian?";
        const curl = `curl -X POST "${API_BASE}/api/v1/chat" \\\n  -H "Content-Type: application/json" \\\n  -d '{"question": "${q}"}'`;
        RA.copyText(curl, "cURL command copied to clipboard");
      });
    }

    // Preview Sources Toggle (Both initial and dock toggles stay synced)
    function syncPreviewButtons() {
      if (previewToggleBtn) previewToggleBtn.setAttribute("aria-pressed", String(previewSources));
      if (dockPreviewBtn) dockPreviewBtn.setAttribute("aria-pressed", String(previewSources));
    }
    if (previewToggleBtn) {
      previewToggleBtn.addEventListener("click", () => {
        previewSources = !previewSources;
        Storage.set("preview-sources", previewSources);
        syncPreviewButtons();
      });
    }
    if (dockPreviewBtn) {
      dockPreviewBtn.addEventListener("click", () => {
        previewSources = !previewSources;
        Storage.set("preview-sources", previewSources);
        syncPreviewButtons();
      });
    }
    syncPreviewButtons();

    // Char counters
    if (initialTextarea && initialCounter) {
      initialTextarea.addEventListener("input", () => {
        initialCounter.textContent = `${initialTextarea.value.length} / 1000`;
      });
    }
    if (dockTextarea && dockCounter) {
      dockTextarea.addEventListener("input", () => {
        dockCounter.textContent = `${dockTextarea.value.length} / 1000`;
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

      // If thread has no messages cached, fetch from backend SQLite
      if ((!thread.messages || thread.messages.length === 0) && Auth.isAuthenticated()) {
        fetch(API_BASE + `/api/v1/threads/${id}`, {
          headers: { "Authorization": "Bearer " + Auth.getToken() }
        })
          .then(r => r.json())
          .then(data => {
            if (data && Array.isArray(data.messages)) {
              thread.messages = data.messages.map(m => {
                let sources = [];
                let meta = {};
                try { if (m.sources) sources = JSON.parse(m.sources); } catch (_) {}
                try { if (m.meta) meta = JSON.parse(m.meta); } catch (_) {}
                return {
                  role: m.role,
                  content: m.content,
                  sources: sources,
                  meta: meta,
                  timestamp: new Date(m.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
                };
              });
              Storage.set("threads", threads);
              if (currentThreadId === id) {
                loadThread(id);
              }
            }
          })
          .catch(err => console.warn("Failed to fetch thread messages:", err));
      }

      initialView.style.display = "none";
      conversationView.style.display = "flex";
      dockComposer.style.display = "block";

      if (cpInitial) cpInitial.style.display = "none";
      if (cpAnswered) cpAnswered.style.display = "flex";

      const exchangeCount = Math.floor(((thread.messages ? thread.messages.length : 0) + 1) / 2);
      threadHead.textContent = `CONVERSATION · ${exchangeCount} EXCHANGE${exchangeCount > 1 ? "S" : ""}`;
      exchangeContainer.innerHTML = "";

      let lastAssistantMsg = null;
      if (Array.isArray(thread.messages)) {
        thread.messages.forEach(msg => {
          if (!msg) return;
          if (msg.role === "user") {
            exchangeContainer.appendChild(createUserTurnElement(msg.content, msg.timestamp));
          } else if (msg.role === "assistant") {
            lastAssistantMsg = msg;
            exchangeContainer.appendChild(createAssistantTurnElement(msg.content, msg.sources || [], msg.meta || {}));
          }
        });
      }

      if (lastAssistantMsg) {
        updateContextPanelAnswered(lastAssistantMsg);
      }
      applySpoilerGuardState();

      // Scroll to bottom of chat
      scrollChatToBottom();
    }

    async function syncThreadsFromServer() {
      if (!Auth.isAuthenticated()) return;
      try {
        const res = await fetch(API_BASE + "/api/v1/threads", {
          headers: { "Authorization": "Bearer " + Auth.getToken() }
        });
        if (res.ok) {
          const data = await res.json();
          if (data && Array.isArray(data.threads)) {
            threads = data.threads.map(st => {
              const local = (threads || []).find(lt => lt.id === st.id);
              return {
                id: st.id,
                title: st.title,
                createdAt: new Date(st.created_at).getTime(),
                updatedAt: new Date(st.updated_at).getTime(),
                messages: (local && local.messages && local.messages.length > 0) ? local.messages : []
              };
            });
            Storage.set("threads", threads);
            renderHistory();
          }
        }
      } catch (e) {
        console.warn("Failed to sync threads from server:", e);
      }
    }
    RA.refreshReader = function() {
      syncThreadsFromServer();
      Auth.updateUI();
    };
    RA.submitPendingQuestion = function() {
      if (RA.pendingQuestion) {
        const pq = RA.pendingQuestion;
        RA.pendingQuestion = null;
        submitQuestion(pq);
      }
    };

    // Initial server thread synchronization
    syncThreadsFromServer();

    function showInitialView() {
      currentThreadId = null;
      Storage.set("active-thread-id", null);
      if (exchangeContainer) exchangeContainer.innerHTML = "";
      if (threadHead) threadHead.textContent = "CONVERSATION · 0 EXCHANGES";
      renderHistory();
      if (initialView) initialView.style.display = "flex";
      if (conversationView) conversationView.style.display = "none";
      if (dockComposer) dockComposer.style.display = "none";

      if (cpInitial) cpInitial.style.display = "flex";
      if (cpAnswered) cpAnswered.style.display = "none";

      if (initialTextarea) {
        initialTextarea.value = "";
        if (initialCounter) initialCounter.textContent = "0 / 1000";
        initialTextarea.focus();
      }
      if (dockTextarea) {
        dockTextarea.value = "";
        if (dockCounter) dockCounter.textContent = "0 / 1000";
      }
    }

    // Create New Thread
    if (newThreadBtn) {
      newThreadBtn.addEventListener("click", showInitialView);
    }
    if (newTopicBtn) {
      newTopicBtn.addEventListener("click", showInitialView);
    }

    // Clear History Modal & All Local Storage
    const clearModal = document.getElementById("ra-clear-modal");
    const clearCancelBtn = document.getElementById("ra-clear-cancel-btn");
    const clearConfirmBtn = document.getElementById("ra-clear-confirm-btn");
    const clearCloseBtn = document.getElementById("ra-clear-modal-close");

    function openClearModal() {
      if (!clearModal) return;
      clearModal.removeAttribute("hidden");
      requestAnimationFrame(() => {
        clearModal.classList.add("is-open");
        if (clearCancelBtn) clearCancelBtn.focus();
      });
    }

    function closeClearModal() {
      if (!clearModal) return;
      clearModal.classList.remove("is-open");
      setTimeout(() => {
        if (!clearModal.classList.contains("is-open")) {
          clearModal.setAttribute("hidden", "");
        }
      }, 200);
      if (clearHistoryBtn) clearHistoryBtn.focus();
    }

    if (clearHistoryBtn) {
      clearHistoryBtn.addEventListener("click", openClearModal);
    }
    if (clearCancelBtn) {
      clearCancelBtn.addEventListener("click", closeClearModal);
    }
    if (clearCloseBtn) {
      clearCloseBtn.addEventListener("click", closeClearModal);
    }
    if (clearModal) {
      clearModal.addEventListener("click", (e) => {
        if (e.target === clearModal) {
          closeClearModal();
        }
      });
    }
    window.addEventListener("keydown", (e) => {
      if (e.key === "Escape" && clearModal && clearModal.classList.contains("is-open")) {
        closeClearModal();
      }
    });

    if (clearConfirmBtn) {
      clearConfirmBtn.addEventListener("click", () => {
        try {
          localStorage.clear();
        } catch (e) {}
        threads = [];
        currentThreadId = null;
        renderHistory();
        showInitialView();
        closeClearModal();
        RA.toast("All conversation history cleared");
      });
    }

    // Filter threads
    if (threadFilterInput) {
      threadFilterInput.addEventListener("input", (e) => {
        renderHistory(e.target.value);
      });
    }

    // Suggestion Buttons and Horizontal Rows
    document.querySelectorAll(".ra-suggestion-btn, .ra-suggestion-row-btn").forEach(btn => {
      btn.addEventListener("click", () => {
        const text = btn.getAttribute("data-prompt") || btn.querySelector(".ra-suggestion-text")?.textContent.trim() || btn.textContent.trim();
        if (text) {
          if (initialTextarea) initialTextarea.value = text;
          submitQuestion(text);
        }
      });
    });

    // Submitting a Question
    async function submitQuestion(questionText) {
      if (!questionText || !questionText.trim()) return;
      const q = questionText.trim();

      // Require authentication to ask questions
      if (!Auth.isAuthenticated()) {
        RA.pendingQuestion = q;
        RA.toast("Please sign in or create an account to ask the archive (5 free questions)", "info");
        RA.openAuthModal("login");
        return;
      }

      // AUTO REMOVE / CLEAR IMMEDIATELY UPON PRESSING ASK
      if (initialTextarea) {
        initialTextarea.value = "";
        if (initialCounter) initialCounter.textContent = "0 / 1000";
      }
      if (dockTextarea) {
        dockTextarea.value = "";
        if (dockCounter) dockCounter.textContent = "0 / 1000";
      }

      // Disable inputs while pending
      if (initialAskBtn) initialAskBtn.disabled = true;
      if (dockAskBtn) dockAskBtn.disabled = true;

      // Determine or create thread
      if (!Array.isArray(threads)) threads = [];
      let thread = threads.find(t => t && t.id === currentThreadId);
      if (!thread) {
        thread = {
          id: "th-" + Date.now().toString(36) + "-" + Math.random().toString(36).slice(2, 6),
          title: q.slice(0, 48) + (q.length > 48 ? "…" : ""),
          createdAt: Date.now(),
          updatedAt: Date.now(),
          messages: []
        };
        threads.unshift(thread);
        currentThreadId = thread.id;
        Storage.set("threads", threads);
        Storage.set("active-thread-id", thread.id);
        if (exchangeContainer) exchangeContainer.innerHTML = "";
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
      const curIdx = threads.findIndex(t => t && t.id === thread.id);
      if (curIdx > 0) {
        threads.splice(curIdx, 1);
        threads.unshift(thread);
      }
      Storage.set("threads", threads);

      // Render conversation view
      initialView.style.display = "none";
      conversationView.style.display = "flex";
      dockComposer.style.display = "block";
      if (cpInitial) cpInitial.style.display = "none";
      if (cpAnswered) cpAnswered.style.display = "flex";
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
      scrollChatToBottom();

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

        const token = Auth.getToken();
        const headers = { "Content-Type": "application/json" };
        if (token) headers["Authorization"] = "Bearer " + token;

        const t0 = performance.now();
        const res = await fetch(API_BASE + "/api/v1/chat", {
          method: "POST",
          headers: headers,
          body: JSON.stringify({
            question: q,
            history: historyPayload,
            thread_id: currentThreadId
          })
        });

        if (!res.ok) {
          const errText = await res.text();
          let msg = `API responded with status ${res.status}`;
          let isQuota = false;
          try {
            const errJson = JSON.parse(errText);
            if (errJson && errJson.error) {
              msg = errJson.message || errJson.error;
              if (errJson.error === "question_limit_reached") isQuota = true;
            }
          } catch (_) {}

          if (isQuota) {
            const user = Auth.getUser();
            if (user) {
              user.questions_remaining = 0;
              Auth.setUser(user);
              Auth.updateUI();
            }
          }
          throw new Error(msg);
        }

        const data = await res.json();
        const latencyMs = Math.round(data.latency_ms || (performance.now() - t0));

        // Update quota tracking
        if (data.questions_remaining !== undefined) {
          const user = Auth.getUser();
          if (user) {
            user.questions_remaining = data.questions_remaining;
            Auth.setUser(user);
            Auth.updateUI();
          }
        }

        if (data.thread_id && thread) {
          thread.id = data.thread_id;
          currentThreadId = data.thread_id;
        }

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
        thread.updatedAt = Date.now();
        Storage.set("threads", threads);
        renderHistory();

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

        // Update Context Panel and apply Spoiler Guard
        updateContextPanelAnswered(assistantMsg);
        applySpoilerGuardState();

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
        scrollChatToBottom();
      }
    }

    RA.retryQuestion = function (q) {
      if (initialTextarea) initialTextarea.value = q;
      if (dockTextarea) dockTextarea.value = q;
      submitQuestion(q);
    };

    // Initial Ask Button & Enter key handler (clears immediately on click)
    if (initialAskBtn && initialTextarea) {
      initialAskBtn.addEventListener("click", () => {
        const val = initialTextarea.value;
        initialTextarea.value = "";
        submitQuestion(val);
      });
      initialTextarea.addEventListener("keydown", (e) => {
        if (e.key === "Enter" && !e.shiftKey) {
          e.preventDefault();
          const val = initialTextarea.value;
          initialTextarea.value = "";
          submitQuestion(val);
        }
      });
    }

    // Dock Ask Button & Enter key handler (clears immediately on click)
    if (dockAskBtn && dockTextarea) {
      dockAskBtn.addEventListener("click", () => {
        const val = dockTextarea.value;
        dockTextarea.value = "";
        submitQuestion(val);
      });
      dockTextarea.addEventListener("keydown", (e) => {
        if (e.key === "Enter" && !e.shiftKey) {
          e.preventDefault();
          const val = dockTextarea.value;
          dockTextarea.value = "";
          submitQuestion(val);
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

    // Initial Render & URL Query Parameter Support (?q=... / ?question=... / #new / ?new)
    renderHistory();
    const urlParams = new URLSearchParams(window.location.search);
    const initialQ = urlParams.get("q") || urlParams.get("question");
    const isExplicitNew = urlParams.has("new") || window.location.hash === "#new";

    if (initialQ && initialQ.trim()) {
      showInitialView();
      if (initialTextarea) initialTextarea.value = initialQ.trim();
      submitQuestion(initialQ.trim());
    } else if (!isExplicitNew && currentThreadId) {
      loadThread(currentThreadId);
    } else {
      showInitialView();
    }

    // Clicking Ask link in topbar when already on ask page should start a new session
    document.querySelectorAll('.ra-nav a[href="ask.html"], .ra-nav a[href="/ask.html"]').forEach(link => {
      link.addEventListener("click", (e) => {
        if (window.location.pathname.endsWith("ask.html") || window.location.pathname === "/ask") {
          e.preventDefault();
          showInitialView();
        }
      });
    });
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

  /* Helper: Speaker Initials */
  RA.speakerInitials = function (name) {
    const parts = String(name || "").trim().split(/\s+/);
    if (parts.length === 0 || !parts[0]) return "·";
    if (parts.length === 1) return parts[0].slice(0, 1).toUpperCase();
    return (parts[0][0] + parts[1][0]).toUpperCase();
  };

  /* Helper: Toast System */
  RA.toast = function (msg, kind = "info") {
    const existing = document.querySelector(".ra-toast");
    if (existing) existing.remove();
    const el = document.createElement("div");
    el.className = "ra-toast";
    el.innerHTML = `
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2">
        <path d="M20 6L9 17l-5-5"></path>
      </svg>
      <span>${escapeHTML(msg)}</span>
    `;
    document.body.appendChild(el);
    setTimeout(() => {
      el.style.opacity = "0";
      el.style.transition = "opacity 0.3s ease";
      setTimeout(() => el.remove(), 300);
    }, 2400);
  };

  /* Helper: Clipboard copy */
  RA.copyText = async function (text, successMsg = "Copied to clipboard") {
    try {
      await navigator.clipboard.writeText(text);
      RA.toast(successMsg);
    } catch (_) {
      const ta = document.createElement("textarea");
      ta.value = text;
      document.body.appendChild(ta);
      ta.select();
      document.execCommand("copy");
      ta.remove();
      RA.toast(successMsg);
    }
  };

  /* Helper: Smooth jump and highlight source */
  RA.highlightSource = function (idx, e) {
    if (e) e.preventDefault();
    const sourceCard = document.getElementById("source-" + idx);
    if (!sourceCard) return;

    const drawer = sourceCard.closest(".ra-sources-drawer");
    const drawerBody = drawer?.querySelector(".ra-sources-drawer-body");
    const toggleText = drawer?.querySelector(".ra-drawer-toggle-text");
    if (drawerBody && drawerBody.style.display === "none") {
      drawerBody.style.display = "flex";
      if (toggleText) toggleText.textContent = "Click to collapse ▾";
    }

    sourceCard.scrollIntoView({ behavior: "smooth", block: "center" });
    sourceCard.classList.remove("ra-source-highlight");
    void sourceCard.offsetWidth;
    sourceCard.classList.add("ra-source-highlight");
  };

  /* Helper: Toggle evidence drawer */
  RA.toggleSources = function (headEl) {
    const body = headEl.nextElementSibling;
    const toggleText = headEl.querySelector(".ra-drawer-toggle-text");
    const isHidden = body.style.display === "none";
    body.style.display = isHidden ? "flex" : "none";
    if (toggleText) {
      toggleText.textContent = isHidden ? "Click to collapse ▾" : "Click to expand ▸";
    }
  };

  /* Helper: Toggle passage text box */
  RA.toggleExcerpt = function (btn) {
    const card = btn.closest(".ra-source-card-expanded") || btn.closest(".ra-search-result-card");
    const textBox = card?.querySelector(".ra-source-text-box");
    if (!textBox) return;
    const isExpanded = textBox.classList.toggle("expanded");
    const label = btn.querySelector("span");
    if (label) label.textContent = isExpanded ? "Collapse" : "Expand";
  };

  /* Helper: Copy single source card excerpt */
  RA.copySourceCard = function (btn) {
    const card = btn.closest(".ra-source-card-expanded") || btn.closest(".ra-search-result-card");
    const title = card?.querySelector(".ra-source-title")?.textContent || "";
    const text = card?.querySelector(".ra-source-text-box")?.textContent || "";
    RA.copyText(`[${title}]\n${text}`, "Passage excerpt copied");
  };

  /* Helper: Copy entire turn answer */
  RA.copyTurnAnswer = function (btn) {
    const turn = btn.closest(".ra-assistant-turn");
    const prose = turn?.querySelector(".ra-answer-prose");
    if (prose) {
      RA.copyText(prose.innerText || prose.textContent, "Answer copied to clipboard");
    }
  };

  /* Helper: Copy answer with all evidence citations */
  RA.copyTurnAnswerWithSources = function (btn) {
    const turn = btn.closest(".ra-assistant-turn");
    const prose = turn?.querySelector(".ra-answer-prose")?.innerText || "";
    const sources = Array.from(turn?.querySelectorAll(".ra-source-card-expanded") || []).map((card, i) => {
      const title = card.querySelector(".ra-source-title")?.innerText || "";
      const text = card.querySelector(".ra-source-text-box")?.innerText || "";
      return `[${i + 1}] ${title}\n${text}`;
    }).join("\n\n");
    const full = `${prose}\n\n---\n### Sources & Evidence:\n${sources}`;
    RA.copyText(full, "Answer + Sources copied to clipboard");
  };

  /* Helper: Export single turn to Markdown file */
  RA.exportCurrentTurn = function (btn) {
    const turn = btn.closest(".ra-assistant-turn");
    const prose = turn?.querySelector(".ra-answer-prose")?.innerText || "";
    const blob = new Blob([prose], { type: "text/markdown" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `resonance-answer-${Date.now().toString(36)}.md`;
    a.click();
    URL.revokeObjectURL(url);
    RA.toast("Answer downloaded as Markdown");
  };

  /* Helper: Focus new question composer */
  RA.focusNewQuestion = function () {
    const dockInput = document.getElementById("ra-dock-input");
    const initialInput = document.getElementById("ra-question-input");
    if (dockInput && dockInput.offsetParent !== null) {
      dockInput.focus();
      dockInput.scrollIntoView({ behavior: "smooth", block: "center" });
    } else if (initialInput) {
      initialInput.focus();
      initialInput.scrollIntoView({ behavior: "smooth", block: "center" });
    }
  };

  /* Helper: Create Assistant Turn DOM Element */
  function createAssistantTurnElement(answerMd, sources = [], meta = {}) {
    const el = document.createElement("div");
    el.className = "ra-assistant-turn";

    let modelName = "gemini-3.1-flash-lite";
    if (meta && meta.model) {
      modelName = meta.model.replace("gemini/", "");
      if (modelName === "wuwa-lore-model") {
        modelName = "gemini-3.1-flash-lite";
      }
    }
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
          speakersHtml = `<div class="ra-speakers-row">` +
            speakers.map(sp => {
              const hue = RA.speakerHue(sp);
              const initials = RA.speakerInitials(sp);
              return `<span class="ra-speaker-tag" style="background-color: hsl(${hue}, 45%, 90%); color: hsl(${hue}, 85%, 22%);">
                <span class="ra-speaker-avatar" style="background-color: hsl(${hue}, 60%, 80%); color: hsl(${hue}, 90%, 15%);">${escapeHTML(initials)}</span>
                <span>${escapeHTML(sp)}</span>
              </span>`;
            }).join("") + `</div>`;
        }

        cardsHtml += `
          <div class="ra-source-card-expanded" id="source-${num}">
            <div class="ra-source-head">
              <div class="ra-source-index">${num}</div>
              <div class="ra-source-meta">
                <div style="display: flex; align-items: center; gap: 8px; flex-wrap: wrap;">
                  <span class="ra-source-title">${escapeHTML(src.title || src.page_title || "Unknown Document")}</span>
                  <span class="ra-badge ${tagClass}">${datasetTag}</span>
                </div>
                <div class="ra-source-sub">${escapeHTML(src.section || src.section_title || "")} ${src.subsection ? `· Scene: ${escapeHTML(src.subsection)}` : ""}</div>
                ${speakersHtml}
              </div>
              <div class="ra-source-score">
                <span>${scoreNum.toFixed(2)}</span>
                <span class="ra-badge ra-badge-accent">RRF ${num}</span>
              </div>
            </div>
            <div class="ra-source-text-box">${escapeHTML(src.content || src.chunk_text || "")}</div>
            <div class="ra-source-action-row">
              <div class="ra-source-score-bar-wrap">
                <span class="ra-eyebrow" style="font-size: 9px; color: var(--text-muted);">SCORE</span>
                <div class="ra-source-score-bar">
                  <div class="ra-source-score-fill" style="width: ${scorePct}%;"></div>
                </div>
                <span style="font-family: var(--font-mono); font-size: 10px; color: var(--text-muted);">${Math.round(scoreNum * 100)}%</span>
              </div>
              <div class="ra-source-actions">
                <a href="about.html" class="ra-source-action-btn" title="View in Archive documentation">
                  <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"></path><polyline points="15 3 21 3 21 9"></polyline><line x1="10" y1="14" x2="21" y2="3"></line></svg>
                  <span>Wiki</span>
                </a>
                <button class="ra-source-action-btn" type="button" onclick="window.RA.toggleExcerpt(this)" title="Toggle full passage view">
                  <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="15 3 21 3 21 9"></polyline><polyline points="9 21 3 21 3 15"></polyline><line x1="21" y1="3" x2="14" y2="10"></line><line x1="3" y1="21" x2="10" y2="14"></line></svg>
                  <span>Expand</span>
                </button>
                <button class="ra-source-action-btn" type="button" onclick="window.RA.copySourceCard(this)" title="Copy passage excerpt">
                  <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg>
                  <span>Copy</span>
                </button>
              </div>
            </div>
          </div>
        `;
      });

      sourcesHtml = `
        <div class="ra-sources-drawer">
          <div class="ra-sources-drawer-head" onclick="window.RA.toggleSources(this)">
            <span class="ra-eyebrow" style="color: var(--text-primary); font-size: 11px; display: flex; align-items: center; gap: 8px;">
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"></path><path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"></path></svg>
              EVIDENCE DOCUMENTS (${sources.length} PASSAGES) · HYBRID RRF
            </span>
            <span class="ra-drawer-toggle-text" style="font-family: var(--font-mono); font-size: 11px; color: var(--text-muted); display: flex; align-items: center; gap: 4px;">
              Click to collapse ▾
            </span>
          </div>
          <div class="ra-sources-drawer-body">
            ${cardsHtml}
          </div>
        </div>
      `;
    }

    const turnActionsHtml = `
      <div class="ra-turn-actions-bar">
        <button class="ra-turn-action-btn" type="button" onclick="window.RA.copyTurnAnswer(this)" title="Copy answer text">
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg>
          <span>Copy answer</span>
        </button>
        <button class="ra-turn-action-btn" type="button" onclick="window.RA.copyTurnAnswerWithSources(this)" title="Copy answer with cited evidence">
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"></path><rect x="8" y="2" width="8" height="4" rx="1" ry="1"></rect></svg>
          <span>Copy + sources</span>
        </button>
        <button class="ra-turn-action-btn" type="button" onclick="window.RA.exportCurrentTurn(this)" title="Export exchange as Markdown">
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path><polyline points="7 10 12 15 17 10"></polyline><line x1="12" y1="15" x2="12" y2="3"></line></svg>
          <span>Export</span>
        </button>
        <div style="flex: 1;"></div>
        <button class="ra-turn-action-btn primary" type="button" onclick="window.RA.focusNewQuestion()">
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><line x1="12" y1="8" x2="12" y2="16"></line><line x1="8" y1="12" x2="16" y2="12"></line></svg>
          <span>New question</span>
        </button>
      </div>
    `;

    el.innerHTML = `
      <div class="ra-answer-header">
        <div class="ra-answer-header-meta">
          <span class="ra-badge ra-badge-accent" style="display: inline-flex; align-items: center; gap: 5px;">
            <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path d="M2 10v3"></path><path d="M6 6v11"></path><path d="M10 3v18"></path><path d="M14 8v7"></path><path d="M18 5v13"></path><path d="M22 10v3"></path></svg>
            ANSWER
          </span>
          <span>${escapeHTML(modelName)} · ${latency}${tokens}</span>
        </div>
        <div class="ra-answer-header-actions">
          <button class="ra-header-action-btn" type="button" onclick="window.RA.copyTurnAnswer(this)" title="Copy answer text">
            <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg>
            <span>Copy</span>
          </button>
        </div>
      </div>
      <div class="ra-answer-prose">
        ${renderedHtml}
      </div>
      ${sourcesHtml}
      ${turnActionsHtml}
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
        const headers = { "Content-Type": "application/json" };
        const token = Auth.getToken();
        if (token) headers["Authorization"] = "Bearer " + token;

        const res = await fetch(API_BASE + "/api/v1/search", {
          method: "POST",
          headers: headers,
          body: JSON.stringify({ query: q, limit: limit })
        });

        if (res.status === 401) {
          Auth.logout();
          alert("Your session has expired. Please log in again.");
          window.location.href = `index.html?auth=required&redirect=${encodeURIComponent(window.location.pathname.split('/').pop() || 'search.html')}`;
          return;
        }

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
          const scoreNum = Number(item.score != null ? item.score : 0.85);
          const scorePct = Math.min(100, Math.max(5, Math.round((!isNaN(scoreNum) ? scoreNum : 0.85) * 100)));
          const datasetTag = item.dataset_code === "lore-corpus-v1" ? "WORLD LORE" : "MAIN QUEST";
          const tagClass = item.dataset_code === "lore-corpus-v1" ? "ra-badge-accent" : "ra-badge-neutral";

          let speakersHtml = "";
          const speakers = Array.isArray(item.speakers) ? item.speakers : (item.speaker ? [item.speaker] : []);
          if (speakers.length > 0) {
            speakersHtml = `<div class="ra-speakers-row">` +
              speakers.map(sp => {
                const hue = RA.speakerHue(sp);
                const initials = RA.speakerInitials(sp);
                return `<span class="ra-speaker-tag" style="background-color: hsl(${hue}, 45%, 90%); color: hsl(${hue}, 85%, 22%);">
                  <span class="ra-speaker-avatar" style="background-color: hsl(${hue}, 60%, 80%); color: hsl(${hue}, 90%, 15%);">${escapeHTML(initials)}</span>
                  <span>${escapeHTML(sp)}</span>
                </span>`;
              }).join("") + `</div>`;
          }

          const card = document.createElement("div");
          card.className = "ra-source-card-expanded ra-search-result-card";
          card.innerHTML = `
            <div class="ra-source-head">
              <div class="ra-source-index">${num}</div>
              <div class="ra-source-meta">
                <div style="display: flex; align-items: center; gap: 8px; flex-wrap: wrap;">
                  <span class="ra-source-title">${escapeHTML(item.title || item.page_title || "Unknown Document")}</span>
                  <span class="ra-badge ${tagClass}">${datasetTag}</span>
                </div>
                <div class="ra-source-sub">${escapeHTML(item.section || item.section_title || "")} ${item.subsection ? `· Scene: ${escapeHTML(item.subsection)}` : ""}</div>
                ${speakersHtml}
              </div>
              <div class="ra-source-score">
                <span>${(!isNaN(scoreNum) ? scoreNum : 0.85).toFixed(2)}</span>
                <span class="ra-badge ra-badge-accent">RRF ${num}</span>
              </div>
            </div>
            <div class="ra-source-text-box" style="max-height: 180px;">${escapeHTML(item.content || item.chunk_text || "")}</div>
            <div class="ra-source-action-row">
              <div class="ra-source-score-bar-wrap">
                <span class="ra-eyebrow" style="font-size: 9px; color: var(--text-muted);">SCORE</span>
                <div class="ra-source-score-bar">
                  <div class="ra-source-score-fill" style="width: ${scorePct}%;"></div>
                </div>
                <span style="font-family: var(--font-mono); font-size: 10px; color: var(--text-muted);">${scorePct}%</span>
              </div>
              <div class="ra-source-actions">
                <button class="ra-source-action-btn" type="button" onclick="window.RA.toggleExcerpt(this)" title="Toggle full passage view">
                  <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="15 3 21 3 21 9"></polyline><polyline points="9 21 3 21 3 15"></polyline><line x1="21" y1="3" x2="14" y2="10"></line><line x1="3" y1="21" x2="10" y2="14"></line></svg>
                  <span>Expand</span>
                </button>
                <button class="ra-source-action-btn" type="button" onclick="window.RA.copySourceCard(this)" title="Copy passage excerpt">
                  <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg>
                  <span>Copy</span>
                </button>
              </div>
            </div>
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
  function initApp() {
    try { RA.initTheme(); } catch (e) { console.error("Theme init error:", e); }
    try { RA.initStatus(); } catch (e) { console.error("Status init error:", e); }
    try { RA.initAuth(); } catch (e) { console.error("Auth init error:", e); }
    try { RA.initReader(); } catch (e) { console.error("Reader init error:", e); }
    try { RA.initSearch(); } catch (e) { console.error("Search init error:", e); }
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", initApp);
  } else {
    initApp();
  }
})();
