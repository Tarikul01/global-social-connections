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
  let myMapMarker = null;
  let lastUsers = [];
  let mySavedLat = typeof cfg.myLat === 'number' ? cfg.myLat : null;
  let mySavedLng = typeof cfg.myLng === 'number' ? cfg.myLng : null;
  let pollTimer = null;
  let loadInFlight = false;
  let loadQueued = false;
  let allProfessions = [];
  let selectedProfessions = new Set();
  let professionSearchQ = '';
  let professionPanelOpen = false;
  let eventLayer = null;
  let activeEventId = null;
  let hostedEvents = [];
  // Near-realtime updates via HTTP polling (no Redis / WebSocket).
  const POLL_MS = Math.max(3000, Number(cfg.pollMs) || 8000);

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
    eventLayer = L.layerGroup().addTo(map);

    document.getElementById('zoom-in').onclick = () => map.zoomIn();
    document.getElementById('zoom-out').onclick = () => map.zoomOut();
    document.getElementById('locate-me').onclick = locateMe;
    document.getElementById('expand-map').onclick = toggleExpand;
    document.getElementById('toggle-people').onclick = togglePanel;
    const panelClose = document.getElementById('people-panel-close');
    if (panelClose) panelClose.onclick = closePanel;
    const eventClose = document.getElementById('hosted-events-close');
    if (eventClose) eventClose.onclick = clearEventMode;

    setupSideNav();
    setupProfessionFilter();
    if (cfg.loggedIn) loadHostedEvents();

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
    startPolling();

    map.on('moveend', debounce(loadPeople, 400));
    // Clicking the map closes the side nav
    map.on('click', () => {
      closeNav();
      closeProfessionPanel();
    });

    document.addEventListener('visibilitychange', onVisibilityChange);
    document.addEventListener('click', onDocumentClick);
  }

  function setupProfessionFilter() {
    const toggle = document.getElementById('profession-filter-toggle');
    const panel = document.getElementById('profession-filter-panel');
    const search = document.getElementById('profession-filter-search');
    const clearBtn = document.getElementById('profession-filter-clear');
    if (!toggle || !panel) return;

    toggle.addEventListener('click', (e) => {
      e.stopPropagation();
      if (professionPanelOpen) {
        closeProfessionPanel();
      } else {
        openProfessionPanel();
      }
    });

    if (search) {
      search.addEventListener('input', debounce(() => {
        professionSearchQ = search.value.trim().toLowerCase();
        renderProfessionOptions();
      }, 150));
      search.addEventListener('click', (e) => e.stopPropagation());
    }

    if (clearBtn) {
      clearBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        clearProfessionFilters();
      });
    }

    panel.addEventListener('click', (e) => e.stopPropagation());
    loadProfessions();
  }

  function loadProfessions() {
    fetch('/api/v1/professions')
      .then((r) => r.json())
      .then((body) => {
        const data = body.data || body.professions || [];
        allProfessions = Array.isArray(data) ? data : [];
        renderProfessionOptions();
      })
      .catch(() => {});
  }

  function openProfessionPanel() {
    professionPanelOpen = true;
    const toggle = document.getElementById('profession-filter-toggle');
    const panel = document.getElementById('profession-filter-panel');
    if (toggle) {
      toggle.classList.add('is-open');
      toggle.setAttribute('aria-expanded', 'true');
    }
    if (panel) panel.classList.add('is-open');
    const search = document.getElementById('profession-filter-search');
    if (search) search.focus();
  }

  function closeProfessionPanel() {
    professionPanelOpen = false;
    const toggle = document.getElementById('profession-filter-toggle');
    const panel = document.getElementById('profession-filter-panel');
    if (toggle) {
      toggle.classList.remove('is-open');
      toggle.setAttribute('aria-expanded', 'false');
    }
    if (panel) panel.classList.remove('is-open');
  }

  function onDocumentClick() {
    if (professionPanelOpen) closeProfessionPanel();
  }

  function toggleProfession(slug) {
    if (selectedProfessions.has(slug)) {
      selectedProfessions.delete(slug);
    } else {
      selectedProfessions.add(slug);
    }
    updateProfessionUI();
    loadPeople();
  }

  function clearProfessionFilters() {
    selectedProfessions.clear();
    professionSearchQ = '';
    const search = document.getElementById('profession-filter-search');
    if (search) search.value = '';
    updateProfessionUI();
    renderProfessionOptions();
    loadPeople();
  }

  function updateProfessionUI() {
    const toggle = document.getElementById('profession-filter-toggle');
    const clearBtn = document.getElementById('profession-filter-clear');
    const chipsEl = document.getElementById('profession-filter-chips');
    const hasActive = selectedProfessions.size > 0;

    if (toggle) toggle.classList.toggle('has-active', hasActive);
    if (clearBtn) clearBtn.hidden = !hasActive;
    if (!chipsEl) return;

    if (!hasActive) {
      chipsEl.hidden = true;
      chipsEl.innerHTML = '';
      return;
    }

    chipsEl.hidden = false;
    chipsEl.innerHTML = Array.from(selectedProfessions).map((slug) => {
      const p = allProfessions.find((x) => x.slug === slug);
      const label = p ? p.name : slug;
      return `<span class="disc-filter-chip">${escapeHtml(label)}<button type="button" data-slug="${escapeAttr(slug)}" aria-label="Remove ${escapeAttr(label)}">×</button></span>`;
    }).join('');

    chipsEl.querySelectorAll('button[data-slug]').forEach((btn) => {
      btn.addEventListener('click', (e) => {
        e.stopPropagation();
        selectedProfessions.delete(btn.getAttribute('data-slug'));
        updateProfessionUI();
        renderProfessionOptions();
        loadPeople();
      });
    });
  }

  function renderProfessionOptions() {
    const list = document.getElementById('profession-filter-list');
    if (!list) return;

    const filtered = allProfessions.filter((p) => {
      if (!professionSearchQ) return true;
      return (p.name || '').toLowerCase().includes(professionSearchQ);
    });

    if (!filtered.length) {
      list.innerHTML = '<p style="padding:.75rem;font-size:.78rem;opacity:.55;text-align:center">No designations found</p>';
      return;
    }

    list.innerHTML = filtered.map((p) => {
      const checked = selectedProfessions.has(p.slug);
      return `<label class="disc-filter-option${checked ? ' is-selected' : ''}">
        <input type="checkbox" data-slug="${escapeAttr(p.slug)}"${checked ? ' checked' : ''}>
        <span>${escapeHtml(p.name)}</span>
        <span class="disc-filter-option__count">${p.count || 0}</span>
      </label>`;
    }).join('');

    list.querySelectorAll('input[data-slug]').forEach((input) => {
      input.addEventListener('change', (e) => {
        e.stopPropagation();
        toggleProfession(input.getAttribute('data-slug'));
        renderProfessionOptions();
      });
    });
  }

  function onVisibilityChange() {
    if (document.hidden) {
      stopPolling();
    } else {
      if (activeEventId) refreshActiveEvent();
      else loadPeople();
      startPolling();
    }
  }

  function startPolling() {
    stopPolling();
    if (document.hidden) return;
    pollTimer = setInterval(function () {
      if (document.hidden) return;
      if (activeEventId) {
        refreshActiveEvent();
      } else {
        loadPeople();
      }
    }, POLL_MS);
  }

  function stopPolling() {
    if (pollTimer) {
      clearInterval(pollTimer);
      pollTimer = null;
    }
  }

  function setupSideNav() {
    document.addEventListener('keydown', (e) => {
      if (e.key === 'Escape') {
        closeNav();
        closePanel();
        closeProfessionPanel();
      }
    });
  }

  function closeNav() {
    if (typeof window.FN_closeSideNav === 'function') {
      window.FN_closeSideNav();
    }
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

  // Called while live GPS updates so the marker follows you
  window.FN_onLiveMove = function (lat, lng) {
    center = [lat, lng];
    updateMyDot(lat, lng);
    if (myMapMarker) {
      myMapMarker.setLatLng([lat, lng]);
    }
  };

  window.FN_applyLivePulse = function (on) {
    cfg.liveOn = !!on;
    if (lastUsers.length) {
      renderMarkers(lastUsers);
    } else {
      loadPeople();
    }
  };

  window.FN_refreshLiveMarker = window.FN_applyLivePulse;

  function loadPeople() {
    if (!map || activeEventId) return;
    if (loadInFlight) {
      loadQueued = true;
      return;
    }
    loadInFlight = true;

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
    selectedProfessions.forEach((slug) => params.append('profession', slug));

    fetch('/api/v1/users/nearby?' + params.toString())
      .then((r) => r.json())
      .then((body) => {
        const data = body.data || body;
        const users = data.users || [];
        lastUsers = users;
        if (data.live === true || data.live === false) {
          cfg.liveOn = !!data.live;
        }
        const me = users.find((u) => u.is_me);
        if (me && typeof me.lat === 'number' && typeof me.lng === 'number') {
          mySavedLat = me.lat;
          mySavedLng = me.lng;
        }
        renderMarkers(users);
        renderList(users);
        updateMapStats(data.total || 0, data.online || 0, users.length);
      })
      .catch(() => {})
      .finally(() => {
        loadInFlight = false;
        if (loadQueued) {
          loadQueued = false;
          loadPeople();
        }
      });
  }

  function updateMapStats(total, online, nearby) {
    const mapTotal = document.getElementById('map-total-count');
    const mapOnline = document.getElementById('map-online-count');
    const pc = document.getElementById('people-count');
    const oc = document.getElementById('online-count');
    const fs = document.getElementById('footer-stats');
    if (mapTotal) mapTotal.textContent = total;
    if (mapOnline) mapOnline.textContent = online;
    if (pc) pc.textContent = total;
    if (oc) oc.textContent = online;
    if (fs) fs.textContent = nearby + ' nearby · ' + total + ' worldwide';
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

  function isMeLiveSharing(u) {
    if (!u.is_me) return false;
    if (u.live) return true;
    if (cfg.liveOn) return true;
    if (window.FN_live && window.FN_live.isActive()) return true;
    return false;
  }

  function markerHTML(u) {
    const liveSharing = isMeLiveSharing(u);
    const cls = [
      'disc-marker',
      u.online ? 'online' : 'offline',
      u.is_me ? 'me' : '',
      (u.live || liveSharing) ? 'live' : ''
    ].filter(Boolean).join(' ');

    let inner;
    if (u.photo_url) {
      inner = `<div class="${cls}"><img src="${escapeAttr(u.photo_url)}" alt=""><span class="dot" aria-hidden="true"></span></div>`;
    } else {
      inner = `<div class="${cls}">${escapeHtml(u.initials || '?')}<span class="dot" aria-hidden="true"></span></div>`;
    }

    if (liveSharing) {
      return `<div class="disc-marker-wrap is-live-sharing">
        <span class="live-ring" aria-hidden="true"></span>
        <span class="live-ring live-ring--b" aria-hidden="true"></span>
        ${inner}
      </div>`;
    }
    return inner;
  }

  function markerIconOpts(u) {
    if (isMeLiveSharing(u)) {
      return { iconSize: [72, 72], iconAnchor: [36, 36] };
    }
    return { iconSize: [46, 46], iconAnchor: [23, 23] };
  }

  function renderMarkers(users) {
    markerLayer.clearLayers();
    myMapMarker = null;
    users.forEach((u) => {
      const dims = markerIconOpts(u);
      const icon = L.divIcon({
        className: '',
        html: markerHTML(u),
        iconSize: dims.iconSize,
        iconAnchor: dims.iconAnchor
      });
      const m = L.marker([u.lat, u.lng], {
        icon,
        zIndexOffset: u.is_me ? 1000 : Math.round((u.lat % 1) * 100)
      });
      m.on('click', () => openPerson(u));
      const sharing = isMeLiveSharing(u);
      const tip = u.full_name + (u.is_me ? ' (You)' : '') +
        (sharing ? ' · Live sharing' : (u.live ? ' · Live' : (u.online ? ' · Online' : ' · Offline')));
      m.bindTooltip(tip, { direction: 'top', offset: [0, -18] });
      markerLayer.addLayer(m);
      if (u.is_me) myMapMarker = m;
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
      const statusCls = u.online ? 'online' : 'offline';
      const avatar = u.photo_url
        ? `<div class="disc-marker ${statusCls}" style="width:36px;height:36px;flex-shrink:0"><img src="${escapeAttr(u.photo_url)}" alt=""><span class="dot" aria-hidden="true"></span></div>`
        : `<div class="disc-marker ${statusCls}" style="width:36px;height:36px;font-size:11px;flex-shrink:0">${escapeHtml(u.initials || '?')}<span class="dot" aria-hidden="true"></span></div>`;
      return `
      <button type="button" data-username="${escapeAttr(u.username)}"
        style="width:100%;text-align:left;display:flex;gap:.75rem;align-items:center;padding:.55rem;border-radius:.85rem;background:transparent;border:0;color:#fff;cursor:pointer">
        ${avatar}
        <span style="min-width:0;flex:1">
          <span style="display:block;font-weight:600;white-space:nowrap;overflow:hidden;text-overflow:ellipsis">${escapeHtml(u.full_name)}${u.is_me ? ' (You)' : ''}</span>
          <span style="display:block;font-size:.72rem;opacity:.55;white-space:nowrap;overflow:hidden;text-overflow:ellipsis">${escapeHtml(u.profession || 'Professional')} · ${u.distance_km} km · ${u.online ? 'Online' : 'Offline'}</span>
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

  function loadHostedEvents() {
    fetch('/api/events/hosted', { credentials: 'same-origin', headers: { Accept: 'application/json' } })
      .then((r) => r.json())
      .then((body) => {
        hostedEvents = body.data || [];
        renderHostedEventsList();
      })
      .catch(() => {});
  }

  function renderHostedEventsList() {
    const panel = document.getElementById('hosted-events-panel');
    const list = document.getElementById('hosted-events-list');
    if (!panel || !list) return;
    if (!hostedEvents.length) {
      panel.classList.remove('is-visible');
      return;
    }
    panel.classList.add('is-visible');
    list.innerHTML = hostedEvents.map((e) => `
      <button type="button" class="disc-events__item${activeEventId === e.id ? ' is-active' : ''}" data-event-id="${escapeAttr(e.id)}">
        <span class="disc-events__name">${escapeHtml(e.short_name || e.title)}</span>
        <span class="disc-events__meta">${escapeHtml(e.event_date || '')}${e.location_name ? ' · ' + escapeHtml(e.location_name) : ''} · ${e.member_count || 0} going</span>
      </button>
    `).join('');
    list.querySelectorAll('[data-event-id]').forEach((btn) => {
      btn.addEventListener('click', () => {
        const id = btn.getAttribute('data-event-id');
        if (activeEventId === id) clearEventMode();
        else enterEventMode(id);
      });
    });
  }

  function enterEventMode(eventId) {
    activeEventId = eventId;
    document.body.classList.add('fn-event-mode');
    const panel = document.getElementById('hosted-events-panel');
    if (panel) panel.classList.add('is-active');
    closePanel();
    if (markerLayer) markerLayer.clearLayers();
    myMapMarker = null;
    renderHostedEventsList();
    const hint = document.getElementById('hosted-events-hint');
    if (hint) hint.textContent = 'Showing accepted members → event directions. Tap ✕ to show people again.';
    refreshActiveEvent();
  }

  function clearEventMode() {
    activeEventId = null;
    document.body.classList.remove('fn-event-mode');
    const panel = document.getElementById('hosted-events-panel');
    if (panel) panel.classList.remove('is-active');
    if (eventLayer) eventLayer.clearLayers();
    const hint = document.getElementById('hosted-events-hint');
    if (hint) hint.textContent = 'Tap an event to show accepted members & directions';
    renderHostedEventsList();
    loadPeople();
  }

  function refreshActiveEvent() {
    if (!activeEventId) return;
    fetch('/api/events/' + encodeURIComponent(activeEventId) + '/map-members', {
      credentials: 'same-origin',
      headers: { Accept: 'application/json' }
    })
      .then((r) => r.json())
      .then((body) => {
        if (!body.success || !body.data) return;
        if (activeEventId !== body.data.event.id) return;
        renderEventMap(body.data);
      })
      .catch(() => {});
  }

  function googleDirectionsURL(fromLat, fromLng, toLat, toLng) {
    return 'https://www.google.com/maps/dir/?api=1'
      + '&origin=' + encodeURIComponent(fromLat + ',' + fromLng)
      + '&destination=' + encodeURIComponent(toLat + ',' + toLng)
      + '&travelmode=driving';
  }

  function renderEventMap(data) {
    if (!eventLayer) return;
    eventLayer.clearLayers();
    const ev = data.event;
    const members = data.members || [];
    const bounds = [];

    const eventIcon = L.divIcon({
      className: '',
      html: '<div class="disc-event-pin" title="Event">📍</div>',
      iconSize: [42, 42],
      iconAnchor: [21, 21]
    });
    const eventMarker = L.marker([ev.lat, ev.lng], { icon: eventIcon, zIndexOffset: 2000 });
    eventMarker.bindTooltip(ev.title + (ev.location_name ? ' · ' + ev.location_name : ''), {
      direction: 'top', offset: [0, -18]
    });
    eventLayer.addLayer(eventMarker);
    bounds.push([ev.lat, ev.lng]);

    members.forEach((m) => {
      const cls = 'disc-member-marker' + (m.live ? ' live' : '');
      const icon = L.divIcon({
        className: '',
        html: '<div class="' + cls + '">' + escapeHtml(m.initials || '?') + '</div>',
        iconSize: [40, 40],
        iconAnchor: [20, 20]
      });
      const marker = L.marker([m.lat, m.lng], { icon, zIndexOffset: 1100 });
      const gmaps = googleDirectionsURL(m.lat, m.lng, ev.lat, ev.lng);
      marker.bindTooltip(m.full_name + (m.live ? ' · Live' : '') + ' → ' + (ev.short_name || ev.title), {
        direction: 'top', offset: [0, -16]
      });
      marker.bindPopup(
        '<strong>' + escapeHtml(m.full_name) + '</strong><br>'
        + '<span style="opacity:.7;font-size:12px">@' + escapeHtml(m.username) + '</span><br>'
        + '<a href="' + gmaps + '" target="_blank" rel="noopener" style="color:#60a5fa">Open directions in Google Maps</a>'
      );
      eventLayer.addLayer(marker);

      const line = L.polyline([[m.lat, m.lng], [ev.lat, ev.lng]], {
        color: '#facc15',
        weight: 3,
        opacity: 0.85,
        dashArray: '8 6'
      });
      line.on('click', () => window.open(gmaps, '_blank', 'noopener'));
      eventLayer.addLayer(line);
      bounds.push([m.lat, m.lng]);
    });

    if (bounds.length) {
      map.fitBounds(bounds, { padding: [60, 60], maxZoom: 15 });
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
