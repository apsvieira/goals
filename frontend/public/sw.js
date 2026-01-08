/**
 * Service Worker Cache Strategy
 *
 * CACHE VERSIONING:
 * - Update CACHE_VERSION when deploying code changes to ensure users get fresh JS
 * - Version format: YYYYMMDD-HHMMSS or commit hash
 * - Manual update: Change the timestamp/hash below before building for production
 * - Automated update: Use build script to inject version (e.g., via environment variable)
 *
 * CACHE STRATEGIES BY FILE TYPE:
 * - JavaScript files (.js): Network-first with cache fallback
 *   Why: Users must get fresh code after deployments to prevent IndexedDB version mismatches
 * - API requests (/api/*): Network-only
 *   Why: Always fetch fresh data from the server
 * - Static assets (images, fonts, HTML, CSS): Cache-first with network fallback
 *   Why: These change less frequently and benefit from offline availability
 */

// IMPORTANT: Update this version on each deployment to bust the cache
// Format: YYYYMMDD-HHMMSS or commit hash
// TODO: Automate this via build script (e.g., inject via Vite environment variable)
const CACHE_VERSION = '20260108-000000'; // Manual update required
const CACHE_NAME = `goal-tracker-v${CACHE_VERSION}`;

// Network timeout before falling back to cache (milliseconds)
const NETWORK_TIMEOUT = 3000;

// Assets to pre-cache on install
const STATIC_ASSETS = [
  '/',
  '/index.html',
  '/manifest.json'
];

/**
 * Install event: Pre-cache critical static assets
 * skipWaiting() ensures the new service worker activates immediately
 */
self.addEventListener('install', (event) => {
  console.log('[Service Worker] Installing version:', CACHE_VERSION);
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => {
      console.log('[Service Worker] Pre-caching static assets');
      return cache.addAll(STATIC_ASSETS);
    })
  );
  self.skipWaiting();
});

/**
 * Activate event: Clean up old caches
 * This ensures users don't keep stale caches indefinitely
 */
self.addEventListener('activate', (event) => {
  console.log('[Service Worker] Activating version:', CACHE_VERSION);
  event.waitUntil(
    caches.keys().then((keys) => {
      const oldCaches = keys.filter((key) => key !== CACHE_NAME);
      if (oldCaches.length > 0) {
        console.log('[Service Worker] Deleting old caches:', oldCaches);
      }
      return Promise.all(
        oldCaches.map((key) => {
          console.log('[Service Worker] Deleted cache:', key);
          return caches.delete(key);
        })
      );
    })
  );
  self.clients.claim();
});

/**
 * Fetch event: Implement cache strategies based on resource type
 */
self.addEventListener('fetch', (event) => {
  const url = new URL(event.request.url);

  // API requests: Network only (we want fresh data)
  if (url.pathname.startsWith('/api/')) {
    event.respondWith(fetch(event.request));
    return;
  }

  // JavaScript files: Network-first with cache fallback and timeout
  // This prevents stale code from being served after deployments
  if (url.pathname.endsWith('.js')) {
    event.respondWith(
      networkFirstWithTimeout(event.request, NETWORK_TIMEOUT)
    );
    return;
  }

  // All other static assets: Cache-first with network fallback
  // Images, fonts, CSS, HTML benefit from offline availability
  event.respondWith(
    cacheFirstWithNetworkFallback(event.request)
  );
});

/**
 * Network-first strategy with timeout
 * Try network first, fall back to cache if network fails or times out
 * Always update cache with successful network responses
 */
async function networkFirstWithTimeout(request, timeout) {
  try {
    // Race between network fetch and timeout
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), timeout);

    const response = await fetch(request, { signal: controller.signal });
    clearTimeout(timeoutId);

    // Cache successful responses for offline fallback
    if (response.ok) {
      const cache = await caches.open(CACHE_NAME);
      cache.put(request, response.clone());
    }

    return response;
  } catch (error) {
    // Network failed or timed out, try cache
    console.log('[Service Worker] Network failed for', request.url, '- falling back to cache');
    const cached = await caches.match(request);

    if (cached) {
      return cached;
    }

    // No cache available, re-throw error
    throw error;
  }
}

/**
 * Cache-first strategy with network fallback
 * Return cached version if available, otherwise fetch from network
 * Cache successful network responses for future use
 */
async function cacheFirstWithNetworkFallback(request) {
  const cached = await caches.match(request);

  if (cached) {
    return cached;
  }

  try {
    const response = await fetch(request);

    // Cache successful responses
    if (response.ok) {
      const cache = await caches.open(CACHE_NAME);
      cache.put(request, response.clone());
    }

    return response;
  } catch (error) {
    console.error('[Service Worker] Fetch failed for', request.url, error);
    throw error;
  }
}
