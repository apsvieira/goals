# tiny tracker — frontend

Svelte 5 + TypeScript SPA with offline-first architecture.

## Development

```bash
npm install
npm run dev
```

Dev server runs at http://localhost:5173 and proxies `/api` requests to the backend at `:8080`.

## Testing

```bash
npm run check          # Type checking (svelte-check + tsc)
npm run test           # Unit tests (vitest, watch mode)
npm run test -- --run  # Unit tests (single run)
npm run test:e2e       # E2E tests (Playwright)
```

## Build

```bash
npm run build    # Production build to dist/
```

## Mobile (Capacitor)

```bash
npm run build
npm run cap:sync
npm run cap:android
```
