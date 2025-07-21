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

function transformItems(items) {
  const list = items.map(item => {
    return `<tr>
      <td><span>${item.feed}</span></td>
      <td><a href="${item.link}" target="_blank">${item.title}</a></td>
      <td><span>${item.date.toISOString().slice(0, 10)}</span></td>
    </tr>`;
  }).join("");
  return `<table style="width: 100%">
                <colgroup>
                  <col style="width: 20%">
                  <col style="width: 70%">
                  <col style="width: 10%">
                </colgroup>
                <tr>
                  <th>Blog</th>
                  <th>Title</th>
                  <th>Date</th>
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
