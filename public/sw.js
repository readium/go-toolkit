var CACHE_NAME = 'webpub-viewer';
//HINT: Make sure that this correctly points to the static resources used for the viewer
var urlsToCache = [
  'index.html',
  'sandbox.html',
  'viewer.js'
];

self.addEventListener('install', event => {
  event.waitUntil(
    caches.open(CACHE_NAME)
      .then(function(cache) {
        return cache.addAll(urlsToCache);
      })
  );
  self.skipWaiting();
});

self.addEventListener('activate', event => {
  clients.claim();
});

/* For EPUB files that are streamed, cache then network makes the most sense. */

self.addEventListener('fetch', event => {
  event.respondWith(
    caches.match(event.request).then(function(response) {
      return response || fetch(event.request);
    })
  );

});