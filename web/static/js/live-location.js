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
  let reloadTimer = null;
  let stopping = false;

  function csrf() {
    return cfg.csrf || '';
  }

  function scheduleReload() {
    clearTimeout(reloadTimer);
    reloadTimer = setTimeout(function () {
      if (typeof window.FN_reloadMap === 'function') window.FN_reloadMap();
    }, 2500);
  }

  function setUI(on) {
    active = !!on;
    cfg.liveOn = active;
    const btn = document.getElementById('live-toggle');
    if (btn) {
      btn.dataset.on = on ? '1' : '0';
      btn.title = on ? 'Stop live sharing (tap again)' : 'Share live location';
      btn.classList.toggle('is-active', on);
      btn.classList.toggle('is-live-sharing', on);
      btn.setAttribute('aria-pressed', on ? 'true' : 'false');
    }
    if (typeof window.FN_applyLivePulse === 'function') {
      window.FN_applyLivePulse(on);
    }
  }

  function postForm(url, fields) {
    const fd = new FormData();
    Object.keys(fields || {}).forEach((k) => fd.append(k, fields[k]));
    fd.append('csrf_token', csrf());
    return fetch(url, { method: 'POST', credentials: 'same-origin', body: fd })
      .then((r) => r.json().then((data) => ({ ok: r.ok, data })));
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
        }).then((res) => {
          if (!res.ok) return;
          if (typeof window.FN_onLiveMove === 'function') {
            window.FN_onLiveMove(lat, lng);
          }
          scheduleReload();
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
        if (res.ok && res.data && res.data.ok !== false) {
          setUI(true);
          beginWatch();
          if (typeof window.FN_onLiveMove === 'function') {
            window.FN_onLiveMove(la, ln);
          }
          scheduleReload();
          return true;
        }
        setUI(false);
        return false;
      }).catch(() => {
        setUI(false);
        return false;
      });

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
    if (stopping) return Promise.resolve();
    stopping = true;
    if (watchId != null) {
      navigator.geolocation.clearWatch(watchId);
      watchId = null;
    }
    clearTimeout(reloadTimer);
    return fetch('/api/live-location', {
      method: 'DELETE',
      credentials: 'same-origin',
      headers: { 'X-CSRF-Token': csrf() }
    }).then((r) => r.json().then((data) => ({ ok: r.ok, data })))
      .then((res) => {
        setUI(false);
        if (typeof window.FN_reloadMap === 'function') window.FN_reloadMap();
        if (typeof cfg.myLat === 'number' && typeof cfg.myLng === 'number' &&
            typeof window.FN_onLiveMove === 'function') {
          window.FN_onLiveMove(cfg.myLat, cfg.myLng);
        }
        return res.ok;
      })
      .catch(() => {
        setUI(false);
        return false;
      })
      .finally(() => {
        stopping = false;
      });
  }

  const btn = document.getElementById('live-toggle');
  if (btn) {
    btn.addEventListener('click', () => {
      if (active || btn.dataset.on === '1') {
        stop();
      } else {
        start();
      }
    });
  }

  fetch('/api/live-location/status', { credentials: 'same-origin', headers: { Accept: 'application/json' } })
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
