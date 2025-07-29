const usage = {};

async function checkSites() {
  const { hostname } = new URL((await chrome.tabs.query({ active: true, lastFocusedWindow: true }))[0]?.url || "");
  console.log(hostname);
  const config = (await chrome.storage.local.get(hostname))[hostname];
  if (!config) return;

  usage[hostname] = usage[hostname] || { used: 0 };
  usage[hostname].used++;

  if (usage[hostname].used > config.allowed) {
    chrome.tabs.query({}, (tabs) => {
      tabs.forEach((tab) => {
        if (tab.url.includes(hostname)) chrome.tabs.remove(tab.id);
      });
    });
  }

  setTimeout(checkSites, config.frequency * 60 * 1000);
}

checkSites();
