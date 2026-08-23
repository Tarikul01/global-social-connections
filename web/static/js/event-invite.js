document.addEventListener('alpine:init', () => {
  Alpine.data('eventInvitePanel', (cfg) => ({
    eventId: cfg.eventId,
    csrf: cfg.csrf,
    query: '',
    selected: [],
    suggestions: [],
    open: false,
    loading: false,
    timer: null,

    init() {
      document.addEventListener('click', (e) => {
        if (!this.$el.contains(e.target)) this.open = false;
      });
    },

    onInput() {
      clearTimeout(this.timer);
      this.timer = setTimeout(() => this.search(), 220);
    },

    async search() {
      const q = this.query.trim();
      if (q.length < 1) {
        this.suggestions = [];
        this.open = false;
        return;
      }
      this.loading = true;
      try {
        const res = await fetch('/events/' + this.eventId + '/users/search?q=' + encodeURIComponent(q), {
          headers: { Accept: 'application/json' }
        });
        const data = await res.json();
        const picked = new Set(this.selected.map((u) => u.id));
        this.suggestions = (data.users || []).filter((u) => !picked.has(u.id));
        this.open = this.suggestions.length > 0;
      } catch (_) {
        this.suggestions = [];
        this.open = false;
      } finally {
        this.loading = false;
      }
    },

    addUser(u) {
      if (this.selected.some((s) => s.id === u.id)) return;
      this.selected.push({
        id: u.id,
        username: u.username,
        email: u.email,
        full_name: u.full_name
      });
      this.query = '';
      this.suggestions = [];
      this.open = false;
    },

	removeUser(id) {
      this.selected = this.selected.filter((u) => u.id !== id);
    }
  }));
});
