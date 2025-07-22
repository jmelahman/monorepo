const feeds = [
  { title: "Daring Fireball", url: "https://daringfireball.net/feeds/main" },
  { title: "Thorsten Ball", url: "https://registerspill.thorstenball.com/feed" },
];

const proxy = 'https://corsproxy.io/?';

async function fetchFeed(url) {
  const res = await fetch(proxy + encodeURIComponent(url));
  const xml = await res.text();
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
      <td><span class="monospace">${toMMDDYYYY(item.date)}</span></td>
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
                  <th>Date</th>
                  <th>Title</th>
                  <th>Feed</th>
                </tr>
                  ${list}
                </table>`;
}

async function render(promiseItems) {
  const content = document.getElementById("content");
  const allItems = await promiseItems;
  const sortedItems = allItems.flat().sort((a, b) => b.date - a.date);
  content.innerHTML = transformItems(sortedItems);
}

// Lazy fetch the feeds.
const promiseItems = Promise.all(feeds.map(loadFeed));

document.addEventListener('DOMContentLoaded', () => {
  render(promiseItems);
});
