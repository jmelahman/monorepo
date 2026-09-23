---
title-prefix: "Projects"
nav_projects: true
---
<article class="project">
<div class="project-head">
<h2 id="aur-report-card"><a href="https://jamison.lahman.dev/pkglint/">AUR Report Card</a></h2>
<div class="chips"><span class="chip">Static site</span><span class="chip">pkglint</span></div>
</div>

Generated reports on the quality of every PKGBUILD in the Arch User Repository and the
official Arch repositories.

Each package is statically analyzed by pkglint and given a letter grade, from A for no
warnings down to F for a critical finding.
Every package gets its own page listing each finding and whether it is auto-fixable, and
the full results are published as JSON.

<div class="grades" aria-label="Distribution of letter grades across scanned packages">
<div class="grade-bar"><span class="band-A" style="flex-grow: 24870">A</span><span class="band-B" style="flex-grow: 17009">B</span><span class="band-C" style="flex-grow: 3441">C</span><span class="band-D" style="flex-grow: 2093">D</span><span class="band-F" style="flex-grow: 206">F</span></div>
<div class="grade-caption">grade distribution across scanned packages</div>
</div>

<ul class="project-links">
<li><a href="https://jamison.lahman.dev/pkglint/"><i class="fas fa-clipboard-check"></i>Report card</a></li>
<li><a href="https://jamison.lahman.dev/pkglint/rules/"><i class="fas fa-list-check"></i>Rules</a></li>
<li><a href="https://jamison.lahman.dev/pkglint/results.json"><i class="fas fa-code"></i>JSON</a></li>
<li><a href="https://github.com/jmelahman/pkglint"><i class="fab fa-github"></i>Source</a></li>
</ul>
</article>

<article class="project">
<div class="project-head">
<h2 id="pkglint"><a href="https://github.com/jmelahman/pkglint">pkglint</a></h2>
<div class="chips"><span class="chip">Go</span><span class="chip">Arch Linux</span><span class="chip">GPL-3.0</span></div>
</div>

A security-focused linter for Arch Linux packages.

It statically analyzes PKGBUILDs and their install scriptlets without ever sourcing them,
and reports findings on source integrity, build hermeticity, code execution, and
persistence, condensed into a letter grade per package.
It is built on a real bash AST, so the quoting tricks that slip past regex-based scanners
don't work here.
Built packages are inspected too, in the style of namcap, without executing anything from
the archive.

<div class="terminal">
<div class="terminal-bar"><i></i><i></i><i></i><span>pkglint</span></div>
<pre><code><span class="muted">$</span> pkglint ~/pkgbuilds/somepkg
somepkg: <span class="grade">grade F</span>, 3 finding(s)
  <span class="muted">PKGBUILD:16:3:</span> <span class="sev-critical">critical</span> [PB304] a network download is piped straight into bash and executed
  <span class="muted">PKGBUILD:11:1:</span> <span class="sev-error">error</span> [PB101] remote source "http://..." has no checksum (SKIP): the download is never verified
  <span class="muted">PKGBUILD:24:3:</span> <span class="sev-warn">warn</span> [PB403] chmod 4755 creates a setuid/setgid file

1 package linted: 1 with findings
1 finding(s) fixable with --unsafe-fix</code></pre>
</div>

<ul class="project-links">
<li><a href="https://github.com/jmelahman/pkglint"><i class="fab fa-github"></i>Source</a></li>
<li><a href="https://aur.archlinux.org/packages/pkglint"><i class="fab fa-linux"></i>AUR</a></li>
<li><a href="https://pypi.org/project/pkglint/"><i class="fab fa-python"></i>PyPI</a></li>
<li><a href="https://pkg.go.dev/github.com/jmelahman/pkglint"><i class="fab fa-golang"></i>Go</a></li>
<li><a href="https://github.com/jmelahman/pkglint/releases/latest"><i class="fas fa-download"></i>Releases</a></li>
</ul>
</article>

<article class="project">
<div class="project-head">
<h2 id="local-preview"><a href="https://github.com/jmelahman/local-preview">local-preview</a></h2>
<div class="chips"><span class="chip">Go</span><span class="chip">Docker</span><span class="chip">GPL-3.0</span></div>
</div>

A local-first preview-deployment orchestrator.

