document.addEventListener("DOMContentLoaded", () => {
  const toggle = document.querySelector("[data-nav-toggle]");
  const closeBtn = document.querySelector("[data-nav-close]");
  const sidebar = document.querySelector(".sidebar");
  const backdrop = document.querySelector(".sidebar-backdrop");

  if (toggle && sidebar && backdrop) {
    const setOpen = (open) => {
      sidebar.classList.toggle("open", open);
      backdrop.classList.toggle("open", open);
      document.body.classList.toggle("nav-open", open);
      toggle.setAttribute("aria-expanded", open ? "true" : "false");
    };
    const close = () => setOpen(false);
    toggle.addEventListener("click", () => setOpen(!sidebar.classList.contains("open")));
    if (closeBtn) closeBtn.addEventListener("click", close);
    backdrop.addEventListener("click", close);
    sidebar.addEventListener("click", (event) => {
      if (event.target.closest("a")) close();
    });
    document.addEventListener("keydown", (event) => {
      if (event.key === "Escape" && sidebar.classList.contains("open")) close();
    });
  }

  // Event-delegated copy buttons (works seamlessly across htmx swaps).
  document.addEventListener("click", async (event) => {
    const btn = event.target.closest("[data-copy], [data-copy-target]");
    if (!btn) return;
    let text = btn.getAttribute("data-copy") || "";
    if (!text && btn.getAttribute("data-copy-target") === "next" && btn.nextElementSibling) {
      text = btn.nextElementSibling.textContent || "";
    }
    if (!text) return;
    const orig = btn.textContent;
    try {
      await navigator.clipboard.writeText(text);
    } catch {
      const ta = document.createElement("textarea");
      ta.value = text;
      ta.style.position = "fixed";
      ta.style.opacity = "0";
      document.body.appendChild(ta);
      ta.select();
      document.execCommand("copy");
      ta.remove();
    }
    btn.textContent = "Đã chép";
    btn.classList.add("copied");
    window.setTimeout(() => {
      btn.textContent = orig;
      btn.classList.remove("copied");
    }, 1500);
  });
});
