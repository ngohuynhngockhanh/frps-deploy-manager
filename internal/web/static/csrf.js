// Attach the CSRF token to every htmx request so mutating calls pass the
// server-side CSRF check.
document.addEventListener('htmx:configRequest', function (e) {
  var meta = document.querySelector('meta[name="csrf-token"]');
  if (meta && meta.content) {
    e.detail.headers['X-CSRF-Token'] = meta.content;
  }
});
