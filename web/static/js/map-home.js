(function () {
  const cfg = window.FN_MAP || {};
  if (typeof L === 'undefined') return;

  let map;
  let markerLayer;
  let statusFilter = '';
  let searchQ = '';
  let center = [23.8103, 90.4125];

  function init() {
    map = L.map('map', {
      zoomControl: false,
      attributionControl: true
    }).setView(center, 5);

    L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
      maxZoom: 19,
      attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
    }).addTo(map);

    markerLayer = L.layerGroup().addTo(map);

    document.getElementById('zoom-in').onclick = () => map.zoomIn();
    document.getElementById('zoom-out').onclick = () => map.zoomOut();
    document.getElementById('locate-me').onclick = locateMe;

    document.getElementById('search').addEventListener('input', debounce(() => {
      searchQ = document.getElementById('search').value.trim();
      loadPeople();
    }, 300));

    document.getElementById('status-filter').addEventListener('change', (e) => {
      statusFilter = e.target.value;
      loadPeople();
    });

    document.querySelectorAll('.tab-btn').forEach((btn) => {
      btn.addEventListener('click', () => {
        document.querySelectorAll('.tab-btn').forEach((b) => {
          b.classList.remove('bg-indigo-600');
          b.classList.add('bg-white/10');
        });
        btn.classList.add('bg-indigo-600');
        btn.classList.remove('bg-white/10');
        const tab = btn.dataset.tab;
        statusFilter = tab === 'people' ? '' : tab;
        document.getElementById('status-filter').value = statusFilter;
        loadPeople();
      });
    });

    if (navigator.geolocation) {
      navigator.geolocation.getCurrentPosition((pos) => {
        center = [pos.coords.latitude, pos.coords.longitude];
        map.setView(center, 11);
        loadPeople();
      }, () => loadPeople());
    } else {
      loadPeople();
    }

    map.on('moveend', debounce(loadPeople, 400));
  }

  function locateMe() {
    if (!navigator.geolocation) return;
    navigator.geolocation.getCurrentPosition((pos) => {
      center = [pos.coords.latitude, pos.coords.longitude];
      map.setView(center, 12);
      loadPeople();
    });
  }

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
        renderMarkers(users);
        renderList(users);
        document.getElementById('people-count').textContent = data.total || 0;
        document.getElementById('online-count').textContent = data.online || 0;
        document.getElementById('footer-stats').textContent =
          users.length + ' people in this area · ' + (data.total || 0) + ' worldwide';
      })
      .catch(() => {});
  }

  function zoomToRadius(zoom) {
    if (zoom >= 14) return 15;
    if (zoom >= 12) return 40;
    if (zoom >= 10) return 120;
    if (zoom >= 8) return 400;
    if (zoom >= 5) return 1500;
    return 5000;
  }

  function renderMarkers(users) {
    markerLayer.clearLayers();
    users.forEach((u) => {
      const cls = [
        'person-marker',
        u.online ? '' : 'offline',
        u.is_me ? 'me' : '',
        u.live ? 'live' : ''
      ].filter(Boolean).join(' ');
      const icon = L.divIcon({
        className: '',
        html: `<div class="${cls}">${escapeHtml(u.initials || '?')}<span class="dot"></span></div>`,
        iconSize: [40, 40],
        iconAnchor: [20, 20]
      });
      const m = L.marker([u.lat, u.lng], { icon, zIndexOffset: u.is_me ? 1000 : 0 });
      m.on('click', () => openPerson(u));
      const tip = u.full_name + (u.is_me ? ' (You)' : '') + (u.live ? ' · Live' : (u.online ? ' · Online' : ' · Offline'));
      m.bindTooltip(tip);
      markerLayer.addLayer(m);
    });
  }

  function renderList(users) {
    const list = document.getElementById('people-list');
    if (!users.length) {
      list.innerHTML = '<p class="text-sm text-white/50 py-4 text-center">No people in this area yet. Be the first — complete your profile.</p>';
      return;
    }
    list.innerHTML = users.map((u) => `
      <button type="button" data-username="${escapeAttr(u.username)}" class="person-row w-full text-left flex items-center gap-3 p-2 rounded-xl hover:bg-white/10">
        <span class="person-marker ${u.online ? '' : 'offline'} ${u.is_me ? 'me' : ''} ${u.live ? 'live' : ''}" style="width:36px;height:36px;font-size:11px">${escapeHtml(u.initials || '?')}<span class="dot"></span></span>
        <span class="flex-1 min-w-0">
          <span class="block font-medium truncate">${escapeHtml(u.full_name)}${u.is_me ? ' (You)' : ''}</span>
          <span class="block text-xs text-white/50 truncate">${escapeHtml(u.profession || 'Professional')} · ${u.live ? 'Live' : (u.online ? 'Online' : 'Offline')} · ${u.distance_km} km</span>
        </span>
      </button>
    `).join('');

    list.querySelectorAll('.person-row').forEach((row) => {
      row.addEventListener('click', () => {
        openPerson({ username: row.getAttribute('data-username') });
      });
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