Every commit of a registered git repository becomes a servable preview at its own
subdomain, built once, deduplicated by content, and served from a single binary.
Commits that don't touch the frontend or backend reuse the existing artifact and the
already-running backend process.
Backend state follows git lineage: a new backend version forks its data from the nearest
deployed ancestor, so previews feel continuous along a branch while divergent branches can
never corrupt each other.

<div class="terminal">
<div class="terminal-bar"><i></i><i></i><i></i><span>local-preview</span></div>
<pre><code><span class="muted">$</span> preview serve
<span class="muted">$</span> preview deploy
<span class="grade">●</span> 3f2a9c1-app.preview.localhost:8080   <span class="muted">build frontend · build backend</span>
<span class="muted">│</span>
<span class="grade">●</span> 8b71e04-app.preview.localhost:8080   <span class="muted">reuse frontend · fork backend from 3f2a9c1</span>
<span class="muted">│</span>
<span class="grade">●</span> c04d5ee-app.preview.localhost:8080   <span class="muted">reuse frontend · reuse backend</span></code></pre>
</div>

<ul class="project-links">
<li><a href="https://jmelahman.github.io/local-preview/"><i class="fas fa-book"></i>Docs</a></li>
<li><a href="https://github.com/jmelahman/local-preview"><i class="fab fa-github"></i>Source</a></li>
<li><a href="https://pypi.org/project/local-preview/"><i class="fab fa-python"></i>PyPI</a></li>
<li><a href="https://pkg.go.dev/github.com/jmelahman/local-preview"><i class="fab fa-golang"></i>Go</a></li>
</ul>
</article>

<article class="project">
<div class="project-head">
<h2 id="5-wild"><a href="https://5-wild.com/">5 Wild</a></h2>
<div class="chips"><span class="chip">TypeScript</span><span class="chip">Web</span><span class="chip">Android</span><span class="chip">GPL-3.0</span></div>
</div>

A word-guessing roguelike.

Guess the five letter word, but every guess you play is also a hand you score.
Letters score points based on how rare they are, the green and yellow feedback multiplies
them, and the relics you buy between rounds quietly rewrite the arithmetic underneath.
Bosses break a rule you were relying on every third round.
It is a word game that turns into a numbers game.

<div class="tiles" aria-hidden="true"><span class="tile tile-green">W</span><span class="tile tile-gray">I</span><span class="tile tile-yellow">L</span><span class="tile tile-gray">D</span><span class="tile tile-green">S</span></div>

<ul class="project-links">
<li><a href="https://5-wild.com/"><i class="fas fa-gamepad"></i>Play</a></li>
<li><a href="https://github.com/jmelahman/5-wild/releases/latest/download/5-wild.apk"><i class="fab fa-android"></i>APK</a></li>
<li><a href="https://github.com/jmelahman/5-wild"><i class="fab fa-github"></i>Source</a></li>
</ul>
</article>

<article class="project">
<div class="project-head">
<h2 id="connections"><a href="https://github.com/jmelahman/connections">connections</a></h2>
<div class="chips"><span class="chip">Go</span><span class="chip">TUI</span><span class="chip">MIT</span></div>
</div>

A command-line client for the NYT Connections game.

Play the daily puzzle in the terminal: select four words, submit, shuffle, and watch the
categories fill in as you solve them.

<div class="terminal dark">
<div class="terminal-bar"><i></i><i></i><i></i><span>connections</span></div>
<div class="conn"><span class="conn-green">Palindromes: Stats, Nun, Abba, Kayak</span><span class="conn-yellow">Slang For Head: Dome, Coconut, Crown, Skull</span><span class="conn-purple">First In A Comedy Duo: Abbott, Fry, Key, Laurel</span><span class="conn-blue">Police Procedurals: Kojak, Elementary, Bones, Monk</span></div>
</div>

<ul class="project-links">
<li><a href="https://github.com/jmelahman/connections"><i class="fab fa-github"></i>Source</a></li>
<li><a href="https://aur.archlinux.org/packages/connections"><i class="fab fa-linux"></i>AUR</a></li>
<li><a href="https://pypi.org/project/nyt-connections/"><i class="fab fa-python"></i>PyPI</a></li>
<li><a href="https://pkg.go.dev/github.com/jmelahman/connections"><i class="fab fa-golang"></i>Go</a></li>
<li><a href="https://github.com/jmelahman/connections/releases/latest"><i class="fas fa-download"></i>Releases</a></li>
</ul>
</article>

