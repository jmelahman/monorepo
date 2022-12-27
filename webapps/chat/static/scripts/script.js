async function updatePage(userIntput) {
  document.getElementById('result').innerHTML = 'Generating reply...';
  const response = await fetch('/update?text=' + userIntput);
  const result = await response.text();
  document.getElementById('result').innerHTML = result;
}