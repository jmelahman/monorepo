async function updatePage(userIntput) {
  document.getElementById('loading').style.display = 'block';
  const response = await fetch('/update?text=' + userIntput);
  const result = await response.text();
  document.getElementById('loading').style.display = 'none';
  document.getElementById('result').innerHTML = result;
}