// FindNeighbour client utilities
document.body.addEventListener('htmx:configRequest', (e) => {
  const csrf = document.querySelector('input[name="csrf_token"]');
  if (csrf) e.detail.headers['X-CSRF-Token'] = csrf.value;
});
