(function () {
  const cfg = window.FN_MAP || {};
  if (typeof L === 'undefined') return;

  let map;
  let markerLayer;
  let statusFilter = '';
  let searchQ = '';
  let center = [23.8103, 90.4125];
  let panelOpen = false;
  let myLocationMarker = null;
  // Saved profile / map marker coords (permanent address) — not browser GPS
  let mySavedLat = typeof cfg.myLat === 'number' ? cfg.myLat : null;
  let mySavedLng = typeof cfg.myLng === 'number' ? cfg.myLng : null;

  function init() {
    if (mySavedLat != null && mySavedLng != null) {
      center = [mySavedLat, mySavedLng];
    }

    map = L.map('map', {
      zoomControl: false,
      attributionControl: true
    }).setView(center, mySavedLat != null ? 14 : 12);

    L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
      maxZoom: 19,
      attribution: '&copy; OpenStreetMap'
    }).addTo(map);

    markerLayer = L.layerGroup().addTo(map);

    document.getElementById('zoom-in').onclick = () => map.zoomIn();
    document.getElementById('zoom-out').onclick = () => map.zoomOut();
    document.getElementById('locate-me').onclick = locateMe;
    document.getElementById('expand-map').onclick = toggleExpand;
    document.getElementById('toggle-people').onclick = togglePanel;
    const panelClose = document.getElementById('people-panel-close');
    if (panelClose) panelClose.onclick = closePanel;

    setupSideNav();

    const search = document.getElementById('search');
    if (search) {
      search.addEventListener('input', debounce(() => {
        searchQ = search.value.trim();
        loadPeople();
      }, 300));
    }

    const statusEl = document.getElementById('status-filter');
    if (statusEl) {
      statusEl.addEventListener('change', (e) => {
        statusFilter = e.target.value;
        loadPeople();
      });
    }

    // Use saved profile location only — do not ask the browser for GPS on load
    loadPeople();

    map.on('moveend', debounce(loadPeople, 400));
    // Clicking the map closes the side nav
    map.on('click', closeNav);
  }

  function setupSideNav() {
    const openBtn = document.getElementById('menu-toggle');
    const closeBtn = document.getElementById('nav-close');
    const backdrop = document.getElementById('nav-backdrop');
    const nav = document.getElementById('side-nav');

    if (openBtn) openBtn.addEventListener('click', (e) => {
      e.stopPropagation();
      openNav();
    });
    if (closeBtn) closeBtn.addEventListener('click', closeNav);
    if (backdrop) backdrop.addEventListener('click', closeNav);

    // Close when choosing a nav link (except live toggle)
    if (nav) {
      nav.querySelectorAll('a.disc-nav-link').forEach((a) => {
        a.addEventListener('click', () => closeNav());
      });
      nav.addEventListener('click', (e) => e.stopPropagation());
    }

    document.addEventListener('keydown', (e) => {
      if (e.key === 'Escape') {
        closeNav();
        closePanel();
      }
    });
  }

  function openNav() {
    document.body.classList.add('nav-open');
    const btn = document.getElementById('menu-toggle');
    if (btn) btn.setAttribute('aria-expanded', 'true');
  }

  function closeNav() {
    document.body.classList.remove('nav-open');
    const btn = document.getElementById('menu-toggle');
    if (btn) btn.setAttribute('aria-expanded', 'false');
  }

  function toggleExpand() {
    document.body.classList.toggle('is-map-expanded');
    const btn = document.getElementById('expand-map');
    btn.classList.toggle('is-active');
    closeNav();
    setTimeout(() => map.invalidateSize(), 200);
  }

  function setPanelOpen(open) {
    panelOpen = !!open;
    const panel = document.getElementById('people-panel');
    const btn = document.getElementById('toggle-people');
    if (panel) panel.classList.toggle('is-open', panelOpen);
    if (btn) btn.classList.toggle('is-active', panelOpen);
  }

  function togglePanel() {
    setPanelOpen(!panelOpen);
    closeNav();
  }

  function closePanel() {
    setPanelOpen(false);
  }

  function flyToMyPlace(lat, lng, btn) {
    center = [lat, lng];
    map.flyTo([lat, lng], 16, { animate: true, duration: 1.15 });
    updateMyDot(lat, lng);

    const afterZoom = () => {
      loadPeople();
      if (btn) btn.classList.remove('is-locating');
    };
    map.once('moveend', afterZoom);
    setTimeout(afterZoom, 1300);
  }

  // ◎ zooms to your saved profile pin — no browser GPS permission required.
  // Optional live GPS is attempted silently only if the browser already allows it.
  function locateMe() {
    const btn = document.getElementById('locate-me');
    if (btn) btn.classList.add('is-locating');

    const savedLat = mySavedLat;
    const savedLng = mySavedLng;

    if (savedLat != null && savedLng != null) {
      // Zoom to profile pin — never asks the browser for GPS
      flyToMyPlace(savedLat, savedLng, btn);
      return;
    }

    // Guest / no profile pin yet: try GPS once, no hard alert if denied
    if (!navigator.geolocation) {
      if (btn) btn.classList.remove('is-locating');
      return;
    }
    navigator.geolocation.getCurrentPosition(
      (pos) => flyToMyPlace(pos.coords.latitude, pos.coords.longitude, btn),
      () => {
        if (btn) btn.classList.remove('is-locating');
      },
      { enableHighAccuracy: true, timeout: 8000, maximumAge: 60000 }
    );
  }

  function updateMyDot(lat, lng) {
    if (myLocationMarker) {
      myLocationMarker.setLatLng([lat, lng]);
      return;
    }
    myLocationMarker = L.circleMarker([lat, lng], {
      radius: 8,
      color: '#fff',
      weight: 2,
      fillColor: '#3b82f6',
      fillOpacity: 1
    }).addTo(map);
    myLocationMarker.bindTooltip('You are here', { permanent: false });
  }

  // Called while live GPS updates so the blue dot follows you
  window.FN_onLiveMove = function (lat, lng) {
    center = [lat, lng];
    updateMyDot(lat, lng);
  };

  function loadPeople() {
    const c = map.getCenter();
    const zoom = map.getZoom();
    const radiusKm = zoomToRadius(zoom);
    const params = new URLSearchParams({
      lat: String(c.lat),
      lng: String(c.lng),
      radius_km: String(radiusKm)
    });
    if (statusFilter) params.set('status', statusFilter);
    if (searchQ) params.set('q', searchQ);

    fetch('/api/v1/users/nearby?' + params.toString())
      .then((r) => r.json())
      .then((body) => {
        const data = body.data || body;
        const users = data.users || [];
        const me = users.find((u) => u.is_me);
        if (me && typeof me.lat === 'number' && typeof me.lng === 'number') {
          mySavedLat = me.lat;
          mySavedLng = me.lng;
        }
        renderMarkers(users);
        renderList(users);
        const pc = document.getElementById('people-count');
        const oc = document.getElementById('online-count');
        const fs = document.getElementById('footer-stats');
        if (pc) pc.textContent = data.total || 0;
        if (oc) oc.textContent = data.online || 0;
        if (fs) {
          fs.textContent = users.length + ' nearby · ' + (data.total || 0) + ' worldwide';
        }
      })
      .catch(() => {});
  }

  function zoomToRadius(zoom) {
    if (zoom >= 16) return 5;
    if (zoom >= 14) return 15;
    if (zoom >= 12) return 40;
    if (zoom >= 10) return 120;
    if (zoom >= 8) return 400;
    if (zoom >= 5) return 1500;
    return 5000;
  }

  function markerHTML(u) {
    const cls = [
      'disc-marker',
      u.online ? '' : 'offline',
      u.is_me ? 'me' : '',
      u.live ? 'live' : ''
    ].filter(Boolean).join(' ');

    if (u.photo_url) {
      return `<div class="${cls}"><img src="${escapeAttr(u.photo_url)}" alt=""><span class="dot"></span></div>`;
    }
    return `<div class="${cls}">${escapeHtml(u.initials || '?')}<span class="dot"></span></div>`;
  }

  function renderMarkers(users) {
    markerLayer.clearLayers();
    users.forEach((u) => {
      const icon = L.divIcon({
        className: '',
        html: markerHTML(u),
        iconSize: [46, 46],
        iconAnchor: [23, 23]
      });
      const m = L.marker([u.lat, u.lng], {
        icon,
        zIndexOffset: u.is_me ? 1000 : Math.round((u.lat % 1) * 100)
      });
      m.on('click', () => openPerson(u));
      const tip = u.full_name + (u.is_me ? ' (You)' : '') + (u.live ? ' · Live' : (u.online ? ' · Online' : ' · Offline'));
      m.bindTooltip(tip, { direction: 'top', offset: [0, -18] });
      markerLayer.addLayer(m);
    });
  }

  function renderList(users) {
    const list = document.getElementById('people-list');
    if (!list) return;
    if (!users.length) {
      list.innerHTML = '<p style="opacity:.55;font-size:.85rem;padding:1rem;text-align:center">No people in this area yet.</p>';
      return;
    }
    list.innerHTML = users.map((u) => {
      const avatar = u.photo_url
        ? `<div class="disc-marker" style="width:36px;height:36px;flex-shrink:0"><img src="${escapeAttr(u.photo_url)}" alt=""></div>`
        : `<div class="disc-marker" style="width:36px;height:36px;font-size:11px;flex-shrink:0">${escapeHtml(u.initials || '?')}</div>`;
      return `
      <button type="button" data-username="${escapeAttr(u.username)}"
        style="width:100%;text-align:left;display:flex;gap:.75rem;align-items:center;padding:.55rem;border-radius:.85rem;background:transparent;border:0;color:#fff;cursor:pointer">
        ${avatar}
        <span style="min-width:0;flex:1">
          <span style="display:block;font-weight:600;white-space:nowrap;overflow:hidden;text-overflow:ellipsis">${escapeHtml(u.full_name)}${u.is_me ? ' (You)' : ''}</span>
          <span style="display:block;font-size:.72rem;opacity:.55;white-space:nowrap;overflow:hidden;text-overflow:ellipsis">${escapeHtml(u.profession || 'Professional')} · ${u.distance_km} km</span>
        </span>
      </button>`;
    }).join('');

    list.querySelectorAll('[data-username]').forEach((row) => {
      row.addEventListener('click', () => openPerson({ username: row.getAttribute('data-username') }));
    });
  }

  function openPerson(u) {
    const path = '/users/' + encodeURIComponent(u.username);
    if (cfg.loggedIn) {
      window.location.href = path;
    } else {
      window.location.href = '/auth/login?next=' + encodeURIComponent(path);
    }
  }

  function escapeHtml(s) {
    return String(s || '').replace(/[&<>"']/g, (c) => ({
      '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
    }[c]));
  }

  function escapeAttr(s) {
    return escapeHtml(s).replace(/"/g, '&quot;');
  }

  function debounce(fn, ms) {
    let t;
    return (...args) => {
      clearTimeout(t);
      t = setTimeout(() => fn(...args), ms);
    };
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }

  window.FN_reloadMap = loadPeople;
})();
