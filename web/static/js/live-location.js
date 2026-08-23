(function () {
  const cfg = window.FN_MAP || {};
  if (!cfg.loggedIn) {
    window.FN_live = {
      start: function () {},
      stop: function () {},
      isActive: function () { return false; }
    };
    return;
  }

  let watchId = null;
  let active = false;

  function csrf() {
    return cfg.csrf || '';
  }

  function setUI(on) {
    active = !!on;
    const btn = document.getElementById('live-toggle');
    const label = document.getElementById('live-toggle-label');
    const locateBtn = document.getElementById('locate-me');
    if (btn) btn.dataset.on = on ? '1' : '0';
    if (label) {
      label.textContent = on ? 'Stop live tracking' : 'Live sharing (optional)';
    }
    if (locateBtn) locateBtn.classList.toggle('is-active', on);
  }

  function postForm(url, fields) {
    const fd = new FormData();
    Object.keys(fields || {}).forEach((k) => fd.append(k, fields[k]));
    fd.append('csrf_token', csrf());
    return fetch(url, { method: 'POST', body: fd }).then((r) => r.json());
  }

  function beginWatch() {
    if (!navigator.geolocation || watchId != null) return;
    watchId = navigator.geolocation.watchPosition(
      (pos) => {
        const lat = pos.coords.latitude;
        const lng = pos.coords.longitude;
        postForm('/api/live-location', {
          lat: String(lat),
          lng: String(lng)
        }).then(() => {
          if (typeof window.FN_onLiveMove === 'function') {
            window.FN_onLiveMove(lat, lng);
          }
          if (typeof window.FN_reloadMap === 'function') {
            window.FN_reloadMap();
          }
        });
      },
      () => {},
      { enableHighAccuracy: true, maximumAge: 3000, timeout: 15000 }
    );
  }

  function start(lat, lng) {
    const go = (la, ln) =>
      postForm('/api/live-location/start', {
        lat: String(la),
        lng: String(ln)
      }).then((res) => {
        if (res && res.ok !== false) {
          setUI(true);
          // Watch GPS only when the browser allows it — never alert on deny
          beginWatch();
          if (typeof window.FN_reloadMap === 'function') window.FN_reloadMap();
          return true;
        }
        return false;
      }).catch(() => false);

    if (typeof lat === 'number' && typeof lng === 'number') {
      return go(lat, lng);
    }

    if (!navigator.geolocation) {
      return Promise.resolve(false);
    }

    return new Promise((resolve) => {
      navigator.geolocation.getCurrentPosition(
        (pos) => go(pos.coords.latitude, pos.coords.longitude).then(resolve),
        () => resolve(false),
        { enableHighAccuracy: true, timeout: 8000, maximumAge: 60000 }
      );
    });
  }

  function stop() {
    if (watchId != null) {
      navigator.geolocation.clearWatch(watchId);
      watchId = null;
    }
    return fetch('/api/live-location', {
      method: 'DELETE',
      headers: { 'X-CSRF-Token': csrf() }
    }).then(() => {
      setUI(false);
      if (typeof window.FN_reloadMap === 'function') window.FN_reloadMap();
    }).catch(() => setUI(false));
  }

  // Sidebar: only used to STOP tracking (start is via ◎ My location)
  const btn = document.getElementById('live-toggle');
  if (btn) {
    btn.addEventListener('click', () => {
      if (active || btn.dataset.on === '1') stop();
    });
  }

  fetch('/api/live-location/status', { headers: { Accept: 'application/json' } })
    .then((r) => r.json())
    .then((data) => {
      if (data.live) {
        setUI(true);
        beginWatch();
      } else {
        setUI(false);
      }
    })
    .catch(() => setUI(false));

  window.FN_live = {
    start: start,
    stop: stop,
    isActive: function () { return active; }
  };
})();
