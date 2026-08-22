(function () {
  const cfg = window.FN_MAP || {};
  if (!cfg.loggedIn) return;

  let watchId = null;
  const btn = document.getElementById('live-toggle');
  if (!btn) return;

  function csrf() {
    return cfg.csrf || '';
  }

  function setUI(on) {
    btn.dataset.on = on ? '1' : '0';
    btn.textContent = on ? 'Live ON' : 'Live location';
    btn.classList.toggle('bg-emerald-500', on);
    btn.classList.toggle('bg-slate-900/80', !on);
    btn.classList.toggle('text-white', true);
  }

  function postForm(url, fields, method) {
    const fd = new FormData();
    Object.keys(fields || {}).forEach((k) => fd.append(k, fields[k]));
    fd.append('csrf_token', csrf());
    return fetch(url, {
      method: method || 'POST',
      body: method === 'DELETE' ? null : fd,
      headers: method === 'DELETE' ? { 'X-CSRF-Token': csrf() } : undefined
    }).then((r) => r.json());
  }

  function beginTracking() {
    if (!navigator.geolocation) return;
    if (watchId != null) return;
    watchId = navigator.geolocation.watchPosition((pos) => {
      postForm('/api/live-location', {
        lat: String(pos.coords.latitude),
        lng: String(pos.coords.longitude)
      }).then(() => {
        if (typeof window.FN_reloadMap === 'function') window.FN_reloadMap();
      });
    }, () => {}, { enableHighAccuracy: true, maximumAge: 5000, timeout: 15000 });
  }

  function startWatch() {
    if (!navigator.geolocation) {
      alert('Geolocation is not available in this browser.');
      return;
    }
    navigator.geolocation.getCurrentPosition((pos) => {
      postForm('/api/live-location/start', {
        lat: String(pos.coords.latitude),
        lng: String(pos.coords.longitude)
      }).then(() => {
        setUI(true);
        beginTracking();
        if (typeof window.FN_reloadMap === 'function') window.FN_reloadMap();
      });
    });
  }

  function stopWatch() {
    if (watchId != null) {
      navigator.geolocation.clearWatch(watchId);
      watchId = null;
    }
    fetch('/api/live-location', {
      method: 'DELETE',
      headers: { 'X-CSRF-Token': csrf() }
    }).then(() => {
      setUI(false);
      if (typeof window.FN_reloadMap === 'function') window.FN_reloadMap();
    });
  }

  btn.addEventListener('click', () => {
    if (btn.dataset.on === '1') stopWatch();
    else startWatch();
  });

  fetch('/api/live-location/status', { headers: { 'Accept': 'application/json' } })
    .then((r) => r.json())
    .then((data) => {
      if (data.live) {
        setUI(true);
        beginTracking();
      } else {
        setUI(false);
      }
    })
    .catch(() => setUI(false));
})();
