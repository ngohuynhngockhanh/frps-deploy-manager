document.addEventListener("DOMContentLoaded", () => {
  const toggle = document.querySelector("[data-nav-toggle]");
  const sidebar = document.querySelector(".sidebar");
  const backdrop = document.querySelector(".sidebar-backdrop");
  if (!toggle || !sidebar || !backdrop) return;
  const close = () => {
    sidebar.classList.remove("open");
    backdrop.classList.remove("open");
  };
  toggle.addEventListener("click", () => {
    sidebar.classList.toggle("open");
    backdrop.classList.toggle("open");
  });
  backdrop.addEventListener("click", close);
  sidebar.addEventListener("click", (event) => {
    if (event.target.closest("a")) close();
  });
});
