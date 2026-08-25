# Frontend — FindNeighbour

React + Vite + TypeScript SPA.

```bash
npm install
npm run dev
```

Dev server: http://localhost:5173  
Proxies `/api` → `http://localhost:8080` (see `vite.config.ts`).

Docker production build uses nginx and proxies `/api` to the `backend` service.
