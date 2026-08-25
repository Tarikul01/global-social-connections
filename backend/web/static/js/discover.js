function discoverApp() {
  return {
    map: null,
    markerLayer: null,
    lat: 23.8103,
    lng: 90.4125,
    init() {
      if (typeof L === 'undefined') return;
      this.map = L.map('map').setView([this.lat, this.lng], 12);
      L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
        maxZoom: 19,
        attribution: '&copy; OpenStreetMap'
      }).addTo(this.map);
      this.markerLayer = L.layerGroup().addTo(this.map);

      if (navigator.geolocation) {
        navigator.geolocation.getCurrentPosition((pos) => {
          this.lat = pos.coords.latitude;
          this.lng = pos.coords.longitude;
          this.map.setView([this.lat, this.lng], 12);
          this.saveLocation();
          this.loadNearby();
        });
      }
    },
    saveLocation() {
      const csrf = document.querySelector('input[name="csrf_token"]');
      const fd = new FormData();
      fd.append('lat', this.lat);
      fd.append('lng', this.lng);
      if (csrf) fd.append('csrf_token', csrf.value);
      fetch('/settings/location/update', { method: 'POST', body: fd });
    },
    loadNearby() {
      const radius = document.getElementById('radius').value;
      fetch(`/api/v1/users/nearby?lat=${this.lat}&lng=${this.lng}&radius_km=${radius}`)
        .then((r) => r.json())
        .then((body) => {
          const users = (body.data && body.data.users) || body.users || [];
          this.renderUsers(users);
        });
    },
    renderUsers(users) {
      this.markerLayer.clearLayers();
      const list = document.getElementById('nearby-list');
      list.innerHTML = users.map((u) => `
        <a href="/users/${u.username}" class="block p-4 bg-white rounded-xl border hover:border-indigo-300">
          <div class="font-semibold">${u.full_name}</div>
          <div class="text-sm text-slate-500">${u.profession || 'Professional'} · ${u.distance_km} km · ${u.online ? 'Online' : 'Offline'}</div>
        </a>`).join('');
      users.forEach((u) => {
        const icon = L.divIcon({
          className: '',
          html: `<div style="width:12px;height:12px;background:${u.online ? '#22c55e' : '#64748b'};border:2px solid #fff;border-radius:9999px;box-shadow:0 1px 4px rgba(0,0,0,.3)"></div>`,
          iconSize: [12, 12],
          iconAnchor: [6, 6]
        });
        L.marker([u.lat, u.lng], { icon }).addTo(this.markerLayer);
      });
    }
  };
}