<article class="project">
<div class="project-head">
<h2 id="nature-sounds"><a href="https://github.com/jmelahman/nature-sounds">nature-sounds</a></h2>
<div class="chips"><span class="chip">Go</span><span class="chip">CLI</span><span class="chip">MIT</span></div>
</div>

A lightweight nature sounds player for the command-line.

It plays soundscapes from the National Park Service's A Symphony of Sounds collection,
recorded in places like Yellowstone and Rocky Mountain National Park, behind a UI inspired
by pianobar.

<div class="terminal">
<div class="terminal-bar"><i></i><i></i><i></i><span>nature-sounds</span></div>
<pre><code><span class="muted">$</span> nature-sounds
Welcome to nature-sounds (v0.1.0). Press ? for a list of commands.
<span class="grade">➤</span> "Old Faithful (Remixed)" by "NPS/Jennifer Jerrett and Peter Comley"
Available commands:
        p  pause/resume playback
        q  quit
        s  select new sound
<span class="grade">➤</span> "Stream Soundscape from the Black Canyon Trail" by "J. Job"
<span class="grade">⏸</span> "Soundscape - Lower Geyser Basin (Strong Wind)" by "NPS/Peter Comley"</code></pre>
</div>

<ul class="project-links">
<li><a href="https://github.com/jmelahman/nature-sounds"><i class="fab fa-github"></i>Source</a></li>
<li><a href="https://aur.archlinux.org/packages/nature-sounds"><i class="fab fa-linux"></i>AUR</a></li>
<li><a href="https://pypi.org/project/nature-sounds/"><i class="fab fa-python"></i>PyPI</a></li>
<li><a href="https://pkg.go.dev/github.com/jmelahman/nature-sounds"><i class="fab fa-golang"></i>Go</a></li>
<li><a href="https://github.com/jmelahman/nature-sounds/releases/latest"><i class="fas fa-download"></i>Releases</a></li>
</ul>
</article>

<article class="project">
<div class="project-head">
<h2 id="pkgbuilds"><a href="https://github.com/jmelahman/PKGBUILDs">PKGBUILDs</a></h2>
<div class="chips"><span class="chip">Shell</span><span class="chip">Arch Linux</span><span class="chip">MIT</span></div>
</div>

The packages I maintain for the Arch User Repository, in one repository.

Each package is a git subtree that is pushed to the AUR automatically on merge.
nvchecker watches upstream for new releases and opens a pull request overnight when one
lands, and CI builds every package in a clean container with shellcheck and pkglint
running over the whole tree.

<div class="terminal">
<div class="terminal-bar"><i></i><i></i><i></i><span>PKGBUILDs</span></div>
<pre class="wrap"><code><span class="muted">$</span> ls
<span class="nb">agent-deck</span>  <span class="nb">aligo</span>  <span class="nb">annas-mcp</span>  <span class="nb">aws-doctor</span>  <span class="nb">blipgloss</span>  <span class="nb">brev-cli</span>  <span class="nb">buildifier</span>  <span class="nb">buildozer</span>  <span class="nb">captain</span>  <span class="nb">cascadia</span>  <span class="nb">cert</span>  <span class="nb">cfnctl</span>  <span class="nb">check-symlinks</span>  <span class="nb">checkip</span>  <span class="nb">clive</span>  <span class="nb">cloud189</span>  <span class="nb">cls3</span>  <span class="nb">codeowners</span>  <span class="nb">comigo</span>  <span class="nb">connections</span>  <span class="nb">cycle-cli</span>  <span class="nb">docker-debug</span>  <span class="nb">dockerfilegraph</span>  <span class="nb">enpasscli</span>  <span class="nb">faas-cli</span>  <span class="nb">fil</span>  <span class="nb">gh-eco</span>  <span class="nb">gh-markdown-preview</span>  <span class="nb">gh-s</span>  <span class="nb">git-orchard</span>  <span class="nb">gitcs</span>  <span class="nb">go-carpet</span>  <span class="nb">go-global-update</span>  <span class="nb">go-grip</span>  <span class="muted nb">… 107 packages</span></code></pre>
</div>

<ul class="project-links">
<li><a href="https://github.com/jmelahman/PKGBUILDs"><i class="fab fa-github"></i>Source</a></li>
<li><a href="https://aur.archlinux.org/packages?K=Jamison&amp;SeB=m"><i class="fab fa-linux"></i>AUR</a></li>
</ul>
</article>
