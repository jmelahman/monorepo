document.addEventListener("DOMContentLoaded", async () => {
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  const url = new URL(tab.url);
  document.getElementById("site").value = url.hostname;

  document.getElementById("save").onclick = async () => {
    const site = document.getElementById("site").value;
    const frequency = parseInt(document.getElementById("frequency").value);
    const allowed = parseInt(document.getElementById("allowed").value);
    const config = { site, frequency, allowed };

    chrome.storage.local.set({ [site]: config });
  };
});
