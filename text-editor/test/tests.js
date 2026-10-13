'use strict';

// Added to the editor's page by test/index.html, after app.js, so every name
// app.js declares is in reach here as if this were more of the same file.
//
// Every test is synchronous, and that is kept to on purpose: a change to the
// document settles on a timer, which saves the draft and keeps a revision,
// and none of those timers can fire before the last test has returned. The
// one still waiting by then is cleared, so that a run by hand in the browser
// the editor is used in leaves its draft and revisions as they were.

{
  const results = [];

  function test(name, body) {
    try {
      body();
      results.push({ name });
    } catch (error) {
      results.push({ name, error: String(error?.message ?? error) });
    }
  }

  function equal(actual, expected) {
    if (actual !== expected) {
      throw new Error(`expected ${JSON.stringify(expected)}\n     got ${JSON.stringify(actual)}`);
    }
  }

  // Whether rich mode would have to be asked for.
  function loses(text) {
    buildEditor(text);
    return losesHtml();
  }

  // The document as rich mode writes it back with nothing edited.
  function throughRich(text) {
    buildEditor(text);
    if (losesHtml()) throw new Error('Rich mode would not open this without asking.');
    if (!enterRich()) throw new Error('Rich mode could not show this.');
    editor.changeMode('markdown', true);
    return editor.getMarkdown();
  }

  // ---------------------------------------------------------------- sanitizer

  test('sanitize: a script goes, contents and all', () => {
    equal(sanitize('<p>a<script>alert(1)</script>b</p>'), '<p>ab</p>');
    equal(sanitize('<iframe src="https://example.com">x</iframe><style>p{}</style>y'), 'y');
  });

  test('sanitize: an unknown tag goes and its children stay', () => {
    equal(sanitize('<blink><b>x</b></blink>'), '<b>x</b>');
    equal(sanitize('<my-widget>x <em>y</em></my-widget>'), 'x <em>y</em>');
  });

  test('sanitize: only allowlisted attributes are kept', () => {
    equal(sanitize('<p onclick="alert(1)" style="color:red" title="t">a</p>'), '<p title="t">a</p>');
    equal(sanitize('<img src="https://example.com/a.png" onerror="alert(1)" alt="a">'), '<img src="https://example.com/a.png" alt="a">');
  });

  test('sanitize: a link keeps only an address that is safe to follow', () => {
    equal(sanitize('<a href="javascript:alert(1)">x</a>'), '<a>x</a>');
    // Browsers read through whitespace in the scheme.
    equal(sanitize('<a href="java\tscript:alert(1)">x</a>'), '<a>x</a>');
    equal(sanitize('<a href=" JAVASCRIPT:alert(1)">x</a>'), '<a>x</a>');
    equal(sanitize('<a href="data:text/html,x">x</a>'), '<a>x</a>');
    equal(sanitize('<a href="mailto:a@example.com">x</a>').includes('href="mailto:a@example.com"'), true);
  });

  test('sanitize: a link that leaves the page opens apart from it', () => {
    equal(sanitize('<a href="https://example.com">x</a>'), '<a href="https://example.com" target="_blank" rel="noopener noreferrer">x</a>');
    equal(sanitize('<a href="#top" target="_top">x</a>'), '<a href="#top">x</a>');
  });

  test('sanitize: an embedded image has to be a picture', () => {
    equal(sanitize('<img src="data:image/png;base64,AAAA">'), '<img src="data:image/png;base64,AAAA">');
    equal(sanitize('<img src="data:image/svg+xml,<svg/>">'), '<img>');
    equal(sanitize('<img src="data:text/html,x">'), '<img>');
    equal(sanitize('<img src="javascript:alert(1)">'), '<img>');
    equal(sanitize('<p src="https://example.com/a.png">x</p>'), '<p>x</p>');
  });

  test('sanitize: a form goes, and the only input kept is a dead checkbox', () => {
    // A control named for a property would otherwise stand in for it.
    equal(sanitize('<form><input name="attributes"><input name="childNodes"></form>x'), 'x');
    equal(sanitize('<input type="text" value="x">y'), 'y');
    const box = document.createElement('template');
    box.innerHTML = sanitize('<input type="checkbox" checked onclick="alert(1)">');
    const input = box.content.firstElementChild;
    equal(input.localName, 'input');
    equal(input.disabled, true);
    equal(input.checked, true);
    equal(input.hasAttribute('onclick'), false);
  });

  test('sanitize: an SVG is a drawing and nothing else', () => {
    equal(
      sanitize('<svg viewBox="0 0 1 1" onload="alert(1)"><script>alert(1)</script><style>rect{}</style>'
        + '<foreignObject><p>x</p></foreignObject><use href="#a"></use><rect width="1" height="1" fill="#888" onclick="alert(1)"></rect></svg>'),
      '<svg viewBox="0 0 1 1"><rect width="1" height="1" fill="#888"></rect></svg>',
    );
  });

  test('sanitize: an SVG paint cannot fetch', () => {
    equal(sanitize('<svg><rect fill="url(https://example.com/a.svg#b)" stroke="#000"></rect></svg>'), '<svg><rect stroke="#000"></rect></svg>');
    equal(sanitize('<svg><rect fill="URL (#gradient)"></rect></svg>'), '<svg><rect></rect></svg>');
    equal(sanitize('<svg><a href="javascript:alert(1)"><text>x</text></a></svg>'), '<svg><a><text>x</text></a></svg>');
  });

  test('sanitize: strict drops what the editor works with', () => {
    equal(sanitize('<p class="a" data-nodeid="1">x</p>'), '<p class="a" data-nodeid="1">x</p>');
    equal(sanitize('<p class="a" data-nodeid="1">x</p>', true), '<p>x</p>');
    equal(sanitize('<svg class="html-block"><rect></rect></svg>', true), '<svg><rect></rect></svg>');
  });

  test('sanitize: strict keeps the language of a code block', () => {
    equal(sanitize('<pre><code data-language="js">x</code></pre>', true), '<pre><code class="language-js">x</code></pre>');
    equal(sanitize('<code class="language-js evil">x</code>', true), '<code>x</code>');
  });

  test('sanitize: strict draws a task as a real checkbox', () => {
    const page = document.createElement('template');
    page.innerHTML = sanitize('<ul><li data-task data-task-checked><p>done</p></li><li data-task>open</li></ul>', true);
    const [done, open] = page.content.querySelectorAll('input');
    equal(done.parentElement.localName, 'p');
    equal(done.checked, true);
    equal(done.disabled, true);
    equal(open.parentElement.localName, 'li');
    equal(open.checked, false);
    equal(page.content.querySelector('[data-task]'), null);
  });

  test('safeUrl: an address without a scheme is relative, and safe', () => {
    equal(safeUrl('a/b.md', ['http']), true);
    equal(safeUrl('#top', ['http']), true);
    equal(safeUrl('HTTP://example.com', ['http']), true);
    equal(safeUrl('ftp://example.com', ['http', 'https']), false);
    equal(safeUrl('\x01javascript:alert(1)', ['http']), false);
  });

  // ---------------------------------------------------------------- what rich mode keeps

  test('losesHtml: plain Markdown loses nothing', () => {
    equal(loses('# Title\n\nSome *text* and `code`.\n\n- a\n- b\n\n| a |\n| - |\n| b |'), false);
  });

  test('losesHtml: a block of a known tag is kept', () => {
    equal(loses('<div class="note" id="n">\nhello\n</div>'), false);
    equal(loses('<details>\n<summary>More</summary>\nhidden\n</details>'), false);
    equal(loses('<svg viewBox="0 0 1 1" style="max-width:100%">\n<rect width="1" height="1"/>\n</svg>'), false);
  });

  test('losesHtml: a block of any other tag is not', () => {
    equal(loses('<table>\n<tr><td>x</td></tr>\n</table>'), true);
    equal(loses('<p align="center">\nx\n</p>'), true);
  });

  test('losesHtml: nor is a block nested in another of its name', () => {
    equal(loses('<div>\n<div>\ninner\n</div>\n</div>'), true);
    equal(loses('<div>\n<section>\ninner\n</section>\n</div>'), false);
  });

  test('losesHtml: two blocks that differ only in the opening tag cannot be told apart', () => {
    equal(loses('<div class="a">\nsame\n</div>\n\n<div class="b">\nsame\n</div>'), true);
    equal(loses('<div class="a">\nsame\n</div>\n\n<div class="a">\nsame\n</div>'), false);
    equal(loses('<div class="a">\none\n</div>\n\n<div class="b">\ntwo\n</div>'), false);
  });

  test('losesHtml: a comment is kept', () => {
    equal(loses('<!-- a note -->\n\ntext'), false);
    equal(loses('<!--\n<table><tr><td>x</td></tr></table>\n-->\n\ntext'), false);
  });

  test('losesHtml: an inline tag is kept where the editor has a place for it', () => {
    equal(loses('Press <kbd>Ctrl</kbd> and H<sub>2</sub>O, <b>bold</b>.'), false);
    equal(loses('a <span>x</span>'), true);
    equal(loses('a <kbd class="key">x</kbd>'), true);
    equal(loses('a <font color="red">x</font>'), true);
  });

  test('losesHtml: a tag with attributes is kept only as the editor rebuilds it', () => {
    equal(loses('a <a href="https://example.com">x</a>'), false);
    equal(loses('a <a href="https://example.com" title="t">x</a>'), true);
    equal(loses('a <a name="top">x</a>'), true);
    equal(loses('a <img src="a.png">'), false);
    equal(loses('a <img src="a.png" width="10">'), true);
    equal(loses('an <abbr title="Hypertext">HTML</abbr> and an <abbr>SVG</abbr>'), false);
  });

  test('losesHtml: a line break is kept in a table', () => {
    equal(loses('| a |\n| - |\n| x<br>y |'), false);
  });

  // The blog post that asked the question: charts that had been cut and pasted
  // in rich mode came back with their contents on the line of the opening tag.
  test('losesHtml: an SVG whose contents share the line of its opening tag is no block', () => {
    const text = '<svg viewBox="0 0 1 1"><rect width="1" height="1"></rect>\n</svg>';
    buildEditor(text);
    equal(htmlBlocks().length, 0);
    equal(losesHtml(), true);
  });

  test('htmlBlocks: each block comes with its tag and what is inside it', () => {
    buildEditor('<!-- skipped -->\n\n<DIV class="a">\n  inside\n</DIV>\n\ntext\n\n<pre>\nx\n</pre>');
    const blocks = htmlBlocks();
    equal(blocks.length, 2);
    equal(blocks[0].tag, 'div');
    equal(blocks[0].children, 'inside');
    equal(blocks[0].kept, true);
    equal(blocks[0].literal, '<DIV class="a">\n  inside\n</DIV>');
    equal(blocks[1].tag, 'pre');
    equal(blocks[1].kept, false);
  });

  // ---------------------------------------------------------------- through rich mode and back

  test('rich mode: an HTML block is written back as it was read', () => {
    for (const block of [
      '<div class="note"   id="n">\n  Some <em>text</em>,\n\tindented   oddly.\n</div>',
      '<details open>\n<summary>More</summary>\n<p>hidden</p>\n</details>',
      '<svg viewBox="0 0 10 10" role="img" aria-labelledby="t" style="max-width:100%;height:auto">\n'
        + '  <title id="t">A square</title>\n  <rect x="1" y="1" width="8" height="8" fill="#888"/>\n</svg>',
    ]) {
      equal(throughRich(`Before\n\n${block}\n\nAfter`), `Before\n\n${block}\n\nAfter`);
    }
  });

  test('rich mode: two of the same block are both written back', () => {
    const block = '<div class="a">\nsame\n</div>';
    equal(throughRich(`${block}\n\nbetween\n\n${block}`), `${block}\n\nbetween\n\n${block}`);
  });

  test('rich mode: front matter and comments are written back', () => {
    const text = '---\ntitle: "A title"\ntags: [a, b]\n---\n\n# Heading\n\n<!-- a note -->\n\ntext';
    equal(throughRich(text), text);
  });

  test('rich mode: inline tags are written back', () => {
    const text = 'Press <kbd>Ctrl</kbd>+<kbd>S</kbd>, H<sub>2</sub>O, an <abbr title="Hypertext">HTML</abbr> <mark>mark</mark>.';
    equal(throughRich(text), text);
  });

  test('rich mode: the lines of a paragraph run on into one', () => {
    equal(throughRich('one\ntwo\nthree\n\nfour'), 'one two three\n\nfour');
  });

  test('rich mode: a blank line is not an empty paragraph', () => {
    equal(throughRich('one\n\n\n\ntwo'), 'one\n\ntwo');
  });

  test('rich mode: a break forced within a paragraph starts a new one', () => {
    equal(throughRich('one  \ntwo'), 'one\n\ntwo');
    equal(throughRich('one\\\ntwo'), 'one\n\ntwo');
  });

  test('rich mode: Markdown comes back in the editor\'s own style', () => {
    equal(throughRich('- a\n- b'), '* a\n* b');
    equal(throughRich('_soft_ and __loud__'), '*soft* and **loud**');
    equal(throughRich('# Title\n\n> quote\n\n```js\nlet x;\n```'), '# Title\n\n> quote\n\n```js\nlet x;\n```');
  });

  test('rich mode: an SVG is drawn as one, not as an HTML element of that name', () => {
    buildEditor('<svg viewBox="0 0 10 10">\n<rect width="8" height="8" onclick="alert(1)"/>\n</svg>');
    enterRich();
    const svg = editor.wwEditor.view.dom.querySelector('svg');
    equal(svg instanceof SVGSVGElement, true);
    equal(svg.classList.contains('html-block'), true);
    equal(svg.firstElementChild instanceof SVGRectElement, true);
    equal(svg.firstElementChild.hasAttribute('onclick'), false);
  });

  // ---------------------------------------------------------------- a block without its source

  // What rich mode hands over for a block it has no source for, as one that
  // was cut and pasted there is.
  const pasted = (tag, htmlAttrs, childrenHTML) => {
    htmlSources = new Map();
    return htmlToMarkdown({ node: { type: { name: tag }, attrs: { htmlBlock: true, htmlAttrs, childrenHTML } } }, {}).text;
  };

  test('htmlToMarkdown: a block without its source is written so as to be read as a block', () => {
    const text = pasted('svg', { class: 'html-block', viewBox: '0 0 1 1' }, '<rect width="1"></rect>\n\n  \n<circle r="1"></circle>');
    equal(text, '<svg viewBox="0 0 1 1">\n<rect width="1"></rect>\n<circle r="1"></circle>\n</svg>');
    buildEditor(`Before\n\n${text}\n\nAfter`);
    const blocks = htmlBlocks();
    equal(blocks.length, 1);
    equal(blocks[0].literal, text);
    equal(losesHtml(), false);
  });

  test('htmlToMarkdown: it keeps the document\'s classes and not the editor\'s', () => {
    equal(pasted('div', { class: 'note html-block wide' }, 'x'), '<div class="note wide">\nx\n</div>');
    equal(pasted('div', { title: 'a "quoted" one' }, 'x'), '<div title="a &quot;quoted&quot; one">\nx\n</div>');
  });

  test('htmlToMarkdown: a block with its source is written back as the source', () => {
    htmlSources = new Map([['div\nx', '<div   class="a">\n   x\n</div>']]);
    const node = { type: { name: 'div' }, attrs: { htmlBlock: true, htmlAttrs: { class: 'a html-block' }, childrenHTML: 'x' } };
    equal(htmlToMarkdown({ node }, {}).text, '<div   class="a">\n   x\n</div>');
  });

  test('htmlToMarkdown: inline HTML is left to the editor', () => {
    const node = { type: { name: 'kbd' }, attrs: { htmlInline: true } };
    equal(htmlToMarkdown({ node }, { origin: () => 'theirs' }), 'theirs');
  });

  // ---------------------------------------------------------------- pasting

  test('flattenPasted: a block of text becomes a paragraph', () => {
    equal(flattenPasted('<div>hello <b>there</b></div>'), '<p>hello <b>there</b></p>');
  });

  test('flattenPasted: a block of blocks is unwrapped', () => {
    equal(flattenPasted('<article><h2>T</h2><p>a</p></article>'), '<h2>T</h2><p>a</p>');
    equal(flattenPasted('<main><section><div>a</div></section></main>'), '<p>a</p>');
  });

  test('flattenPasted: a drawing is left behind', () => {
    equal(flattenPasted('<p>a</p><svg><rect></rect></svg>'), '<p>a</p>');
  });

  test('flattenPasted: what was copied from the editor is left alone', () => {
    const html = '<div data-pm-slice="0 0 []"><svg><rect></rect></svg></div>';
    equal(flattenPasted(html), html);
  });

  // ---------------------------------------------------------------- which mode a document opens in

  test('createEditor: a document rich mode can keep opens in it', () => {
    const asked = mode;
    try {
      mode = 'wysiwyg';
      createEditor('# Title\n\n<div>\nkept\n</div>');
      equal(editor.isWysiwygMode(), true);
      equal(modeNote(), '');
      equal(document.querySelector('[data-mode="wysiwyg"]').getAttribute('aria-checked'), 'true');
      equal(document.querySelector('[data-mode="markdown"]').getAttribute('aria-checked'), 'false');
    } finally {
      mode = asked;
    }
  });

  test('createEditor: one it cannot opens as Markdown, and says so', () => {
    const asked = mode;
    try {
      mode = 'wysiwyg';
      createEditor('a <span>x</span>');
      equal(editor.isWysiwygMode(), false);
      equal(modeNote(), 'Opened as Markdown, as Rich mode cannot keep everything in this document.');
      equal(document.querySelector('[data-mode="markdown"]').getAttribute('aria-checked'), 'true');
      equal(editor.getMarkdown(), 'a <span>x</span>');
    } finally {
      mode = asked;
    }
  });

  test('createEditor: Markdown mode is kept to when it was asked for', () => {
    const asked = mode;
    try {
      mode = 'markdown';
      createEditor('plain');
      equal(editor.isWysiwygMode(), false);
      equal(modeNote(), '');
    } finally {
      mode = asked;
    }
  });

  test('showMode: only the editor in sight is spell-checked', () => {
    buildEditor('text');
    showMode();
    equal(editor.mdEditor.view.dom.spellcheck, true);
    equal(editor.wwEditor.view.dom.spellcheck, false);
    enterRich();
    showMode();
    equal(editor.mdEditor.view.dom.spellcheck, false);
    equal(editor.wwEditor.view.dom.spellcheck, true);
  });

  // ---------------------------------------------------------------- export

  test('export: the page is the document without the editor\'s marks', () => {
    buildEditor('# Title\n\n- [x] done\n- [ ] open\n\n```js\nlet x;\n```\n\n<div class="note">\nkept\n</div>');
    const page = document.createElement('template');
    page.innerHTML = sanitize(editor.getHTML(), true);
    equal(page.content.querySelector('h1').textContent, 'Title');
    equal(page.content.querySelectorAll('input[type="checkbox"]').length, 2);
    equal(page.content.querySelector('input').checked, true);
    equal(page.content.querySelector('pre > code').className, 'language-js');
    // The editor wraps a code block in a div of its own.
    equal([...page.content.querySelectorAll('div')].some((div) => div.textContent.trim() === 'kept'), true);
    equal([...page.content.querySelectorAll('*')].some((el) => [...el.attributes].some((a) => a.name.startsWith('data-'))), false);
  });

  // ---------------------------------------------------------------- files

  test('isMarkdown, isText: what can be opened', () => {
    equal(isMarkdown('a.md'), true);
    equal(isMarkdown('A.MARKDOWN'), true);
    equal(isMarkdown('notes.txt'), true);
    equal(isMarkdown('a.md.png'), false);
    equal(isMarkdown('md'), false);
    equal(isText({ type: '', name: 'README' }), true);
    equal(isText({ type: 'text/x-python', name: 'a.py' }), true);
    equal(isText({ type: 'application/octet-stream', name: 'a.md' }), true);
    equal(isText({ type: 'image/png', name: 'a.png' }), false);
  });

  test('baseName, fileUrl: a path among the server\'s files', () => {
    equal(baseName('a/b/c.md'), 'c.md');
    equal(baseName('c.md'), 'c.md');
    equal(fileUrl('c.md'), '/files/c.md');
    equal(fileUrl('a b/c#d?.md'), '/files/a%20b/c%23d%3F.md');
  });

  test('escapeHtml: text for a page', () => {
    equal(escapeHtml('<a href="x">Tom & Jerry</a>'), '&lt;a href=&quot;x&quot;&gt;Tom &amp; Jerry&lt;/a&gt;');
  });

  // test/index.html puts the draft back as it was once these are done.
  test('loadDraft: a draft that cannot be read is no draft', () => {
    localStorage.setItem(DRAFT_KEY, '{not json');
    equal(loadDraft(), null);
    localStorage.setItem(DRAFT_KEY, JSON.stringify({ name: 'a.md' }));
    equal(loadDraft(), null);
    localStorage.setItem(DRAFT_KEY, JSON.stringify({ name: 'a.md', text: '# a', saved: true }));
    equal(loadDraft().text, '# a');
    localStorage.removeItem(DRAFT_KEY);
    equal(loadDraft(), null);
  });

  // Nothing here is to be saved; see the top of the file.
  clearTimeout(settleTimer);
  settleTimer = 0;
  clearTimeout(revisionTimer);
  revisionTimer = 0;
  parent.report(results);
}
