function initLocationPicker(opts) {
  if (typeof L === 'undefined') return;

  const latEl = document.getElementById(opts.latInput);
  const lngEl = document.getElementById(opts.lngInput);
  const cityEl = document.getElementById(opts.cityInput);
  const regionEl = document.getElementById(opts.regionInput);
  const countryEl = document.getElementById(opts.countryInput);
  const labelEl = document.getElementById(opts.labelId);
  const root = document.getElementById(opts.rootId || 'location-picker-root');
  const mapEl = document.getElementById(opts.mapId);
  if (!mapEl || !latEl || !lngEl) return;

  let lat = Number(opts.lat) || 23.8103;
  let lng = Number(opts.lng) || 90.4125;
  const approxRadiusM = opts.approxRadiusM || 400;

  const map = L.map(opts.mapId, {
    zoomControl: false,
    attributionControl: true
  }).setView([lat, lng], 14);

  L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
    maxZoom: 19,
    attribution: '&copy; OpenStreetMap'
  }).addTo(map);

  const pinIcon = L.divIcon({
    className: '',
    html: '<div class="fn-loc__pin"><span></span></div>',
    iconSize: [28, 28],
    iconAnchor: [14, 14]
  });

  const marker = L.marker([lat, lng], { draggable: true, icon: pinIcon }).addTo(map);
  const approxCircle = L.circle([lat, lng], {
    radius: approxRadiusM,
    color: '#7c5cfc',
    weight: 2,
    fillColor: '#7c5cfc',
    fillOpacity: 0.15
  }).addTo(map);

  function setPoint(nextLat, nextLng, reverse) {
    lat = nextLat;
    lng = nextLng;
    latEl.value = String(lat);
    lngEl.value = String(lng);
    marker.setLatLng([lat, lng]);
    approxCircle.setLatLng([lat, lng]);
    if (labelEl) {
      labelEl.textContent = 'Approximate area around ' + lat.toFixed(5) + ', ' + lng.toFixed(5);
    }
    if (reverse) reverseGeocode(lat, lng);
  }

  function reverseGeocode(la, ln) {
    fetch('https://nominatim.openstreetmap.org/reverse?format=json&lat=' + la + '&lon=' + ln, {
      headers: { Accept: 'application/json' }
    })
      .then((r) => r.json())
      .then((data) => {
        const a = data.address || {};
        if (cityEl) cityEl.value = a.city || a.town || a.village || a.suburb || '';
        if (regionEl) regionEl.value = a.state || a.region || '';
        if (countryEl) countryEl.value = a.country || '';
        if (labelEl) {
          const place = a.suburb || a.neighbourhood || a.city || a.town || '';
          const region = a.state || a.region || '';
          labelEl.textContent = place
            ? ('Approximate area: ' + place + (region ? ', ' + region : ''))
            : (data.display_name || labelEl.textContent);
        }
      })
      .catch(() => {});
  }

  marker.on('dragend', () => {
    const p = marker.getLatLng();
    setPoint(p.lat, p.lng, true);
  });
  map.on('click', (e) => setPoint(e.latlng.lat, e.latlng.lng, true));

  function locateMe() {
    if (!navigator.geolocation) {
      alert('Location is not available in this browser.');
      return;
    }
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        setPoint(pos.coords.latitude, pos.coords.longitude, true);
        map.setView([pos.coords.latitude, pos.coords.longitude], 15);
      },
      () => alert('Could not get your location. Tap the map to drop a pin instead.'),
      { enableHighAccuracy: true, timeout: 12000 }
    );
  }

  function toggleExpand() {
    if (!root) return;
    root.classList.toggle('fn-loc--expanded');
    setTimeout(() => map.invalidateSize(), 200);
  }

  if (root) {
    root.querySelectorAll('[data-action]').forEach((btn) => {
      btn.addEventListener('click', (e) => {
        e.preventDefault();
        const action = btn.getAttribute('data-action');
        if (action === 'zoom-in') map.zoomIn();
        if (action === 'zoom-out') map.zoomOut();
        if (action === 'locate') locateMe();
        if (action === 'expand') toggleExpand();
      });
    });
  }

  const hasSaved = opts.lat && opts.lng && !(Number(opts.lat) === 23.8103 && Number(opts.lng) === 90.4125);
  if (navigator.geolocation && !hasSaved) {
    locateMe();
  } else {
    setPoint(lat, lng, true);
  }

  setTimeout(() => map.invalidateSize(), 120);
  window.addEventListener('resize', () => map.invalidateSize());
}
