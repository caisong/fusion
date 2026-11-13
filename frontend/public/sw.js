// sw.js is served from <base>/sw.js, so app URLs are resolved relative to its
// own location; this keeps the worker correct when FUSION_BASE_PATH is used.
const BASE_URL = new URL("./", self.location.href);

function appUrl(path) {
  return new URL(path, BASE_URL).pathname;
}

const CACHE_NAME = "fusion-static-v3";
const APP_SHELL = [
  appUrl("./"),
  appUrl("index.html"),
  appUrl("manifest.json"),
  appUrl("favicon.ico"),
  appUrl("icon-96.png"),
  appUrl("icon-192.png"),
  appUrl("icon-512.png"),
  appUrl("apple-touch-icon.png"),
];
const API_PREFIX = appUrl("api/");

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches
      .open(CACHE_NAME)
      .then((cache) => cache.addAll(APP_SHELL)),
  );
});

self.addEventListener("message", (event) => {
  if (event.data?.type === "SKIP_WAITING") {
    void self.skipWaiting();
  }
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) =>
        Promise.all(
          keys.filter((key) => key !== CACHE_NAME).map((key) => caches.delete(key)),
        ),
      )
      .then(() => self.clients.claim()),
  );
});

function shouldHandleRequest(request) {
  if (request.method !== "GET") {
    return false;
  }

  const url = new URL(request.url);
  if (url.origin !== self.location.origin) {
    return false;
  }

  if (url.pathname.startsWith(API_PREFIX)) {
    return false;
  }

  return true;
}

async function networkFirst(request) {
  try {
    const response = await fetch(request);
    const cache = await caches.open(CACHE_NAME);
    await cache.put(request, response.clone());
    return response;
  } catch {
    const cachedResponse = await caches.match(request);
    if (cachedResponse) {
      return cachedResponse;
    }

    const appShell = await caches.match(appUrl("index.html"));
    if (appShell) {
      return appShell;
    }

    return new Response("Offline", { status: 503, statusText: "Offline" });
  }
}

async function staleWhileRevalidate(request) {
  const cache = await caches.open(CACHE_NAME);
  const cachedResponse = await cache.match(request);

  const networkResponsePromise = fetch(request)
    .then((response) => {
      if (response.ok) {
        void cache.put(request, response.clone());
      }
      return response;
    })
    .catch(() => null);

  if (cachedResponse) {
    return cachedResponse;
  }

  const networkResponse = await networkResponsePromise;
  if (networkResponse) {
    return networkResponse;
  }

  return new Response("Offline", { status: 503, statusText: "Offline" });
}

self.addEventListener("fetch", (event) => {
  const { request } = event;
  if (!shouldHandleRequest(request)) {
    return;
  }

  if (request.mode === "navigate") {
    event.respondWith(networkFirst(request));
    return;
  }

  event.respondWith(staleWhileRevalidate(request));
});
