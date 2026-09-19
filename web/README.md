# FARM web interface

Vue 3, Vite, and Tailwind CSS v4 provide FARM's read-only operations UI.

```sh
pnpm install
pnpm dev
```

The development server proxies `/api` and `/healthz` to
`http://127.0.0.1:8080`.

```sh
pnpm run type-check
pnpm run test:unit --run
pnpm run build
```

The production build is written below `internal/server/dist` and embedded in
the Go binary.
