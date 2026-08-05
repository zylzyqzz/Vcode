const CACHE = 'vcode-mobile-v1';
const SHELL = ['/manifest.json', '/sw.js'];
self.addEventListener('install', event => {
  event.waitUntil(caches.open(CACHE).then(cache => cache.addAll(SHELL)));
  self.skipWaiting();
});
self.addEventListener('activate', event => {
  event.waitUntil(self.clients.claim());
});
self.addEventListener('fetch', event => {
  if (event.request.method !== 'GET' || event.request.url.includes('/events')) return;
  event.respondWith(fetch(event.request).catch(() => caches.match(event.request)));
});
