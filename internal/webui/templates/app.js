// Wiring for the real app (cmd/gui's web_windows.go): every actionable
// control posts its action name to the local server, which dispatches
// into the same app.go handlers the old walk widgets called directly. No
// element here ever navigates away (no real <a href> to an external site —
// see internal/webui/page.go's prereqCardView doc comment): everything
// that used to be a link is a button whose action opens the system
// browser from the Go side (openBrowser), so the embedded WebView2 itself
// never leaves this page.
//
// Harmless no-ops when this page is served standalone (the preview tool,
// the screenshot harness): fetch() against a server that isn't listening
// for /action/* just rejects silently, and nothing here reads the
// response.
(function () {
  function post(action, body) {
    fetch("/action/" + action, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body || {}),
    }).catch(function () {});
  }

  document.addEventListener("click", function (e) {
    const copy = e.target.closest("[data-copy]");
    if (copy) {
      const text = copy.getAttribute("data-copy");
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).catch(function () {});
      }
      return;
    }
    const el = e.target.closest("[data-action]");
    if (el) post(el.getAttribute("data-action"), {});
  });

  document.addEventListener("change", function (e) {
    const el = e.target.closest("input[type=checkbox][data-action]");
    if (!el) return;
    post(el.getAttribute("data-action"), { checked: el.checked });
  });

  // cmd/gui's render() (web_windows.go) calls chromium.Eval to invoke this
  // after any state change, instead of a full page navigation: it re-fetches
  // the current page and swaps just the <div class="app"> root, so the
  // browser never shows a blank frame between screens.
  window.__refresh = function () {
    fetch("/")
      .then(function (r) {
        return r.text();
      })
      .then(function (html) {
        const doc = new DOMParser().parseFromString(html, "text/html");
        const next = doc.querySelector(".app");
        const current = document.querySelector(".app");
        if (next && current) current.replaceWith(next);
        if (doc.title) document.title = doc.title;
      })
      .catch(function () {});
  };
})();
