const feeds = [
  { title: "Thorsten Ball", url: "https://registerspill.thorstenball.com/feed" },
  { title: "Kyla Scanlon", url: "https://kyla.substack.com/feed" },
  { title: "Sam Harris", url: "https://wakingup.libsyn.com/rss" },
  { title: "Pragmatic Engineer", url: "https://newsletter.pragmaticengineer.com/feed" },
  { title: "Evil Martians", url: "https://evilmartians.com/chronicles.atom" },
];

const proxy = 'https://corsproxy.io/?';
const CACHE_DURATION = 30 * 60 * 1000; // 30 minutes in milliseconds

// Initialize IndexedDB
function initDB() {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open('FeedCache', 1);

    request.onerror = () => reject(request.error);
    request.onsuccess = () => resolve(request.result);

    request.onupgradeneeded = (event) => {
      const db = event.target.result;
      const objectStore = db.createObjectStore('feeds', { keyPath: 'url' });
      objectStore.createIndex('timestamp', 'timestamp', { unique: false });
    };
  });
}

// Save feed data to IndexedDB
async function saveFeedToCache(url, data) {
  const db = await initDB();
  return new Promise((resolve, reject) => {
    const transaction = db.transaction(['feeds'], 'readwrite');
    const objectStore = transaction.objectStore('feeds');
    const request = objectStore.put({
      url: url,
      data: data,
      timestamp: Date.now()
    });

    request.onsuccess = () => resolve();
    request.onerror = () => reject(request.error);
  });
}

// Load feed data from IndexedDB
async function loadFeedFromCache(url) {
  const db = await initDB();
  return new Promise((resolve, reject) => {
    const transaction = db.transaction(['feeds'], 'readonly');
    const objectStore = transaction.objectStore('feeds');
    const request = objectStore.get(url);

    request.onsuccess = () => {
      const result = request.result;
      if (result && (Date.now() - result.timestamp) < CACHE_DURATION) {
        resolve(result.data);
      } else {
        resolve(null);
      }
    };
    request.onerror = () => reject(request.error);
  });
}

async function fetchFeed(url) {
  // Try to load from cache first
  const cachedData = await loadFeedFromCache(url);
  if (cachedData) {
    const parser = new DOMParser();
    return parser.parseFromString(cachedData, "application/xml");
  }

  // If not in cache or expired, fetch from network
  const res = await fetch(proxy + encodeURIComponent(url));
  const xml = await res.text();

  // Save to cache
  await saveFeedToCache(url, xml);

  const parser = new DOMParser();
  return parser.parseFromString(xml, "application/xml");
}

function parseItems(feed, items) {
  return [...items.querySelectorAll("item, entry")].map(item => ({
    title: item.querySelector("title")?.textContent,
    feed: feed.title,
    date: new Date(item.querySelector("published")?.textContent || item.querySelector("pubDate")?.textContent || 0),
    link: item.querySelector("link")?.getAttribute("href") || item.querySelector("link")?.textContent,
  }));
}

async function loadFeed(feed) {
  try {
    const doc = await fetchFeed(feed.url);
    return parseItems(feed, doc)
  } catch (err) {
    console.error(`Failed to load ${feed.url}: ${err}`);
  }
}

function toMMDDYYYY(date) {
  return [
    String(date.getMonth() + 1).padStart(2, '0'),
    String(date.getDate()).padStart(2, '0'),
    date.getFullYear()
  ].join('/');
}

function transformItems(items) {
  const list = items.map(item => {
    return `<tr>
      <td class="date-col">${toMMDDYYYY(item.date)}</td>
      <td><a href="${item.link}" target="_blank">${item.title}</a></td>
      <td><span style="white-space: nowrap">${item.feed}</span></td>
    </tr>`;
  }).join("");
  return `<table>
                <colgroup>
                  <col>
                  <col>
                  <col>
                </colgroup>
                <tr>
                  <th class="date-col">Date</th>
                  <th>Title</th>
                  <th>Feed</th>
                </tr>
                  ${list}
                </table>`;
}

async function render(promiseItems) {
  const content = document.getElementById("content");
  const allItems = await promiseItems;
  const sortedItems = allItems.flat().sort((a, b) => b.date - a.date).slice(0, 50);
  content.innerHTML = transformItems(sortedItems);
}

// Lazy fetch the feeds.
const promiseItems = Promise.all(feeds.map(loadFeed));

document.addEventListener('DOMContentLoaded', () => {
  render(promiseItems);
});
