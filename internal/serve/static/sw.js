// Vcode app-shell service worker.
// Strategy: the shell must never be stale (the server stamps Cache-Control:
// no-store on "/"), so navigations are network-first with an offline fallback
// to the last-known shell. Static PWA assets are cache-first. SSE (/events)
// and every non-GET request bypass the worker entirely.
const CACHE = 'vcode-shell-v1';
const PRECACHE = ['/icons/icon-192.png', '/icons/icon-512.png', '/manifest.webmanifest'];

self.addEventListener('install', (e) => {
  e.waitUntil(caches.open(CACHE).then((c) => c.addAll(PRECACHE)).then(() => self.skipWaiting()));
});

self.addEventListener('activate', (e) => {
  e.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
      .then(() => self.clients.claim()),
  );
});

self.addEventListener('fetch', (e) => {
  const req = e.request;
  if (req.method !== 'GET') return;
  const url = new URL(req.url);
  if (url.origin !== self.location.origin) return;
  if (url.pathname === '/events' || url.pathname === '/login') return;

  if (req.mode === 'navigate') {
    e.respondWith(
      fetch(req)
        .then((resp) => {
          const copy = resp.clone();
          caches.open(CACHE).then((c) => c.put('/', copy));
          return resp;
        })
        .catch(() => caches.match('/')),
    );
    return;
  }

  e.respondWith(
    caches.match(req).then(
      (hit) =>
        hit ||
        fetch(req).then((resp) => {
          if (resp.ok && (url.pathname.startsWith('/icons/') || url.pathname === '/manifest.webmanifest')) {
            const copy = resp.clone();
            caches.open(CACHE).then((c) => c.put(req, copy));
          }
          return resp;
        }),
    ),
  );
});
