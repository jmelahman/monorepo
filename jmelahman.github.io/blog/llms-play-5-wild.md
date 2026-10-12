---
og-title: "5 Wild Bench, a word-game roguelike LLM benchmark"
title-prefix: "Blog"
date: 2026-10-10
nav_blog: true
---

# 5 Wild Bench, a word-game roguelike LLM benchmark

[5 Wild](https://5-wild.com) is a free-to-play word-guessing roguelike I made combining [Wordle](https://www.nytimes.com/games/wordle) with [Balatro](https://www.playbalatro.com/)'s scoring and shop mechanics. I had fourteen LLMs play it, five runs each.

The idea came from [RuneScape Bench](https://maxbittker.github.io/runebench/) and Anthropic's [Project Vend](https://www.anthropic.com/research/project-vend-1). Both put a model inside an economy, where every action has a cost, a reward and an opportunity cost, and spending now has to be weighed against investing for later. I find that kind of benchmark interesting, and useful for what it shows about how a model behaves as an agent. 5 Wild has the same shape: gold spent to get through this round is gold that cannot go into the build that has to clear the later ones.

There is more than one way to play it, and the models found several. The stronger models were more consistent, adapted to each run, and played for the run's economy rather than the current round.

<svg class="results" style="width:100%;height:auto;margin:1em 0" viewBox="0 0 720 468" role="img" aria-labelledby="results-title" font-family="inherit">
<title id="results-title">Final score of each model, where higher is better: the median of five runs as a bar, and each run as a dot.</title>
<style>svg:has(.runs-shown:not(:checked)) .run{display:none}</style>
<foreignObject x="590" y="0" width="130" height="22"><label style="display:flex;justify-content:flex-end;align-items:center;gap:4px;font-size:11px;color:#8a8878;cursor:pointer"><input type="checkbox" class="runs-shown" checked style="margin:0;accent-color:#8c0000"> show each run</label></foreignObject>
<text x="0.0" y="14.0" font-size="12" fill="#8c0000" text-anchor="start" font-weight="700">Ascension 10, the hardest</text>
<line x1="34" x2="720" y1="180.0" y2="180.0" stroke="#8a8878" stroke-width="1"/>
<text x="28.0" y="184.0" font-size="11" fill="#8a8878" text-anchor="end">1K</text>
<line x1="34" x2="720" y1="142.5" y2="142.5" stroke="#e3e1cf" stroke-width="1"/>
<text x="28.0" y="146.5" font-size="11" fill="#8a8878" text-anchor="end">10K</text>
<line x1="34" x2="720" y1="105.0" y2="105.0" stroke="#e3e1cf" stroke-width="1"/>
<text x="28.0" y="109.0" font-size="11" fill="#8a8878" text-anchor="end">100K</text>
<line x1="34" x2="720" y1="67.5" y2="67.5" stroke="#e3e1cf" stroke-width="1"/>
<text x="28.0" y="71.5" font-size="11" fill="#8a8878" text-anchor="end">1M</text>
<line x1="34" x2="720" y1="30.0" y2="30.0" stroke="#e3e1cf" stroke-width="1"/>
<text x="28.0" y="34.0" font-size="11" fill="#8a8878" text-anchor="end">10M</text>
<g><title>Claude Opus 5.5: a median of 3.5M points, mean 3.1M, 3 of 5 runs won</title><rect x="49.0" y="47.1" width="46.2" height="132.9" fill="#d9d6c1"/></g>
<g class="run">
<a href="https://www.youtube.com/watch?v=iayM2a_q9eg"><circle cx="72.1" cy="36.9" r="5.5" fill="#8c0000" stroke="#fbfaef" stroke-width="2" paint-order="stroke"><title>Claude Opus 5.5, seed 5: won, 6,551,698 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=adrTWq4DkTw"><circle cx="85.1" cy="45.4" r="5.5" fill="#8c0000" stroke="#fbfaef" stroke-width="2" paint-order="stroke"><title>Claude Opus 5.5, seed 1: won, 3,873,095 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=vwdLLUmtw48"><circle cx="59.1" cy="47.1" r="5.5" fill="#8c0000" stroke="#fbfaef" stroke-width="2" paint-order="stroke"><title>Claude Opus 5.5, seed 2: won, 3,489,091 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=qTJvwRW1SEg"><circle cx="72.1" cy="61.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Claude Opus 5.5, seed 3: 23 of 24 rounds, 1,492,971 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=a2fVOav6bKg"><circle cx="72.1" cy="166.1" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Claude Opus 5.5, seed 4: 2 of 24 rounds, 2,348 points (watch)</title></circle></a>
</g>
<text x="72.1" y="196.0" font-size="11.5" fill="#2c2c2c" text-anchor="middle">Claude</text>
<text x="72.1" y="209.0" font-size="11.5" fill="#2c2c2c" text-anchor="middle">Opus 5.5</text>
<g><title>Claude Sonnet 5.5: a median of 621K points, mean 1.2M, 1 of 5 runs won</title><rect x="125.2" y="75.3" width="46.2" height="104.7" fill="#d9d6c1"/></g>
<g class="run">
<a href="https://www.youtube.com/watch?v=U8igROuMQtQ"><circle cx="148.3" cy="48.2" r="5.5" fill="#8c0000" stroke="#fbfaef" stroke-width="2" paint-order="stroke"><title>Claude Sonnet 5.5, seed 5: won, 3,264,282 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=p6WOduxnL5A"><circle cx="148.3" cy="64.1" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Claude Sonnet 5.5, seed 4: 22 of 24 rounds, 1,232,555 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=UH9l_1GaPm0"><circle cx="161.3" cy="75.3" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Claude Sonnet 5.5, seed 3: 20 of 24 rounds, 620,789 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=GrOcnkQdwDU"><circle cx="148.3" cy="81.4" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Claude Sonnet 5.5, seed 2: 17 of 24 rounds, 425,178 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=yrusditZPE4"><circle cx="161.3" cy="90.6" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Claude Sonnet 5.5, seed 1: 15 of 24 rounds, 241,878 points (watch)</title></circle></a>
</g>
<text x="148.3" y="196.0" font-size="11.5" fill="#2c2c2c" text-anchor="middle">Claude</text>
<text x="148.3" y="209.0" font-size="11.5" fill="#2c2c2c" text-anchor="middle">Sonnet 5.5</text>
<g><title>GPT-6 Sol: a median of 351K points, mean 1.0M, 1 of 5 runs won</title><rect x="201.4" y="84.5" width="46.2" height="95.5" fill="#d9d6c1"/></g>
<g class="run">
<a href="https://www.youtube.com/watch?v=ey_7Feq_gns"><circle cx="224.6" cy="43.4" r="5.5" fill="#8c0000" stroke="#fbfaef" stroke-width="2" paint-order="stroke"><title>GPT-6 Sol, seed 5: won, 4,388,081 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=y8eHdJXrx2E"><circle cx="224.6" cy="80.7" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GPT-6 Sol, seed 2: 17 of 24 rounds, 443,748 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=Jbg0F-wpENg"><circle cx="237.6" cy="84.5" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GPT-6 Sol, seed 1: 17 of 24 rounds, 351,490 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=aK1txCGgeyo"><circle cx="224.6" cy="119.7" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GPT-6 Sol, seed 3: 8 of 24 rounds, 40,532 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=kPTtJhUeqk8"><circle cx="224.6" cy="167.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GPT-6 Sol, seed 4: 2 of 24 rounds, 2,224 points (watch)</title></circle></a>
</g>
<text x="224.6" y="196.0" font-size="11.5" fill="#2c2c2c" text-anchor="middle">GPT-6</text>
<text x="224.6" y="209.0" font-size="11.5" fill="#2c2c2c" text-anchor="middle">Sol</text>
<g><title>MiMo-V2.6-Pro: a median of 275K points, mean 304K, 0 of 5 runs won</title><rect x="277.7" y="88.5" width="46.2" height="91.5" fill="#d9d6c1"/></g>
<g class="run">
<a href="https://www.youtube.com/watch?v=PVj7my88Uqs"><circle cx="300.8" cy="72.1" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>MiMo-V2.6-Pro, seed 1: 21 of 24 rounds, 751,730 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=vD9t4aaMHwk"><circle cx="313.8" cy="83.6" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>MiMo-V2.6-Pro, seed 4: 14 of 24 rounds, 372,866 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=pwt9r1NLz1k"><circle cx="300.8" cy="88.5" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>MiMo-V2.6-Pro, seed 5: 14 of 24 rounds, 275,474 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=ZdRVXAKdwT0"><circle cx="300.8" cy="105.5" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>MiMo-V2.6-Pro, seed 3: 11 of 24 rounds, 96,733 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=1vtdj-A3FOU"><circle cx="300.8" cy="128.2" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>MiMo-V2.6-Pro, seed 2: 5 of 24 rounds, 24,078 points (watch)</title></circle></a>
</g>
<text x="300.8" y="196.0" font-size="11.5" fill="#2c2c2c" text-anchor="middle">MiMo-V2.6-Pro</text>
<text x="300.8" y="209.0" font-size="11.5" fill="#2c2c2c" text-anchor="middle"></text>
<g><title>DeepSeek V4.1 Flash: a median of 262K points, mean 190K, 0 of 5 runs won</title><rect x="353.9" y="89.3" width="46.2" height="90.7" fill="#d9d6c1"/></g>
<g class="run">
<a href="https://www.youtube.com/watch?v=K5uVfhW6PdA"><circle cx="377.0" cy="84.6" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>DeepSeek V4.1 Flash, seed 2: 17 of 24 rounds, 350,065 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=0Lq3hE_nVW0"><circle cx="390.0" cy="88.4" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>DeepSeek V4.1 Flash, seed 3: 16 of 24 rounds, 277,886 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=9hojXwaQMQI"><circle cx="364.0" cy="89.3" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>DeepSeek V4.1 Flash, seed 5: 16 of 24 rounds, 262,140 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=yDI7jf9qUkE"><circle cx="377.0" cy="113.5" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>DeepSeek V4.1 Flash, seed 1: 11 of 24 rounds, 59,266 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=Z3mdoXoRwkA"><circle cx="377.0" cy="166.6" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>DeepSeek V4.1 Flash, seed 4: 2 of 24 rounds, 2,279 points (watch)</title></circle></a>
</g>
<text x="377.0" y="196.0" font-size="11.5" fill="#2c2c2c" text-anchor="middle">DeepSeek</text>
<text x="377.0" y="209.0" font-size="11.5" fill="#2c2c2c" text-anchor="middle">V4.1 Flash</text>
<g><title>GPT-6.1 Sol: a median of 164K points, mean 395K, 0 of 5 runs won</title><rect x="430.1" y="97.0" width="46.2" height="83.0" fill="#d9d6c1"/></g>
<g class="run">
<a href="https://www.youtube.com/watch?v=AR95WKHPWTo"><circle cx="453.2" cy="65.7" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GPT-6.1 Sol, seed 5: 18 of 24 rounds, 1,118,355 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=jD37Wuu29Lg"><circle cx="466.2" cy="74.8" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GPT-6.1 Sol, seed 3: 21 of 24 rounds, 637,062 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=erikjgW8lt4"><circle cx="453.2" cy="97.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GPT-6.1 Sol, seed 1: 11 of 24 rounds, 163,688 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=53FE_meAP1g"><circle cx="453.2" cy="114.6" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GPT-6.1 Sol, seed 2: 8 of 24 rounds, 55,385 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=nDLUAjcKKEs"><circle cx="453.2" cy="170.9" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GPT-6.1 Sol, seed 4: 2 of 24 rounds, 1,749 points (watch)</title></circle></a>
</g>
<text x="453.2" y="196.0" font-size="11.5" fill="#2c2c2c" text-anchor="middle">GPT-6.1</text>
<text x="453.2" y="209.0" font-size="11.5" fill="#2c2c2c" text-anchor="middle">Sol</text>
<g><title>DeepSeek V4 Pro: a median of 104K points, mean 151K, 0 of 5 runs won</title><rect x="506.3" y="104.3" width="46.2" height="75.7" fill="#d9d6c1"/></g>
<g class="run">
<a href="https://www.youtube.com/watch?v=RlobHTVNY_U"><circle cx="529.4" cy="83.9" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>DeepSeek V4 Pro, seed 5: 17 of 24 rounds, 364,607 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=rEKWM1S5Qmo"><circle cx="542.4" cy="94.2" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>DeepSeek V4 Pro, seed 4: 14 of 24 rounds, 193,611 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=g97u_BuKtfI"><circle cx="529.4" cy="104.3" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>DeepSeek V4 Pro, seed 1: 11 of 24 rounds, 104,477 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=Oql4Mgik6Ps"><circle cx="542.4" cy="107.3" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>DeepSeek V4 Pro, seed 2: 11 of 24 rounds, 86,925 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=W_zHf3ygGWs"><circle cx="529.4" cy="150.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>DeepSeek V4 Pro, seed 3: 3 of 24 rounds, 6,298 points (watch)</title></circle></a>
</g>
<text x="529.4" y="196.0" font-size="11.5" fill="#2c2c2c" text-anchor="middle">DeepSeek</text>
<text x="529.4" y="209.0" font-size="11.5" fill="#2c2c2c" text-anchor="middle">V4 Pro</text>
<g><title>GLM 5.3 Flash: a median of 95K points, mean 409K, 1 of 5 runs won</title><rect x="582.6" y="105.9" width="46.2" height="74.1" fill="#d9d6c1"/></g>
<g class="run">
<a href="https://www.youtube.com/watch?v=hBWIqTIDLE0"><circle cx="605.7" cy="58.5" r="5.5" fill="#8c0000" stroke="#fbfaef" stroke-width="2" paint-order="stroke"><title>GLM 5.3 Flash, seed 5: won, 1,734,821 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=U4_qGKCounw"><circle cx="605.7" cy="101.5" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GLM 5.3 Flash, seed 1: 11 of 24 rounds, 124,271 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=YRuqONkt800"><circle cx="618.7" cy="105.9" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GLM 5.3 Flash, seed 3: 8 of 24 rounds, 94,582 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=NuywloorlgE"><circle cx="592.7" cy="106.5" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GLM 5.3 Flash, seed 2: 10 of 24 rounds, 91,395 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=8ySYrb62vkE"><circle cx="605.7" cy="171.7" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GLM 5.3 Flash, seed 4: 2 of 24 rounds, 1,661 points (watch)</title></circle></a>
</g>
<text x="605.7" y="196.0" font-size="11.5" fill="#2c2c2c" text-anchor="middle">GLM</text>
<text x="605.7" y="209.0" font-size="11.5" fill="#2c2c2c" text-anchor="middle">5.3 Flash</text>
<g><title>Gemini 3.8 Flash: a median of 52K points, mean 246K, 0 of 5 runs won</title><rect x="658.8" y="115.6" width="46.2" height="64.4" fill="#d9d6c1"/></g>
<g class="run">
<a href="https://www.youtube.com/watch?v=jx_FDosJzD4"><circle cx="681.9" cy="70.6" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Gemini 3.8 Flash, seed 4: 20 of 24 rounds, 828,777 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=0R70-cRIg6g"><circle cx="681.9" cy="87.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Gemini 3.8 Flash, seed 1: 17 of 24 rounds, 302,122 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=pyWffPw-YWw"><circle cx="681.9" cy="115.6" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Gemini 3.8 Flash, seed 3: 8 of 24 rounds, 52,176 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=Xs2nELVIfV4"><circle cx="694.9" cy="126.3" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Gemini 3.8 Flash, seed 5: 8 of 24 rounds, 27,068 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=VWbtI_bBIJI"><circle cx="681.9" cy="129.9" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Gemini 3.8 Flash, seed 2: 5 of 24 rounds, 21,680 points (watch)</title></circle></a>
</g>
<text x="681.9" y="196.0" font-size="11.5" fill="#2c2c2c" text-anchor="middle">Gemini</text>
<text x="681.9" y="209.0" font-size="11.5" fill="#2c2c2c" text-anchor="middle">3.8 Flash</text>
<text x="0.0" y="245.0" font-size="12" fill="#8c0000" text-anchor="start" font-weight="700">Ascension 0, the easiest</text>
<line x1="34" x2="720" y1="411.0" y2="411.0" stroke="#8a8878" stroke-width="1"/>
<text x="28.0" y="415.0" font-size="11" fill="#8a8878" text-anchor="end">1K</text>
<line x1="34" x2="720" y1="373.5" y2="373.5" stroke="#e3e1cf" stroke-width="1"/>
<text x="28.0" y="377.5" font-size="11" fill="#8a8878" text-anchor="end">10K</text>
<line x1="34" x2="720" y1="336.0" y2="336.0" stroke="#e3e1cf" stroke-width="1"/>
<text x="28.0" y="340.0" font-size="11" fill="#8a8878" text-anchor="end">100K</text>
<line x1="34" x2="720" y1="298.5" y2="298.5" stroke="#e3e1cf" stroke-width="1"/>
<text x="28.0" y="302.5" font-size="11" fill="#8a8878" text-anchor="end">1M</text>
<line x1="34" x2="720" y1="261.0" y2="261.0" stroke="#e3e1cf" stroke-width="1"/>
<text x="28.0" y="265.0" font-size="11" fill="#8a8878" text-anchor="end">10M</text>
<g><title>Gemini 3.8 Flash: a median of 3.8M points, mean 3.8M, 5 of 5 runs won</title><rect x="49.0" y="276.7" width="68.0" height="134.3" fill="#d9d6c1"/></g>
<g class="run">
<a href="https://www.youtube.com/watch?v=R2jsTuhsbXU"><circle cx="83.0" cy="272.1" r="5.5" fill="#8c0000" stroke="#fbfaef" stroke-width="2" paint-order="stroke"><title>Gemini 3.8 Flash, seed 2: won, 5,069,690 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=49SKHzw9LDM"><circle cx="96.0" cy="275.1" r="5.5" fill="#8c0000" stroke="#fbfaef" stroke-width="2" paint-order="stroke"><title>Gemini 3.8 Flash, seed 1: won, 4,198,512 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=nJocTunBu2o"><circle cx="70.0" cy="276.7" r="5.5" fill="#8c0000" stroke="#fbfaef" stroke-width="2" paint-order="stroke"><title>Gemini 3.8 Flash, seed 5: won, 3,805,591 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=mOlavEqeURk"><circle cx="109.0" cy="279.6" r="5.5" fill="#8c0000" stroke="#fbfaef" stroke-width="2" paint-order="stroke"><title>Gemini 3.8 Flash, seed 4: won, 3,193,387 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=DuzftCWhnjY"><circle cx="57.0" cy="283.5" r="5.5" fill="#8c0000" stroke="#fbfaef" stroke-width="2" paint-order="stroke"><title>Gemini 3.8 Flash, seed 3: won, 2,514,385 points (watch)</title></circle></a>
</g>
<text x="83.0" y="427.0" font-size="11.5" fill="#2c2c2c" text-anchor="middle">Gemini</text>
<text x="83.0" y="440.0" font-size="11.5" fill="#2c2c2c" text-anchor="middle">3.8 Flash</text>
<g><title>Claude Sonnet 5.5: a median of 991K points, mean 1.2M, 1 of 5 runs won</title><rect x="147.0" y="298.6" width="68.0" height="112.4" fill="#d9d6c1"/></g>
<g class="run">
<a href="https://www.youtube.com/watch?v=tqtyQ6L5mvw"><circle cx="181.0" cy="284.6" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Claude Sonnet 5.5, seed 2: 23 of 24 rounds, 2,352,722 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=Zp8Y4W7u8hE"><circle cx="194.0" cy="289.1" r="5.5" fill="#8c0000" stroke="#fbfaef" stroke-width="2" paint-order="stroke"><title>Claude Sonnet 5.5, seed 5: won, 1,777,015 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=T5hgQKiSg1w"><circle cx="181.0" cy="298.6" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Claude Sonnet 5.5, seed 1: 22 of 24 rounds, 991,301 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=y2chT1UjSYs"><circle cx="194.0" cy="304.4" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Claude Sonnet 5.5, seed 3: 19 of 24 rounds, 697,070 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=RlzB8TtLrtc"><circle cx="181.0" cy="323.1" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Claude Sonnet 5.5, seed 4: 14 of 24 rounds, 220,820 points (watch)</title></circle></a>
</g>
<text x="181.0" y="427.0" font-size="11.5" fill="#2c2c2c" text-anchor="middle">Claude</text>
<text x="181.0" y="440.0" font-size="11.5" fill="#2c2c2c" text-anchor="middle">Sonnet 5.5</text>
<g><title>Step 5 Preview: a median of 237K points, mean 179K, 0 of 5 runs won</title><rect x="245.0" y="321.9" width="68.0" height="89.1" fill="#d9d6c1"/></g>
<g class="run">
<a href="https://www.youtube.com/watch?v=kAo7-11T-YA"><circle cx="279.0" cy="319.2" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Step 5 Preview, seed 2: 17 of 24 rounds, 280,571 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=E4CgtemAkD8"><circle cx="292.0" cy="321.6" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Step 5 Preview, seed 1: 14 of 24 rounds, 241,668 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=vy6FkmqC4rE"><circle cx="266.0" cy="321.9" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Step 5 Preview, seed 5: 15 of 24 rounds, 237,290 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=MGGPMgselgs"><circle cx="305.0" cy="331.2" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Step 5 Preview, seed 3: 15 of 24 rounds, 134,164 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=AUelOBa-WTI"><circle cx="279.0" cy="411.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Step 5 Preview, seed 4: 1 of 24 rounds, 990 points (watch)</title></circle></a>
</g>
<text x="279.0" y="427.0" font-size="11.5" fill="#2c2c2c" text-anchor="middle">Step</text>
<text x="279.0" y="440.0" font-size="11.5" fill="#2c2c2c" text-anchor="middle">5 Preview</text>
<g><title>GPT-6 Luna: a median of 186K points, mean 523K, 0 of 5 runs won</title><rect x="343.0" y="325.9" width="68.0" height="85.1" fill="#d9d6c1"/></g>
<g class="run">
<a href="https://www.youtube.com/watch?v=5zj1K5oiwkE"><circle cx="377.0" cy="290.3" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GPT-6 Luna, seed 3: 23 of 24 rounds, 1,652,069 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=tIKdDAuWXis"><circle cx="377.0" cy="306.9" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GPT-6 Luna, seed 5: 18 of 24 rounds, 598,395 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=2Y5m6Y5lT_k"><circle cx="377.0" cy="325.9" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GPT-6 Luna, seed 2: 13 of 24 rounds, 186,222 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=zzGGdhAlKUg"><circle cx="390.0" cy="326.9" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GPT-6 Luna, seed 1: 14 of 24 rounds, 174,437 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=C3l3Dze_Ols"><circle cx="377.0" cy="398.8" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GPT-6 Luna, seed 4: 2 of 24 rounds, 2,116 points (watch)</title></circle></a>
</g>
<text x="377.0" y="427.0" font-size="11.5" fill="#2c2c2c" text-anchor="middle">GPT-6</text>
<text x="377.0" y="440.0" font-size="11.5" fill="#2c2c2c" text-anchor="middle">Luna</text>
<g><title>Claude Haiku 5.5: a median of 97K points, mean 105K, 0 of 5 runs won</title><rect x="441.0" y="336.4" width="68.0" height="74.6" fill="#d9d6c1"/></g>
<g class="run">
<a href="https://www.youtube.com/watch?v=-nUz-qL6yNY"><circle cx="475.0" cy="322.3" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Claude Haiku 5.5, seed 1: 14 of 24 rounds, 231,696 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=Ytxe7EFLmwQ"><circle cx="488.0" cy="325.3" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Claude Haiku 5.5, seed 3: 14 of 24 rounds, 193,064 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=G2O1qn5KZr8"><circle cx="475.0" cy="336.4" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Claude Haiku 5.5, seed 2: 11 of 24 rounds, 97,385 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=64A30oLI-ZM"><circle cx="475.0" cy="403.7" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Claude Haiku 5.5, seed 5: 2 of 24 rounds, 1,568 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=bH6YjQL5pG4"><circle cx="488.0" cy="409.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Claude Haiku 5.5, seed 4: 2 of 24 rounds, 1,130 points (watch)</title></circle></a>
</g>
<text x="475.0" y="427.0" font-size="11.5" fill="#2c2c2c" text-anchor="middle">Claude</text>
<text x="475.0" y="440.0" font-size="11.5" fill="#2c2c2c" text-anchor="middle">Haiku 5.5</text>
<g><title>Nemotron 3 Ultra: a median of 5K points, mean 316K, 0 of 5 runs won</title><rect x="539.0" y="385.3" width="68.0" height="25.7" fill="#d9d6c1"/></g>
<g class="run">
<a href="https://www.youtube.com/watch?v=WofXv42yKPU"><circle cx="573.0" cy="292.3" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Nemotron 3 Ultra, seed 2: 23 of 24 rounds, 1,460,912 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=fBCMfZwrqaI"><circle cx="573.0" cy="333.9" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Nemotron 3 Ultra, seed 3: 11 of 24 rounds, 113,743 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=etKycxsho6E"><circle cx="573.0" cy="385.3" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Nemotron 3 Ultra, seed 5: 4 of 24 rounds, 4,851 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=3Pmo7xsy2s0"><circle cx="573.0" cy="402.1" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Nemotron 3 Ultra, seed 4: 2 of 24 rounds, 1,732 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=m7tC_lDiXxQ"><circle cx="586.0" cy="407.6" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Nemotron 3 Ultra, seed 1: 2 of 24 rounds, 1,232 points (watch)</title></circle></a>
</g>
<text x="573.0" y="427.0" font-size="11.5" fill="#2c2c2c" text-anchor="middle">Nemotron</text>
<text x="573.0" y="440.0" font-size="11.5" fill="#2c2c2c" text-anchor="middle">3 Ultra</text>
<g><title>MiniMax M3: a median of 2K points, mean 66K, 0 of 5 runs won</title><rect x="637.0" y="399.1" width="68.0" height="11.9" fill="#d9d6c1"/></g>
<g class="run">
<circle cx="671.0" cy="321.5" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>MiniMax M3, seed 5: 15 of 24 rounds, 243,319 points</title></circle>
<circle cx="671.0" cy="338.3" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>MiniMax M3, seed 2: 11 of 24 rounds, 86,634 points</title></circle>
<circle cx="671.0" cy="399.1" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>MiniMax M3, seed 4: 2 of 24 rounds, 2,079 points</title></circle>
<circle cx="684.0" cy="411.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>MiniMax M3, seed 1: 0 of 24 rounds, 0 points</title></circle>
<circle cx="658.0" cy="411.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>MiniMax M3, seed 3: 0 of 24 rounds, 0 points</title></circle>
</g>
<text x="671.0" y="427.0" font-size="11.5" fill="#2c2c2c" text-anchor="middle">MiniMax</text>
<text x="671.0" y="440.0" font-size="11.5" fill="#2c2c2c" text-anchor="middle">M3</text>
<text x="34" y="462" font-size="11" fill="#8a8878">↑ higher is better · final score, log scale · median of five runs<tspan class="run"> · ● a run that won   ○ a run that lost</tspan></text>
</svg>

Each dot is one run and links to a video of it.

A run is 24 rounds; clearing all 24 wins. Nine models played the hardest difficulty, which is meant for a skilled Wordle player to win about 40% of the time. Seven played the easiest, which an average Wordle player should win most of the time. Sonnet and Gemini 3.8 Flash played both.

<figure style="margin:1em 0">
<iframe src="https://www.youtube-nocookie.com/embed/iayM2a_q9eg?start=100" title="Claude Opus 5.5 winning a run of 5 Wild at ascension 10" loading="lazy" allow="accelerometer; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" allowfullscreen style="width:100%;aspect-ratio:16/9;border:0"></iframe>
</figure>

## What stood out

* **Each model had its own style.** Haiku opened with CRANE 23 times. Luna opened with high-scoring words like JAZZY. Sonnet won one base-game run by [playing TAXIS 26 times](#the-strategies-they-found). Opus won three times with three different builds. Gemini opened with CRANE in 100 of its 120 rounds; Sol never settled on an opener and spread its first guesses over CRANE, SLATE, IRATE, STARE and a dozen more.
* **The stronger models were consistent.** Opus cleared 24, 24, 24, 23 and 2 rounds; Sonnet, on the same difficulty, 15, 17, 20, 22 and 24. Luna ranged from 2 to 23 and Haiku from 2 to 14. Gemini won all five of its runs on the easiest difficulty, with scores between 2.5 and 5.1 million. Sol was the exception among the larger models: 17, 17, 8, 2 and one win.
* **They adapted.** Ascension 10 bans repeated words and requires solving every round. Sonnet's median was 20 rounds there and 22 on the base game. Gemini did not carry over: it won all five runs on the base game and none at ascension 10, where its median was 8 rounds.
* **They played the economy, not the round.** Opus and Sonnet found the word about equally fast, in 3.6 and 3.9 guesses. Five of their six losses came with the word solved and the score short. Opus earned 8.9 gold a round to Sonnet's 6.7 and rerolled the shop 18 times to Sonnet's once. Gemini rerolled 31 times across its five wins. The cheaper models at ascension 10 barely touched any of it: Opus earned 125 gold in interest over its five runs, GPT-6.1 Sol 10, DeepSeek V4.1 Flash 7 and GLM 5.3 Flash 3.
* **Finding the word and scoring are separate skills.** DeepSeek V4.1 Flash found the word in 4.2 guesses a round and still lost four of its five runs with the word solved and the score short. MiMo-V2.6-Pro and DeepSeek V4 Pro solved every round they played and lost all ten runs the same way. DeepSeek V4 Pro was the closest to Opus on paper, at 3.7 guesses and 8.2 gold a round, and never rerolled the shop once. GLM 5.3 Flash and GPT-6.1 Sol went the other way: three losses each with the word unsolved, several of them with two to four times the round's target already banked. Step 5 Preview played for points on the base game, with AMAZE 20 times and JAZZY 19, and solved 35 of its 67 rounds. All five of its runs ended just short of the target, between 86% and 98% of it.

## The game

From Wordle it takes the board: six guesses at a five-letter word, with green, yellow and gray tiles after each. From Balatro it takes everything around the board. A guess is a hand: its letters are worth points, rare ones far more than common ones, a green tile multiplies them and a gray one does nothing, and the round has a target score to reach. Between rounds a shop sells relics, which are Balatro's jokers: each one changes what a guess is worth, and a run is won by finding a handful that work together. A run is eight stages of three rounds, the third of each against a boss that changes a rule.

The two halves pull against each other, which is what makes it a decent test of a model. A guess that narrows the answer is made of cheap, common letters and scores almost nothing. A guess that scores well tells you very little. On the base game, solving the word is not even the goal; clearing the round's target is, and the target grows much faster than a good vocabulary does.

Ascensions stack, Slay the Spire style: level ten plays every rule below it. The ones that matter here:

* Every guess must reuse the letters you have found, in the places you found them
* No word twice in the whole run
* Four relic slots rather than five, and every round pays less
* Reaching the target is not enough; you have to solve the word

## The setup

The game's engine is a pure function of a seed and a list of actions, so a benchmark was mostly a matter of putting a door on it. That door is an [MCP](https://modelcontextprotocol.io/) server with four tools:

* `rules`: the whole briefing, once
* `new_run`: deal the next seed
* `observe`: read the board
* `act`: one command (`guess crane`, `buy 2`, `collect`, `next`)

The model gets those four tools and nothing else. No shell, no files, no web, no subagents, so no solver and no peeking at the word list. Nothing carries over from one seed to the next.

Every model plays the same five seeds. A seed fixes the answers and the bosses, so everyone faces the same words in the same order, on either ascension. Shops start identical and drift apart as soon as two players buy differently.

Nothing in it is specific to one vendor. Any harness that can connect an MCP server can play, and every run is saved as its list of moves, which is where every chart and video below comes from.

## Caveats

Five seeds is a small sample. I would not rank two models whose medians are within a few rounds of each other, and "three of five" is a long way from a win rate.

The Claude models ran through Claude Code and the others through [OpenCode](https://opencode.ai/), so the harness differs as well as the model. Luna, Haiku, Step 5 Preview and MiniMax M3 ran on OpenCode's Go plan, and the rest through [OpenRouter](https://openrouter.ai/). DeepSeek V4 Pro used both: the Go plan's limit ran out with four of its runs 7 to 10 rounds in, and they were finished through OpenRouter from the saved moves, as was MiniMax M3's last run. Both were held to the same four tools.

Thinking effort was not held equal either. I did not set it, so each model ran at whatever its harness defaulted to: Opus at medium and Sonnet at high in Claude Code, and the OpenCode models at OpenCode's defaults. Opus beat Sonnet with less of it.

Only Sonnet and Gemini 3.8 Flash played both ascensions, so there is no Opus on the base game and no Luna at ascension 10 above.

The benchmark lives in the game's repository under `tools/bench/`, and every result here was played on v0.10.4.

## The strategies they found

A few of the play styles in more detail.

**Sonnet burned half the alphabet and played TAXIS 26 times.** On seed 5 of the base game it bought Pyromaniac, which breaks a random letter out of the alphabet every round, and Scorched Earth, which pays for every letter broken. By round 20, twelve letters were gone, and it had spent its upgrades on T, A, I and S. TAXIS is those four upgraded letters around an X, which is worth eight times a common letter. From there it stopped solving:

```
round 21   taxis taxis taxis taxis taxis taxis
round 22   taxis taxis taxis taxis taxis taxis
round 23   taxis taxis taxis taxis taxis taxis
round 24   taxis taxis taxis          (won)
```

The ascensions take this away: no word twice in a round from ascension 2, no word twice in a run from ascension 9, and at ascension 10 the target is not enough without the solve.

**Luna opened with JAZZY.** A normal opener is CRANE or SLATE: common letters, lots of information, almost no points. Luna's most-played words were EERIE (15 times), CRANE (13), JAZZY (11) and FUZZY (6), with QUEER and EJECT behind them. Every one of those but CRANE repeats a letter, and in three of its five runs Luna was holding Twins, which multiplies any guess with a doubled letter by 2.5. JAZZY is also about the most expensive five letters in the game, 33 points against 5 for AROSE. It traded the information of a first guess for points.

**Opus and Sonnet at ascension 10 had to do it the hard way.** With no word allowed twice in a run, neither could lean on a favourite. Opus opened with SLATE once in every run and CRANE once in every run, and then had to find an opener for each of the other rounds; across five runs, 305 of its 358 guesses were different words. Opus solved every round it cleared and Sonnet all but one, because they had to.

On the base game, the most-played words show how far a favourite can carry a run:

<svg class="results" style="width:100%;height:auto;margin:1em 0" viewBox="0 0 720 502" role="img" aria-labelledby="words-title" font-family="inherit">
<title id="words-title">The five most-played words for each model across its five runs.</title>
<text x="265.5" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">most played</text>
<text x="366.5" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">2nd</text>
<text x="467.5" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">3rd</text>
<text x="568.5" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">4th</text>
<text x="669.5" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">5th</text>
<text x="0.0" y="28.0" font-size="12" fill="#8c0000" text-anchor="start" font-weight="700">Ascension 10, the hardest</text>
<text x="0.0" y="51.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Opus 5.5</text>
<rect x="216.0" y="36" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Claude Opus 5.5: CRANE played 5 times in 99 rounds</title></rect>
<text x="265.5" y="51.0" font-size="11" fill="#2c2c2c" text-anchor="middle">CRANE 5</text>
<rect x="317.0" y="36" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Claude Opus 5.5: SLATE played 5 times in 99 rounds</title></rect>
<text x="366.5" y="51.0" font-size="11" fill="#2c2c2c" text-anchor="middle">SLATE 5</text>
<rect x="418.0" y="36" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Claude Opus 5.5: ARISE played 4 times in 99 rounds</title></rect>
<text x="467.5" y="51.0" font-size="11" fill="#2c2c2c" text-anchor="middle">ARISE 4</text>
<rect x="519.0" y="36" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Claude Opus 5.5: RAISE played 4 times in 99 rounds</title></rect>
<text x="568.5" y="51.0" font-size="11" fill="#2c2c2c" text-anchor="middle">RAISE 4</text>
<rect x="620.0" y="36" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Claude Opus 5.5: IRATE played 3 times in 99 rounds</title></rect>
<text x="669.5" y="51.0" font-size="11" fill="#2c2c2c" text-anchor="middle">IRATE 3</text>
<text x="0.0" y="77.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Sonnet 5.5</text>
<rect x="216.0" y="62" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Claude Sonnet 5.5: AROSE played 5 times in 102 rounds</title></rect>
<text x="265.5" y="77.0" font-size="11" fill="#2c2c2c" text-anchor="middle">AROSE 5</text>
<rect x="317.0" y="62" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Claude Sonnet 5.5: RAISE played 5 times in 102 rounds</title></rect>
<text x="366.5" y="77.0" font-size="11" fill="#2c2c2c" text-anchor="middle">RAISE 5</text>
<rect x="418.0" y="62" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Claude Sonnet 5.5: STARE played 5 times in 102 rounds</title></rect>
<text x="467.5" y="77.0" font-size="11" fill="#2c2c2c" text-anchor="middle">STARE 5</text>
<rect x="519.0" y="62" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Claude Sonnet 5.5: ALONE played 4 times in 102 rounds</title></rect>
<text x="568.5" y="77.0" font-size="11" fill="#2c2c2c" text-anchor="middle">ALONE 4</text>
<rect x="620.0" y="62" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Claude Sonnet 5.5: CRANE played 4 times in 102 rounds</title></rect>
<text x="669.5" y="77.0" font-size="11" fill="#2c2c2c" text-anchor="middle">CRANE 4</text>
<text x="0.0" y="103.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6 Sol</text>
<rect x="216.0" y="88" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>GPT-6 Sol: CRANE played 5 times in 72 rounds</title></rect>
<text x="265.5" y="103.0" font-size="11" fill="#2c2c2c" text-anchor="middle">CRANE 5</text>
<rect x="317.0" y="88" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>GPT-6 Sol: SLATE played 5 times in 72 rounds</title></rect>
<text x="366.5" y="103.0" font-size="11" fill="#2c2c2c" text-anchor="middle">SLATE 5</text>
<rect x="418.0" y="88" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>GPT-6 Sol: IRATE played 4 times in 72 rounds</title></rect>
<text x="467.5" y="103.0" font-size="11" fill="#2c2c2c" text-anchor="middle">IRATE 4</text>
<rect x="519.0" y="88" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>GPT-6 Sol: STARE played 4 times in 72 rounds</title></rect>
<text x="568.5" y="103.0" font-size="11" fill="#2c2c2c" text-anchor="middle">STARE 4</text>
<rect x="620.0" y="88" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>GPT-6 Sol: ALERT played 3 times in 72 rounds</title></rect>
<text x="669.5" y="103.0" font-size="11" fill="#2c2c2c" text-anchor="middle">ALERT 3</text>
<text x="0.0" y="129.0" font-size="13" fill="#2c2c2c" text-anchor="start">MiMo-V2.6-Pro</text>
<rect x="216.0" y="114" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>MiMo-V2.6-Pro: ARISE played 4 times in 70 rounds</title></rect>
<text x="265.5" y="129.0" font-size="11" fill="#2c2c2c" text-anchor="middle">ARISE 4</text>
<rect x="317.0" y="114" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>MiMo-V2.6-Pro: SLATE played 3 times in 70 rounds</title></rect>
<text x="366.5" y="129.0" font-size="11" fill="#2c2c2c" text-anchor="middle">SLATE 3</text>
<rect x="418.0" y="114" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>MiMo-V2.6-Pro: STARE played 3 times in 70 rounds</title></rect>
<text x="467.5" y="129.0" font-size="11" fill="#2c2c2c" text-anchor="middle">STARE 3</text>
<rect x="519.0" y="114" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>MiMo-V2.6-Pro: AZURE played 2 times in 70 rounds</title></rect>
<text x="568.5" y="129.0" font-size="11" fill="#2c2c2c" text-anchor="middle">AZURE 2</text>
<rect x="620.0" y="114" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>MiMo-V2.6-Pro: BEAST played 2 times in 70 rounds</title></rect>
<text x="669.5" y="129.0" font-size="11" fill="#2c2c2c" text-anchor="middle">BEAST 2</text>
<text x="0.0" y="155.0" font-size="13" fill="#2c2c2c" text-anchor="start">DeepSeek V4.1 Flash</text>
<rect x="216.0" y="140" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>DeepSeek V4.1 Flash: CRANE played 5 times in 67 rounds</title></rect>
<text x="265.5" y="155.0" font-size="11" fill="#2c2c2c" text-anchor="middle">CRANE 5</text>
<rect x="317.0" y="140" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>DeepSeek V4.1 Flash: CHIME played 3 times in 67 rounds</title></rect>
<text x="366.5" y="155.0" font-size="11" fill="#2c2c2c" text-anchor="middle">CHIME 3</text>
<rect x="418.0" y="140" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>DeepSeek V4.1 Flash: QUICK played 3 times in 67 rounds</title></rect>
<text x="467.5" y="155.0" font-size="11" fill="#2c2c2c" text-anchor="middle">QUICK 3</text>
<rect x="519.0" y="140" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>DeepSeek V4.1 Flash: AUDIO played 2 times in 67 rounds</title></rect>
<text x="568.5" y="155.0" font-size="11" fill="#2c2c2c" text-anchor="middle">AUDIO 2</text>
<rect x="620.0" y="140" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>DeepSeek V4.1 Flash: BAKER played 2 times in 67 rounds</title></rect>
<text x="669.5" y="155.0" font-size="11" fill="#2c2c2c" text-anchor="middle">BAKER 2</text>
<text x="0.0" y="181.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6.1 Sol</text>
<rect x="216.0" y="166" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>GPT-6.1 Sol: CRANE played 5 times in 65 rounds</title></rect>
<text x="265.5" y="181.0" font-size="11" fill="#2c2c2c" text-anchor="middle">CRANE 5</text>
<rect x="317.0" y="166" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>GPT-6.1 Sol: STARE played 5 times in 65 rounds</title></rect>
<text x="366.5" y="181.0" font-size="11" fill="#2c2c2c" text-anchor="middle">STARE 5</text>
<rect x="418.0" y="166" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>GPT-6.1 Sol: TRACE played 5 times in 65 rounds</title></rect>
<text x="467.5" y="181.0" font-size="11" fill="#2c2c2c" text-anchor="middle">TRACE 5</text>
<rect x="519.0" y="166" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>GPT-6.1 Sol: SLATE played 4 times in 65 rounds</title></rect>
<text x="568.5" y="181.0" font-size="11" fill="#2c2c2c" text-anchor="middle">SLATE 4</text>
<rect x="620.0" y="166" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>GPT-6.1 Sol: SNARE played 4 times in 65 rounds</title></rect>
<text x="669.5" y="181.0" font-size="11" fill="#2c2c2c" text-anchor="middle">SNARE 4</text>
<text x="0.0" y="207.0" font-size="13" fill="#2c2c2c" text-anchor="start">DeepSeek V4 Pro</text>
<rect x="216.0" y="192" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>DeepSeek V4 Pro: CRANE played 5 times in 61 rounds</title></rect>
<text x="265.5" y="207.0" font-size="11" fill="#2c2c2c" text-anchor="middle">CRANE 5</text>
<rect x="317.0" y="192" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>DeepSeek V4 Pro: STARE played 5 times in 61 rounds</title></rect>
<text x="366.5" y="207.0" font-size="11" fill="#2c2c2c" text-anchor="middle">STARE 5</text>
<rect x="418.0" y="192" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>DeepSeek V4 Pro: SLATE played 4 times in 61 rounds</title></rect>
<text x="467.5" y="207.0" font-size="11" fill="#2c2c2c" text-anchor="middle">SLATE 4</text>
<rect x="519.0" y="192" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>DeepSeek V4 Pro: AROSE played 3 times in 61 rounds</title></rect>
<text x="568.5" y="207.0" font-size="11" fill="#2c2c2c" text-anchor="middle">AROSE 3</text>
<rect x="620.0" y="192" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>DeepSeek V4 Pro: HOUSE played 3 times in 61 rounds</title></rect>
<text x="669.5" y="207.0" font-size="11" fill="#2c2c2c" text-anchor="middle">HOUSE 3</text>
<text x="0.0" y="233.0" font-size="13" fill="#2c2c2c" text-anchor="start">GLM 5.3 Flash</text>
<rect x="216.0" y="218" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>GLM 5.3 Flash: QUACK played 4 times in 59 rounds</title></rect>
<text x="265.5" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">QUACK 4</text>
<rect x="317.0" y="218" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>GLM 5.3 Flash: QUICK played 4 times in 59 rounds</title></rect>
<text x="366.5" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">QUICK 4</text>
<rect x="418.0" y="218" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>GLM 5.3 Flash: CRANE played 3 times in 59 rounds</title></rect>
<text x="467.5" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">CRANE 3</text>
<rect x="519.0" y="218" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>GLM 5.3 Flash: ADIEU played 2 times in 59 rounds</title></rect>
<text x="568.5" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">ADIEU 2</text>
<rect x="620.0" y="218" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>GLM 5.3 Flash: BEACH played 2 times in 59 rounds</title></rect>
<text x="669.5" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">BEACH 2</text>
<text x="0.0" y="259.0" font-size="13" fill="#2c2c2c" text-anchor="start">Gemini 3.8 Flash</text>
<rect x="216.0" y="244" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Gemini 3.8 Flash: CRANE played 5 times in 63 rounds</title></rect>
<text x="265.5" y="259.0" font-size="11" fill="#2c2c2c" text-anchor="middle">CRANE 5</text>
<rect x="317.0" y="244" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Gemini 3.8 Flash: STARE played 5 times in 63 rounds</title></rect>
<text x="366.5" y="259.0" font-size="11" fill="#2c2c2c" text-anchor="middle">STARE 5</text>
<rect x="418.0" y="244" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Gemini 3.8 Flash: AUDIO played 4 times in 63 rounds</title></rect>
<text x="467.5" y="259.0" font-size="11" fill="#2c2c2c" text-anchor="middle">AUDIO 4</text>
<rect x="519.0" y="244" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Gemini 3.8 Flash: SLATE played 4 times in 63 rounds</title></rect>
<text x="568.5" y="259.0" font-size="11" fill="#2c2c2c" text-anchor="middle">SLATE 4</text>
<rect x="620.0" y="244" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Gemini 3.8 Flash: TRACE played 3 times in 63 rounds</title></rect>
<text x="669.5" y="259.0" font-size="11" fill="#2c2c2c" text-anchor="middle">TRACE 3</text>
<text x="0.0" y="290.0" font-size="12" fill="#8c0000" text-anchor="start" font-weight="700">Ascension 0, the easiest</text>
<text x="0.0" y="313.0" font-size="13" fill="#2c2c2c" text-anchor="start">Gemini 3.8 Flash</text>
<rect x="216.0" y="298" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.50"><title>Gemini 3.8 Flash: CRANE played 100 times in 120 rounds</title></rect>
<text x="265.5" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">CRANE 100</text>
<rect x="317.0" y="298" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.08"><title>Gemini 3.8 Flash: SOUTH played 16 times in 120 rounds</title></rect>
<text x="366.5" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">SOUTH 16</text>
<rect x="418.0" y="298" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.07"><title>Gemini 3.8 Flash: AUDIO played 14 times in 120 rounds</title></rect>
<text x="467.5" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">AUDIO 14</text>
<rect x="519.0" y="298" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Gemini 3.8 Flash: CRATE played 6 times in 120 rounds</title></rect>
<text x="568.5" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">CRATE 6</text>
<rect x="620.0" y="298" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Gemini 3.8 Flash: ABOUT played 4 times in 120 rounds</title></rect>
<text x="669.5" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">ABOUT 4</text>
<text x="0.0" y="339.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Sonnet 5.5</text>
<rect x="216.0" y="324" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.22"><title>Claude Sonnet 5.5: CRANE played 39 times in 106 rounds</title></rect>
<text x="265.5" y="339.0" font-size="11" fill="#2c2c2c" text-anchor="middle">CRANE 39</text>
<rect x="317.0" y="324" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.15"><title>Claude Sonnet 5.5: TAXIS played 26 times in 106 rounds</title></rect>
<text x="366.5" y="339.0" font-size="11" fill="#2c2c2c" text-anchor="middle">TAXIS 26</text>
<rect x="418.0" y="324" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.07"><title>Claude Sonnet 5.5: AROSE played 12 times in 106 rounds</title></rect>
<text x="467.5" y="339.0" font-size="11" fill="#2c2c2c" text-anchor="middle">AROSE 12</text>
<rect x="519.0" y="324" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Claude Sonnet 5.5: STEAL played 7 times in 106 rounds</title></rect>
<text x="568.5" y="339.0" font-size="11" fill="#2c2c2c" text-anchor="middle">STEAL 7</text>
<rect x="620.0" y="324" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Claude Sonnet 5.5: MOIST played 6 times in 106 rounds</title></rect>
<text x="669.5" y="339.0" font-size="11" fill="#2c2c2c" text-anchor="middle">MOIST 6</text>
<text x="0.0" y="365.0" font-size="13" fill="#2c2c2c" text-anchor="start">Step 5 Preview</text>
<rect x="216.0" y="350" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.18"><title>Step 5 Preview: AMAZE played 20 times in 67 rounds</title></rect>
<text x="265.5" y="365.0" font-size="11" fill="#2c2c2c" text-anchor="middle">AMAZE 20</text>
<rect x="317.0" y="350" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.17"><title>Step 5 Preview: JAZZY played 19 times in 67 rounds</title></rect>
<text x="366.5" y="365.0" font-size="11" fill="#2c2c2c" text-anchor="middle">JAZZY 19</text>
<rect x="418.0" y="350" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.10"><title>Step 5 Preview: EQUIP played 11 times in 67 rounds</title></rect>
<text x="467.5" y="365.0" font-size="11" fill="#2c2c2c" text-anchor="middle">EQUIP 11</text>
<rect x="519.0" y="350" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.07"><title>Step 5 Preview: QUICK played 8 times in 67 rounds</title></rect>
<text x="568.5" y="365.0" font-size="11" fill="#2c2c2c" text-anchor="middle">QUICK 8</text>
<rect x="620.0" y="350" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Step 5 Preview: EPOXY played 7 times in 67 rounds</title></rect>
<text x="669.5" y="365.0" font-size="11" fill="#2c2c2c" text-anchor="middle">EPOXY 7</text>
<text x="0.0" y="391.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6 Luna</text>
<rect x="216.0" y="376" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.12"><title>GPT-6 Luna: EERIE played 15 times in 75 rounds</title></rect>
<text x="265.5" y="391.0" font-size="11" fill="#2c2c2c" text-anchor="middle">EERIE 15</text>
<rect x="317.0" y="376" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.10"><title>GPT-6 Luna: CRANE played 13 times in 75 rounds</title></rect>
<text x="366.5" y="391.0" font-size="11" fill="#2c2c2c" text-anchor="middle">CRANE 13</text>
<rect x="418.0" y="376" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.09"><title>GPT-6 Luna: JAZZY played 11 times in 75 rounds</title></rect>
<text x="467.5" y="391.0" font-size="11" fill="#2c2c2c" text-anchor="middle">JAZZY 11</text>
<rect x="519.0" y="376" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>GPT-6 Luna: FUZZY played 6 times in 75 rounds</title></rect>
<text x="568.5" y="391.0" font-size="11" fill="#2c2c2c" text-anchor="middle">FUZZY 6</text>
<rect x="620.0" y="376" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>GPT-6 Luna: ATTIC played 4 times in 75 rounds</title></rect>
<text x="669.5" y="391.0" font-size="11" fill="#2c2c2c" text-anchor="middle">ATTIC 4</text>
<text x="0.0" y="417.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Haiku 5.5</text>
<rect x="216.0" y="402" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.29"><title>Claude Haiku 5.5: CRANE played 23 times in 48 rounds</title></rect>
<text x="265.5" y="417.0" font-size="11" fill="#2c2c2c" text-anchor="middle">CRANE 23</text>
<rect x="317.0" y="402" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Claude Haiku 5.5: JOINT played 4 times in 48 rounds</title></rect>
<text x="366.5" y="417.0" font-size="11" fill="#2c2c2c" text-anchor="middle">JOINT 4</text>
<rect x="418.0" y="402" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Claude Haiku 5.5: RAISE played 4 times in 48 rounds</title></rect>
<text x="467.5" y="417.0" font-size="11" fill="#2c2c2c" text-anchor="middle">RAISE 4</text>
<rect x="519.0" y="402" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Claude Haiku 5.5: AUDIO played 2 times in 48 rounds</title></rect>
<text x="568.5" y="417.0" font-size="11" fill="#2c2c2c" text-anchor="middle">AUDIO 2</text>
<rect x="620.0" y="402" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Claude Haiku 5.5: BRIEF played 2 times in 48 rounds</title></rect>
<text x="669.5" y="417.0" font-size="11" fill="#2c2c2c" text-anchor="middle">BRIEF 2</text>
<text x="0.0" y="443.0" font-size="13" fill="#2c2c2c" text-anchor="start">Nemotron 3 Ultra</text>
<rect x="216.0" y="428" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.11"><title>Nemotron 3 Ultra: CRANE played 9 times in 47 rounds</title></rect>
<text x="265.5" y="443.0" font-size="11" fill="#2c2c2c" text-anchor="middle">CRANE 9</text>
<rect x="317.0" y="428" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Nemotron 3 Ultra: QUICK played 4 times in 47 rounds</title></rect>
<text x="366.5" y="443.0" font-size="11" fill="#2c2c2c" text-anchor="middle">QUICK 4</text>
<rect x="418.0" y="428" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Nemotron 3 Ultra: AUDIO played 3 times in 47 rounds</title></rect>
<text x="467.5" y="443.0" font-size="11" fill="#2c2c2c" text-anchor="middle">AUDIO 3</text>
<rect x="519.0" y="428" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Nemotron 3 Ultra: JAZZY played 3 times in 47 rounds</title></rect>
<text x="568.5" y="443.0" font-size="11" fill="#2c2c2c" text-anchor="middle">JAZZY 3</text>
<rect x="620.0" y="428" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Nemotron 3 Ultra: QUART played 3 times in 47 rounds</title></rect>
<text x="669.5" y="443.0" font-size="11" fill="#2c2c2c" text-anchor="middle">QUART 3</text>
<text x="0.0" y="469.0" font-size="13" fill="#2c2c2c" text-anchor="start">MiniMax M3</text>
<rect x="216.0" y="454" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.20"><title>MiniMax M3: ADIEU played 11 times in 33 rounds</title></rect>
<text x="265.5" y="469.0" font-size="11" fill="#2c2c2c" text-anchor="middle">ADIEU 11</text>
<rect x="317.0" y="454" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.13"><title>MiniMax M3: CRANE played 7 times in 33 rounds</title></rect>
<text x="366.5" y="469.0" font-size="11" fill="#2c2c2c" text-anchor="middle">CRANE 7</text>
<rect x="418.0" y="454" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>MiniMax M3: ABOUT played 2 times in 33 rounds</title></rect>
<text x="467.5" y="469.0" font-size="11" fill="#2c2c2c" text-anchor="middle">ABOUT 2</text>
<rect x="519.0" y="454" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>MiniMax M3: ARISE played 2 times in 33 rounds</title></rect>
<text x="568.5" y="469.0" font-size="11" fill="#2c2c2c" text-anchor="middle">ARISE 2</text>
<rect x="620.0" y="454" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>MiniMax M3: DIZZY played 2 times in 33 rounds</title></rect>
<text x="669.5" y="469.0" font-size="11" fill="#2c2c2c" text-anchor="middle">DIZZY 2</text>
<text x="215.0" y="496.0" font-size="11" fill="#8a8878" text-anchor="start">each model's five most-played words and how many times it played them, over five runs</text>
</svg>

The shape of the words is the other way to look at it, and it works at ascension 10 too, where there are no favourites to count. Every guess scores as one of five shapes, and relics and levels pay for particular ones. Most of every model's guesses were plain words with no repeated letter. A fifth of the game's answers repeat a letter, and only Luna, with its JAZZY and EERIE, played doubles at anything like that rate: 16 percent, against 4 to 8 for the rest.

<svg class="results" style="width:100%;height:auto;margin:1em 0" viewBox="0 0 720 534" role="img" aria-labelledby="shapes-title" font-family="inherit">
<title id="shapes-title">Share of each model's guesses in each of the five word shapes, beside the share of the game's answers in each shape.</title>
<text x="265.5" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">Alphabetical</text>
<text x="366.5" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">Vowel Heavy</text>
<text x="467.5" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">Cluster</text>
<text x="568.5" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">Twinned</text>
<text x="669.5" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">Distinct</text>
<text x="0.0" y="28.0" font-size="12" fill="#8c0000" text-anchor="start" font-weight="700">Ascension 10, the hardest</text>
<text x="0.0" y="51.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Opus 5.5</text>
<rect x="216.0" y="36" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.07"><title>Claude Opus 5.5: Alphabetical is 2.2% of guesses (8 of 358)</title></rect>
<text x="265.5" y="51.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2%</text>
<rect x="317.0" y="36" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.33"><title>Claude Opus 5.5: Vowel Heavy is 9.8% of guesses (35 of 358)</title></rect>
<text x="366.5" y="51.0" font-size="11" fill="#2c2c2c" text-anchor="middle">10%</text>
<rect x="418.0" y="36" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.42"><title>Claude Opus 5.5: Cluster is 12.6% of guesses (45 of 358)</title></rect>
<text x="467.5" y="51.0" font-size="11" fill="#2c2c2c" text-anchor="middle">13%</text>
<rect x="519.0" y="36" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.24"><title>Claude Opus 5.5: Twinned is 7.3% of guesses (26 of 358)</title></rect>
<text x="568.5" y="51.0" font-size="11" fill="#2c2c2c" text-anchor="middle">7%</text>
<rect x="620.0" y="36" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.70"><title>Claude Opus 5.5: Distinct is 68.2% of guesses (244 of 358)</title></rect>
<text x="669.5" y="51.0" font-size="11" fill="#ffffff" text-anchor="middle">68%</text>
<text x="0.0" y="77.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Sonnet 5.5</text>
<rect x="216.0" y="62" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Claude Sonnet 5.5: Alphabetical is 1.2% of guesses (5 of 401)</title></rect>
<text x="265.5" y="77.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1%</text>
<rect x="317.0" y="62" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.50"><title>Claude Sonnet 5.5: Vowel Heavy is 15.0% of guesses (60 of 401)</title></rect>
<text x="366.5" y="77.0" font-size="11" fill="#2c2c2c" text-anchor="middle">15%</text>
<rect x="418.0" y="62" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.33"><title>Claude Sonnet 5.5: Cluster is 10.0% of guesses (40 of 401)</title></rect>
<text x="467.5" y="77.0" font-size="11" fill="#2c2c2c" text-anchor="middle">10%</text>
<rect x="519.0" y="62" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.27"><title>Claude Sonnet 5.5: Twinned is 8.2% of guesses (33 of 401)</title></rect>
<text x="568.5" y="77.0" font-size="11" fill="#2c2c2c" text-anchor="middle">8%</text>
<rect x="620.0" y="62" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.64"><title>Claude Sonnet 5.5: Distinct is 65.6% of guesses (263 of 401)</title></rect>
<text x="669.5" y="77.0" font-size="11" fill="#ffffff" text-anchor="middle">66%</text>
<text x="0.0" y="103.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6 Sol</text>
<rect x="216.0" y="88" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>GPT-6 Sol: Alphabetical is 1.1% of guesses (3 of 275)</title></rect>
<text x="265.5" y="103.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1%</text>
<rect x="317.0" y="88" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.48"><title>GPT-6 Sol: Vowel Heavy is 14.5% of guesses (40 of 275)</title></rect>
<text x="366.5" y="103.0" font-size="11" fill="#2c2c2c" text-anchor="middle">15%</text>
<rect x="418.0" y="88" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Sol: Cluster is 9.5% of guesses (26 of 275)</title></rect>
<text x="467.5" y="103.0" font-size="11" fill="#2c2c2c" text-anchor="middle">9%</text>
<rect x="519.0" y="88" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.22"><title>GPT-6 Sol: Twinned is 6.5% of guesses (18 of 275)</title></rect>
<text x="568.5" y="103.0" font-size="11" fill="#2c2c2c" text-anchor="middle">7%</text>
<rect x="620.0" y="88" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.71"><title>GPT-6 Sol: Distinct is 68.4% of guesses (188 of 275)</title></rect>
<text x="669.5" y="103.0" font-size="11" fill="#ffffff" text-anchor="middle">68%</text>
<text x="0.0" y="129.0" font-size="13" fill="#2c2c2c" text-anchor="start">MiMo-V2.6-Pro</text>
<rect x="216.0" y="114" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.10"><title>MiMo-V2.6-Pro: Alphabetical is 3.1% of guesses (9 of 287)</title></rect>
<text x="265.5" y="129.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3%</text>
<rect x="317.0" y="114" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.46"><title>MiMo-V2.6-Pro: Vowel Heavy is 13.9% of guesses (40 of 287)</title></rect>
<text x="366.5" y="129.0" font-size="11" fill="#2c2c2c" text-anchor="middle">14%</text>
<rect x="418.0" y="114" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.50"><title>MiMo-V2.6-Pro: Cluster is 15.0% of guesses (43 of 287)</title></rect>
<text x="467.5" y="129.0" font-size="11" fill="#2c2c2c" text-anchor="middle">15%</text>
<rect x="519.0" y="114" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.50"><title>MiMo-V2.6-Pro: Twinned is 15.0% of guesses (43 of 287)</title></rect>
<text x="568.5" y="129.0" font-size="11" fill="#2c2c2c" text-anchor="middle">15%</text>
<rect x="620.0" y="114" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiMo-V2.6-Pro: Distinct is 53.0% of guesses (152 of 287)</title></rect>
<text x="669.5" y="129.0" font-size="11" fill="#2c2c2c" text-anchor="middle">53%</text>
<text x="0.0" y="155.0" font-size="13" fill="#2c2c2c" text-anchor="start">DeepSeek V4.1 Flash</text>
<rect x="216.0" y="140" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>DeepSeek V4.1 Flash: Alphabetical is 1.1% of guesses (3 of 279)</title></rect>
<text x="265.5" y="155.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1%</text>
<rect x="317.0" y="140" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.47"><title>DeepSeek V4.1 Flash: Vowel Heavy is 14.0% of guesses (39 of 279)</title></rect>
<text x="366.5" y="155.0" font-size="11" fill="#2c2c2c" text-anchor="middle">14%</text>
<rect x="418.0" y="140" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.39"><title>DeepSeek V4.1 Flash: Cluster is 11.8% of guesses (33 of 279)</title></rect>
<text x="467.5" y="155.0" font-size="11" fill="#2c2c2c" text-anchor="middle">12%</text>
<rect x="519.0" y="140" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.33"><title>DeepSeek V4.1 Flash: Twinned is 10.0% of guesses (28 of 279)</title></rect>
<text x="568.5" y="155.0" font-size="11" fill="#2c2c2c" text-anchor="middle">10%</text>
<rect x="620.0" y="140" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.58"><title>DeepSeek V4.1 Flash: Distinct is 63.1% of guesses (176 of 279)</title></rect>
<text x="669.5" y="155.0" font-size="11" fill="#2c2c2c" text-anchor="middle">63%</text>
<text x="0.0" y="181.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6.1 Sol</text>
<rect x="216.0" y="166" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>GPT-6.1 Sol: Alphabetical is 1.6% of guesses (4 of 257)</title></rect>
<text x="265.5" y="181.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2%</text>
<rect x="317.0" y="166" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.26"><title>GPT-6.1 Sol: Vowel Heavy is 7.8% of guesses (20 of 257)</title></rect>
<text x="366.5" y="181.0" font-size="11" fill="#2c2c2c" text-anchor="middle">8%</text>
<rect x="418.0" y="166" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.44"><title>GPT-6.1 Sol: Cluster is 13.2% of guesses (34 of 257)</title></rect>
<text x="467.5" y="181.0" font-size="11" fill="#2c2c2c" text-anchor="middle">13%</text>
<rect x="519.0" y="166" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.19"><title>GPT-6.1 Sol: Twinned is 5.8% of guesses (15 of 257)</title></rect>
<text x="568.5" y="181.0" font-size="11" fill="#2c2c2c" text-anchor="middle">6%</text>
<rect x="620.0" y="166" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.79"><title>GPT-6.1 Sol: Distinct is 71.6% of guesses (184 of 257)</title></rect>
<text x="669.5" y="181.0" font-size="11" fill="#ffffff" text-anchor="middle">72%</text>
<text x="0.0" y="207.0" font-size="13" fill="#2c2c2c" text-anchor="start">DeepSeek V4 Pro</text>
<rect x="216.0" y="192" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.07"><title>DeepSeek V4 Pro: Alphabetical is 2.1% of guesses (5 of 233)</title></rect>
<text x="265.5" y="207.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2%</text>
<rect x="317.0" y="192" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.40"><title>DeepSeek V4 Pro: Vowel Heavy is 12.0% of guesses (28 of 233)</title></rect>
<text x="366.5" y="207.0" font-size="11" fill="#2c2c2c" text-anchor="middle">12%</text>
<rect x="418.0" y="192" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.26"><title>DeepSeek V4 Pro: Cluster is 7.7% of guesses (18 of 233)</title></rect>
<text x="467.5" y="207.0" font-size="11" fill="#2c2c2c" text-anchor="middle">8%</text>
<rect x="519.0" y="192" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.19"><title>DeepSeek V4 Pro: Twinned is 5.6% of guesses (13 of 233)</title></rect>
<text x="568.5" y="207.0" font-size="11" fill="#2c2c2c" text-anchor="middle">6%</text>
<rect x="620.0" y="192" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.81"><title>DeepSeek V4 Pro: Distinct is 72.5% of guesses (169 of 233)</title></rect>
<text x="669.5" y="207.0" font-size="11" fill="#ffffff" text-anchor="middle">73%</text>
<text x="0.0" y="233.0" font-size="13" fill="#2c2c2c" text-anchor="start">GLM 5.3 Flash</text>
<rect x="216.0" y="218" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.07"><title>GLM 5.3 Flash: Alphabetical is 2.1% of guesses (5 of 243)</title></rect>
<text x="265.5" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2%</text>
<rect x="317.0" y="218" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.51"><title>GLM 5.3 Flash: Vowel Heavy is 15.2% of guesses (37 of 243)</title></rect>
<text x="366.5" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">15%</text>
<rect x="418.0" y="218" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.52"><title>GLM 5.3 Flash: Cluster is 15.6% of guesses (38 of 243)</title></rect>
<text x="467.5" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">16%</text>
<rect x="519.0" y="218" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.27"><title>GLM 5.3 Flash: Twinned is 8.2% of guesses (20 of 243)</title></rect>
<text x="568.5" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">8%</text>
<rect x="620.0" y="218" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.47"><title>GLM 5.3 Flash: Distinct is 58.8% of guesses (143 of 243)</title></rect>
<text x="669.5" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">59%</text>
<text x="0.0" y="259.0" font-size="13" fill="#2c2c2c" text-anchor="start">Gemini 3.8 Flash</text>
<rect x="216.0" y="244" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Gemini 3.8 Flash: Alphabetical is 1.2% of guesses (3 of 249)</title></rect>
<text x="265.5" y="259.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1%</text>
<rect x="317.0" y="244" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.31"><title>Gemini 3.8 Flash: Vowel Heavy is 9.2% of guesses (23 of 249)</title></rect>
<text x="366.5" y="259.0" font-size="11" fill="#2c2c2c" text-anchor="middle">9%</text>
<rect x="418.0" y="244" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.33"><title>Gemini 3.8 Flash: Cluster is 10.0% of guesses (25 of 249)</title></rect>
<text x="467.5" y="259.0" font-size="11" fill="#2c2c2c" text-anchor="middle">10%</text>
<rect x="519.0" y="244" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.21"><title>Gemini 3.8 Flash: Twinned is 6.4% of guesses (16 of 249)</title></rect>
<text x="568.5" y="259.0" font-size="11" fill="#2c2c2c" text-anchor="middle">6%</text>
<rect x="620.0" y="244" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.83"><title>Gemini 3.8 Flash: Distinct is 73.1% of guesses (182 of 249)</title></rect>
<text x="669.5" y="259.0" font-size="11" fill="#ffffff" text-anchor="middle">73%</text>
<text x="0.0" y="290.0" font-size="12" fill="#8c0000" text-anchor="start" font-weight="700">Ascension 0, the easiest</text>
<text x="0.0" y="313.0" font-size="13" fill="#2c2c2c" text-anchor="start">Gemini 3.8 Flash</text>
<rect x="216.0" y="298" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.07"><title>Gemini 3.8 Flash: Alphabetical is 2.2% of guesses (10 of 459)</title></rect>
<text x="265.5" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2%</text>
<rect x="317.0" y="298" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.25"><title>Gemini 3.8 Flash: Vowel Heavy is 7.6% of guesses (35 of 459)</title></rect>
<text x="366.5" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">8%</text>
<rect x="418.0" y="298" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.41"><title>Gemini 3.8 Flash: Cluster is 12.2% of guesses (56 of 459)</title></rect>
<text x="467.5" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">12%</text>
<rect x="519.0" y="298" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.15"><title>Gemini 3.8 Flash: Twinned is 4.6% of guesses (21 of 459)</title></rect>
<text x="568.5" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5%</text>
<rect x="620.0" y="298" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.84"><title>Gemini 3.8 Flash: Distinct is 73.4% of guesses (337 of 459)</title></rect>
<text x="669.5" y="313.0" font-size="11" fill="#ffffff" text-anchor="middle">73%</text>
<text x="0.0" y="339.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Sonnet 5.5</text>
<rect x="216.0" y="324" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Claude Sonnet 5.5: Alphabetical is 0.7% of guesses (3 of 424)</title></rect>
<text x="265.5" y="339.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1%</text>
<rect x="317.0" y="324" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.25"><title>Claude Sonnet 5.5: Vowel Heavy is 7.5% of guesses (32 of 424)</title></rect>
<text x="366.5" y="339.0" font-size="11" fill="#2c2c2c" text-anchor="middle">8%</text>
<rect x="418.0" y="324" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.44"><title>Claude Sonnet 5.5: Cluster is 13.2% of guesses (56 of 424)</title></rect>
<text x="467.5" y="339.0" font-size="11" fill="#2c2c2c" text-anchor="middle">13%</text>
<rect x="519.0" y="324" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.20"><title>Claude Sonnet 5.5: Twinned is 6.1% of guesses (26 of 424)</title></rect>
<text x="568.5" y="339.0" font-size="11" fill="#2c2c2c" text-anchor="middle">6%</text>
<rect x="620.0" y="324" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.81"><title>Claude Sonnet 5.5: Distinct is 72.4% of guesses (307 of 424)</title></rect>
<text x="669.5" y="339.0" font-size="11" fill="#ffffff" text-anchor="middle">72%</text>
<text x="0.0" y="365.0" font-size="13" fill="#2c2c2c" text-anchor="start">Step 5 Preview</text>
<rect x="216.0" y="350" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Step 5 Preview: Alphabetical is 1.4% of guesses (5 of 357)</title></rect>
<text x="265.5" y="365.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1%</text>
<rect x="317.0" y="350" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Step 5 Preview: Vowel Heavy is 14.8% of guesses (53 of 357)</title></rect>
<text x="366.5" y="365.0" font-size="11" fill="#2c2c2c" text-anchor="middle">15%</text>
<rect x="418.0" y="350" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.93"><title>Step 5 Preview: Cluster is 28.0% of guesses (100 of 357)</title></rect>
<text x="467.5" y="365.0" font-size="11" fill="#ffffff" text-anchor="middle">28%</text>
<rect x="519.0" y="350" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.35"><title>Step 5 Preview: Twinned is 10.4% of guesses (37 of 357)</title></rect>
<text x="568.5" y="365.0" font-size="11" fill="#2c2c2c" text-anchor="middle">10%</text>
<rect x="620.0" y="350" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.13"><title>Step 5 Preview: Distinct is 45.4% of guesses (162 of 357)</title></rect>
<text x="669.5" y="365.0" font-size="11" fill="#2c2c2c" text-anchor="middle">45%</text>
<text x="0.0" y="391.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6 Luna</text>
<rect x="216.0" y="376" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>GPT-6 Luna: Alphabetical is 1.7% of guesses (6 of 358)</title></rect>
<text x="265.5" y="391.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2%</text>
<rect x="317.0" y="376" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.52"><title>GPT-6 Luna: Vowel Heavy is 15.6% of guesses (56 of 358)</title></rect>
<text x="366.5" y="391.0" font-size="11" fill="#2c2c2c" text-anchor="middle">16%</text>
<rect x="418.0" y="376" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.65"><title>GPT-6 Luna: Cluster is 19.6% of guesses (70 of 358)</title></rect>
<text x="467.5" y="391.0" font-size="11" fill="#ffffff" text-anchor="middle">20%</text>
<rect x="519.0" y="376" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.52"><title>GPT-6 Luna: Twinned is 15.6% of guesses (56 of 358)</title></rect>
<text x="568.5" y="391.0" font-size="11" fill="#2c2c2c" text-anchor="middle">16%</text>
<rect x="620.0" y="376" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.19"><title>GPT-6 Luna: Distinct is 47.5% of guesses (170 of 358)</title></rect>
<text x="669.5" y="391.0" font-size="11" fill="#2c2c2c" text-anchor="middle">47%</text>
<text x="0.0" y="417.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Haiku 5.5</text>
<rect x="216.0" y="402" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Claude Haiku 5.5: Alphabetical is 1.0% of guesses (2 of 210)</title></rect>
<text x="265.5" y="417.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1%</text>
<rect x="317.0" y="402" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.22"><title>Claude Haiku 5.5: Vowel Heavy is 6.7% of guesses (14 of 210)</title></rect>
<text x="366.5" y="417.0" font-size="11" fill="#2c2c2c" text-anchor="middle">7%</text>
<rect x="418.0" y="402" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.62"><title>Claude Haiku 5.5: Cluster is 18.6% of guesses (39 of 210)</title></rect>
<text x="467.5" y="417.0" font-size="11" fill="#ffffff" text-anchor="middle">19%</text>
<rect x="519.0" y="402" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.13"><title>Claude Haiku 5.5: Twinned is 3.8% of guesses (8 of 210)</title></rect>
<text x="568.5" y="417.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4%</text>
<rect x="620.0" y="402" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.75"><title>Claude Haiku 5.5: Distinct is 70.0% of guesses (147 of 210)</title></rect>
<text x="669.5" y="417.0" font-size="11" fill="#ffffff" text-anchor="middle">70%</text>
<text x="0.0" y="443.0" font-size="13" fill="#2c2c2c" text-anchor="start">Nemotron 3 Ultra</text>
<rect x="216.0" y="428" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.12"><title>Nemotron 3 Ultra: Alphabetical is 3.7% of guesses (8 of 216)</title></rect>
<text x="265.5" y="443.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4%</text>
<rect x="317.0" y="428" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.29"><title>Nemotron 3 Ultra: Vowel Heavy is 8.8% of guesses (19 of 216)</title></rect>
<text x="366.5" y="443.0" font-size="11" fill="#2c2c2c" text-anchor="middle">9%</text>
<rect x="418.0" y="428" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.79"><title>Nemotron 3 Ultra: Cluster is 23.6% of guesses (51 of 216)</title></rect>
<text x="467.5" y="443.0" font-size="11" fill="#ffffff" text-anchor="middle">24%</text>
<rect x="519.0" y="428" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.28"><title>Nemotron 3 Ultra: Twinned is 8.3% of guesses (18 of 216)</title></rect>
<text x="568.5" y="443.0" font-size="11" fill="#2c2c2c" text-anchor="middle">8%</text>
<rect x="620.0" y="428" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.39"><title>Nemotron 3 Ultra: Distinct is 55.6% of guesses (120 of 216)</title></rect>
<text x="669.5" y="443.0" font-size="11" fill="#2c2c2c" text-anchor="middle">56%</text>
<text x="0.0" y="469.0" font-size="13" fill="#2c2c2c" text-anchor="start">MiniMax M3</text>
<rect x="216.0" y="454" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>MiniMax M3: Alphabetical is 0.7% of guesses (1 of 153)</title></rect>
<text x="265.5" y="469.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1%</text>
<rect x="317.0" y="454" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.48"><title>MiniMax M3: Vowel Heavy is 14.4% of guesses (22 of 153)</title></rect>
<text x="366.5" y="469.0" font-size="11" fill="#2c2c2c" text-anchor="middle">14%</text>
<rect x="418.0" y="454" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.76"><title>MiniMax M3: Cluster is 22.9% of guesses (35 of 153)</title></rect>
<text x="467.5" y="469.0" font-size="11" fill="#ffffff" text-anchor="middle">23%</text>
<rect x="519.0" y="454" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.20"><title>MiniMax M3: Twinned is 5.9% of guesses (9 of 153)</title></rect>
<text x="568.5" y="469.0" font-size="11" fill="#2c2c2c" text-anchor="middle">6%</text>
<rect x="620.0" y="454" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.41"><title>MiniMax M3: Distinct is 56.2% of guesses (86 of 153)</title></rect>
<text x="669.5" y="469.0" font-size="11" fill="#2c2c2c" text-anchor="middle">56%</text>
<text x="0.0" y="501.0" font-size="13" fill="#8a8878" text-anchor="start">Every answer in the game</text>
<rect x="216.0" y="486" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Every answer in the game: Alphabetical is 1.9% of answers (43 of 2300)</title></rect>
<text x="265.5" y="501.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2%</text>
<rect x="317.0" y="486" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.30"><title>Every answer in the game: Vowel Heavy is 9.0% of answers (208 of 2300)</title></rect>
<text x="366.5" y="501.0" font-size="11" fill="#2c2c2c" text-anchor="middle">9%</text>
<rect x="418.0" y="486" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.55"><title>Every answer in the game: Cluster is 16.4% of answers (377 of 2300)</title></rect>
<text x="467.5" y="501.0" font-size="11" fill="#2c2c2c" text-anchor="middle">16%</text>
<rect x="519.0" y="486" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.66"><title>Every answer in the game: Twinned is 19.9% of answers (457 of 2300)</title></rect>
<text x="568.5" y="501.0" font-size="11" fill="#ffffff" text-anchor="middle">20%</text>
<rect x="620.0" y="486" width="99.0" height="22" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Every answer in the game: Distinct is 52.8% of answers (1215 of 2300)</title></rect>
<text x="669.5" y="501.0" font-size="11" fill="#2c2c2c" text-anchor="middle">53%</text>
<text x="215.0" y="528.0" font-size="11" fill="#8a8878" text-anchor="start">share of guesses in each word shape · a guess scores as the first shape on this list it matches</text>
</svg>

Guesses per cleared round:

<svg class="results" style="width:100%;height:auto;margin:1em 0" viewBox="0 0 720 456" role="img" aria-labelledby="guesses-title" font-family="inherit">
<title id="guesses-title">Guesses per cleared round for each model. Lower is better; six is the limit.</title>
<text x="0.0" y="14.0" font-size="12" fill="#8c0000" text-anchor="start" font-weight="700">Ascension 10, the hardest</text>
<text x="0.0" y="35.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Opus 5.5</text>
<rect x="215" y="24" width="240.0" height="14" rx="2" fill="#8c0000"><title>Claude Opus 5.5: 3.59 guesses per cleared round, solved 97 of 97 cleared rounds</title></rect>
<text x="463.0" y="35.0" font-size="12" fill="#2c2c2c" text-anchor="start">3.59 · solved 97 of 97</text>
<text x="0.0" y="59.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Sonnet 5.5</text>
<rect x="215" y="48" width="262.1" height="14" rx="2" fill="#8c0000"><title>Claude Sonnet 5.5: 3.92 guesses per cleared round, solved 97 of 98 cleared rounds</title></rect>
<text x="485.1" y="59.0" font-size="12" fill="#2c2c2c" text-anchor="start">3.92 · solved 97 of 98</text>
<text x="0.0" y="83.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6 Sol</text>
<rect x="215" y="72" width="251.9" height="14" rx="2" fill="#8c0000"><title>GPT-6 Sol: 3.76 guesses per cleared round, solved 68 of 68 cleared rounds</title></rect>
<text x="474.9" y="83.0" font-size="12" fill="#2c2c2c" text-anchor="start">3.76 · solved 68 of 68</text>
<text x="0.0" y="107.0" font-size="13" fill="#2c2c2c" text-anchor="start">MiMo-V2.6-Pro</text>
<rect x="215" y="96" width="268.6" height="14" rx="2" fill="#8c0000"><title>MiMo-V2.6-Pro: 4.02 guesses per cleared round, solved 65 of 65 cleared rounds</title></rect>
<text x="491.6" y="107.0" font-size="12" fill="#2c2c2c" text-anchor="start">4.02 · solved 65 of 65</text>
<text x="0.0" y="131.0" font-size="13" fill="#2c2c2c" text-anchor="start">DeepSeek V4.1 Flash</text>
<rect x="215" y="120" width="275.2" height="14" rx="2" fill="#8c0000"><title>DeepSeek V4.1 Flash: 4.11 guesses per cleared round, solved 62 of 62 cleared rounds</title></rect>
<text x="498.2" y="131.0" font-size="12" fill="#2c2c2c" text-anchor="start">4.11 · solved 62 of 62</text>
<text x="0.0" y="155.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6.1 Sol</text>
<rect x="215" y="144" width="256.4" height="14" rx="2" fill="#8c0000"><title>GPT-6.1 Sol: 3.83 guesses per cleared round, solved 60 of 60 cleared rounds</title></rect>
<text x="479.4" y="155.0" font-size="12" fill="#2c2c2c" text-anchor="start">3.83 · solved 60 of 60</text>
<text x="0.0" y="179.0" font-size="13" fill="#2c2c2c" text-anchor="start">DeepSeek V4 Pro</text>
<rect x="215" y="168" width="247.3" height="14" rx="2" fill="#8c0000"><title>DeepSeek V4 Pro: 3.70 guesses per cleared round, solved 56 of 56 cleared rounds</title></rect>
<text x="470.3" y="179.0" font-size="12" fill="#2c2c2c" text-anchor="start">3.70 · solved 56 of 56</text>
<text x="0.0" y="203.0" font-size="13" fill="#2c2c2c" text-anchor="start">GLM 5.3 Flash</text>
<rect x="215" y="192" width="268.8" height="14" rx="2" fill="#8c0000"><title>GLM 5.3 Flash: 4.02 guesses per cleared round, solved 55 of 55 cleared rounds</title></rect>
<text x="491.8" y="203.0" font-size="12" fill="#2c2c2c" text-anchor="start">4.02 · solved 55 of 55</text>
<text x="0.0" y="227.0" font-size="13" fill="#2c2c2c" text-anchor="start">Gemini 3.8 Flash</text>
<rect x="215" y="216" width="256.1" height="14" rx="2" fill="#8c0000"><title>Gemini 3.8 Flash: 3.83 guesses per cleared round, solved 58 of 58 cleared rounds</title></rect>
<text x="479.1" y="227.0" font-size="12" fill="#2c2c2c" text-anchor="start">3.83 · solved 58 of 58</text>
<text x="0.0" y="258.0" font-size="12" fill="#8c0000" text-anchor="start" font-weight="700">Ascension 0, the easiest</text>
<text x="0.0" y="279.0" font-size="13" fill="#2c2c2c" text-anchor="start">Gemini 3.8 Flash</text>
<rect x="215" y="268" width="255.9" height="14" rx="2" fill="#8c0000"><title>Gemini 3.8 Flash: 3.83 guesses per cleared round, solved 114 of 120 cleared rounds</title></rect>
<text x="478.9" y="279.0" font-size="12" fill="#2c2c2c" text-anchor="start">3.83 · solved 114 of 120</text>
<text x="0.0" y="303.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Sonnet 5.5</text>
<rect x="215" y="292" width="267.6" height="14" rx="2" fill="#8c0000"><title>Claude Sonnet 5.5: 4.00 guesses per cleared round, solved 90 of 102 cleared rounds</title></rect>
<text x="490.6" y="303.0" font-size="12" fill="#2c2c2c" text-anchor="start">4.00 · solved 90 of 102</text>
<text x="0.0" y="327.0" font-size="13" fill="#2c2c2c" text-anchor="start">Step 5 Preview</text>
<rect x="215" y="316" width="355.0" height="14" rx="2" fill="#8c0000"><title>Step 5 Preview: 5.31 guesses per cleared round, solved 35 of 62 cleared rounds</title></rect>
<text x="578.0" y="327.0" font-size="12" fill="#2c2c2c" text-anchor="start">5.31 · solved 35 of 62</text>
<text x="0.0" y="351.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6 Luna</text>
<rect x="215" y="340" width="314.4" height="14" rx="2" fill="#8c0000"><title>GPT-6 Luna: 4.70 guesses per cleared round, solved 64 of 70 cleared rounds</title></rect>
<text x="537.4" y="351.0" font-size="12" fill="#2c2c2c" text-anchor="start">4.70 · solved 64 of 70</text>
<text x="0.0" y="375.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Haiku 5.5</text>
<rect x="215" y="364" width="284.7" height="14" rx="2" fill="#8c0000"><title>Claude Haiku 5.5: 4.26 guesses per cleared round, solved 38 of 43 cleared rounds</title></rect>
<text x="507.7" y="375.0" font-size="12" fill="#2c2c2c" text-anchor="start">4.26 · solved 38 of 43</text>
<text x="0.0" y="399.0" font-size="13" fill="#2c2c2c" text-anchor="start">Nemotron 3 Ultra</text>
<rect x="215" y="388" width="301.0" height="14" rx="2" fill="#8c0000"><title>Nemotron 3 Ultra: 4.50 guesses per cleared round, solved 36 of 42 cleared rounds</title></rect>
<text x="524.0" y="399.0" font-size="12" fill="#2c2c2c" text-anchor="start">4.50 · solved 36 of 42</text>
<text x="0.0" y="423.0" font-size="13" fill="#2c2c2c" text-anchor="start">MiniMax M3</text>
<rect x="215" y="412" width="298.7" height="14" rx="2" fill="#8c0000"><title>MiniMax M3: 4.46 guesses per cleared round, solved 28 of 28 cleared rounds</title></rect>
<text x="521.7" y="423.0" font-size="12" fill="#2c2c2c" text-anchor="start">4.46 · solved 28 of 28</text>
<text x="215.0" y="450.0" font-size="11" fill="#8a8878" text-anchor="start">guesses per cleared round (lower is better, six is the limit) · rounds solved of rounds cleared</text>
</svg>

"Rounds solved" is how many of the rounds a player cleared it cleared by finding the word, rather than by reaching the target without it. Sonnet's base-game figure carries the TAXIS rounds, which took all six guesses on purpose.

## More from the runs

Every run is a list of moves, so it can be replayed and measured any way that seems interesting.

### How rounds ended

Wordle's own chart, one per model: how many rounds were solved in each number of guesses, how many were cleared on score without solving, and how many were lost. Opus solved 45 of its 99 rounds in three guesses.

<svg class="results" style="width:100%;height:auto;margin:1em 0" viewBox="0 0 720 502" role="img" aria-labelledby="dist-title" font-family="inherit">
<title id="dist-title">How each model's rounds ended: solved in one to six guesses, cleared without solving, or lost.</title>
<text x="246.6" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">1</text>
<text x="309.7" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">2</text>
<text x="372.8" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">3</text>
<text x="435.9" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">4</text>
<text x="499.1" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">5</text>
<text x="562.2" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">6</text>
<text x="625.3" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">no solve</text>
<text x="688.4" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">lost</text>
<text x="0.0" y="28.0" font-size="12" fill="#8c0000" text-anchor="start" font-weight="700">Ascension 10, the hardest</text>
<text x="0.0" y="51.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Opus 5.5</text>
<rect x="279.1" y="36" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.16"><title>Claude Opus 5.5: 8 of 99 rounds, 2</title></rect>
<text x="309.7" y="51.0" font-size="11" fill="#2c2c2c" text-anchor="middle">8</text>
<rect x="342.3" y="36" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.91"><title>Claude Opus 5.5: 45 of 99 rounds, 3</title></rect>
<text x="372.8" y="51.0" font-size="11" fill="#ffffff" text-anchor="middle">45</text>
<rect x="405.4" y="36" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.61"><title>Claude Opus 5.5: 30 of 99 rounds, 4</title></rect>
<text x="435.9" y="51.0" font-size="11" fill="#ffffff" text-anchor="middle">30</text>
<rect x="468.5" y="36" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.14"><title>Claude Opus 5.5: 7 of 99 rounds, 5</title></rect>
<text x="499.1" y="51.0" font-size="11" fill="#2c2c2c" text-anchor="middle">7</text>
<rect x="531.6" y="36" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.14"><title>Claude Opus 5.5: 7 of 99 rounds, 6</title></rect>
<text x="562.2" y="51.0" font-size="11" fill="#2c2c2c" text-anchor="middle">7</text>
<rect x="657.9" y="36" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Claude Opus 5.5: 2 of 99 rounds, lost</title></rect>
<text x="688.4" y="51.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<text x="0.0" y="77.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Sonnet 5.5</text>
<rect x="279.1" y="62" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.08"><title>Claude Sonnet 5.5: 4 of 102 rounds, 2</title></rect>
<text x="309.7" y="77.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4</text>
<rect x="342.3" y="62" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.53"><title>Claude Sonnet 5.5: 27 of 102 rounds, 3</title></rect>
<text x="372.8" y="77.0" font-size="11" fill="#2c2c2c" text-anchor="middle">27</text>
<rect x="405.4" y="62" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.88"><title>Claude Sonnet 5.5: 45 of 102 rounds, 4</title></rect>
<text x="435.9" y="77.0" font-size="11" fill="#ffffff" text-anchor="middle">45</text>
<rect x="468.5" y="62" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.29"><title>Claude Sonnet 5.5: 15 of 102 rounds, 5</title></rect>
<text x="499.1" y="77.0" font-size="11" fill="#2c2c2c" text-anchor="middle">15</text>
<rect x="531.6" y="62" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.12"><title>Claude Sonnet 5.5: 6 of 102 rounds, 6</title></rect>
<text x="562.2" y="77.0" font-size="11" fill="#2c2c2c" text-anchor="middle">6</text>
<rect x="594.8" y="62" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Claude Sonnet 5.5: 1 of 102 rounds, no solve</title></rect>
<text x="625.3" y="77.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="657.9" y="62" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.08"><title>Claude Sonnet 5.5: 4 of 102 rounds, lost</title></rect>
<text x="688.4" y="77.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4</text>
<text x="0.0" y="103.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6 Sol</text>
<rect x="279.1" y="88" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.08"><title>GPT-6 Sol: 3 of 72 rounds, 2</title></rect>
<text x="309.7" y="103.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3</text>
<rect x="342.3" y="88" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.69"><title>GPT-6 Sol: 25 of 72 rounds, 3</title></rect>
<text x="372.8" y="103.0" font-size="11" fill="#ffffff" text-anchor="middle">25</text>
<rect x="405.4" y="88" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.72"><title>GPT-6 Sol: 26 of 72 rounds, 4</title></rect>
<text x="435.9" y="103.0" font-size="11" fill="#ffffff" text-anchor="middle">26</text>
<rect x="468.5" y="88" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.36"><title>GPT-6 Sol: 13 of 72 rounds, 5</title></rect>
<text x="499.1" y="103.0" font-size="11" fill="#2c2c2c" text-anchor="middle">13</text>
<rect x="531.6" y="88" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>GPT-6 Sol: 1 of 72 rounds, 6</title></rect>
<text x="562.2" y="103.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="657.9" y="88" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.11"><title>GPT-6 Sol: 4 of 72 rounds, lost</title></rect>
<text x="688.4" y="103.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4</text>
<text x="0.0" y="129.0" font-size="13" fill="#2c2c2c" text-anchor="start">MiMo-V2.6-Pro</text>
<rect x="279.1" y="114" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>MiMo-V2.6-Pro: 1 of 70 rounds, 2</title></rect>
<text x="309.7" y="129.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="342.3" y="114" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.60"><title>MiMo-V2.6-Pro: 21 of 70 rounds, 3</title></rect>
<text x="372.8" y="129.0" font-size="11" fill="#2c2c2c" text-anchor="middle">21</text>
<rect x="405.4" y="114" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.77"><title>MiMo-V2.6-Pro: 27 of 70 rounds, 4</title></rect>
<text x="435.9" y="129.0" font-size="11" fill="#ffffff" text-anchor="middle">27</text>
<rect x="468.5" y="114" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.23"><title>MiMo-V2.6-Pro: 8 of 70 rounds, 5</title></rect>
<text x="499.1" y="129.0" font-size="11" fill="#2c2c2c" text-anchor="middle">8</text>
<rect x="531.6" y="114" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.23"><title>MiMo-V2.6-Pro: 8 of 70 rounds, 6</title></rect>
<text x="562.2" y="129.0" font-size="11" fill="#2c2c2c" text-anchor="middle">8</text>
<rect x="657.9" y="114" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.14"><title>MiMo-V2.6-Pro: 5 of 70 rounds, lost</title></rect>
<text x="688.4" y="129.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5</text>
<text x="0.0" y="155.0" font-size="13" fill="#2c2c2c" text-anchor="start">DeepSeek V4.1 Flash</text>
<rect x="279.1" y="140" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.09"><title>DeepSeek V4.1 Flash: 3 of 67 rounds, 2</title></rect>
<text x="309.7" y="155.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3</text>
<rect x="342.3" y="140" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.39"><title>DeepSeek V4.1 Flash: 13 of 67 rounds, 3</title></rect>
<text x="372.8" y="155.0" font-size="11" fill="#2c2c2c" text-anchor="middle">13</text>
<rect x="405.4" y="140" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.75"><title>DeepSeek V4.1 Flash: 25 of 67 rounds, 4</title></rect>
<text x="435.9" y="155.0" font-size="11" fill="#ffffff" text-anchor="middle">25</text>
<rect x="468.5" y="140" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.48"><title>DeepSeek V4.1 Flash: 16 of 67 rounds, 5</title></rect>
<text x="499.1" y="155.0" font-size="11" fill="#2c2c2c" text-anchor="middle">16</text>
<rect x="531.6" y="140" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.15"><title>DeepSeek V4.1 Flash: 5 of 67 rounds, 6</title></rect>
<text x="562.2" y="155.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5</text>
<rect x="657.9" y="140" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.15"><title>DeepSeek V4.1 Flash: 5 of 67 rounds, lost</title></rect>
<text x="688.4" y="155.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5</text>
<text x="0.0" y="181.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6.1 Sol</text>
<rect x="279.1" y="166" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>GPT-6.1 Sol: 2 of 65 rounds, 2</title></rect>
<text x="309.7" y="181.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="342.3" y="166" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.58"><title>GPT-6.1 Sol: 19 of 65 rounds, 3</title></rect>
<text x="372.8" y="181.0" font-size="11" fill="#2c2c2c" text-anchor="middle">19</text>
<rect x="405.4" y="166" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.86"><title>GPT-6.1 Sol: 28 of 65 rounds, 4</title></rect>
<text x="435.9" y="181.0" font-size="11" fill="#ffffff" text-anchor="middle">28</text>
<rect x="468.5" y="166" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.28"><title>GPT-6.1 Sol: 9 of 65 rounds, 5</title></rect>
<text x="499.1" y="181.0" font-size="11" fill="#2c2c2c" text-anchor="middle">9</text>
<rect x="531.6" y="166" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>GPT-6.1 Sol: 2 of 65 rounds, 6</title></rect>
<text x="562.2" y="181.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="657.9" y="166" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.15"><title>GPT-6.1 Sol: 5 of 65 rounds, lost</title></rect>
<text x="688.4" y="181.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5</text>
<text x="0.0" y="207.0" font-size="13" fill="#2c2c2c" text-anchor="start">DeepSeek V4 Pro</text>
<rect x="279.1" y="192" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.16"><title>DeepSeek V4 Pro: 5 of 61 rounds, 2</title></rect>
<text x="309.7" y="207.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5</text>
<rect x="342.3" y="192" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.62"><title>DeepSeek V4 Pro: 19 of 61 rounds, 3</title></rect>
<text x="372.8" y="207.0" font-size="11" fill="#ffffff" text-anchor="middle">19</text>
<rect x="405.4" y="192" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.69"><title>DeepSeek V4 Pro: 21 of 61 rounds, 4</title></rect>
<text x="435.9" y="207.0" font-size="11" fill="#ffffff" text-anchor="middle">21</text>
<rect x="468.5" y="192" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.33"><title>DeepSeek V4 Pro: 10 of 61 rounds, 5</title></rect>
<text x="499.1" y="207.0" font-size="11" fill="#2c2c2c" text-anchor="middle">10</text>
<rect x="531.6" y="192" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>DeepSeek V4 Pro: 1 of 61 rounds, 6</title></rect>
<text x="562.2" y="207.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="657.9" y="192" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.16"><title>DeepSeek V4 Pro: 5 of 61 rounds, lost</title></rect>
<text x="688.4" y="207.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5</text>
<text x="0.0" y="233.0" font-size="13" fill="#2c2c2c" text-anchor="start">GLM 5.3 Flash</text>
<rect x="279.1" y="218" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.07"><title>GLM 5.3 Flash: 2 of 59 rounds, 2</title></rect>
<text x="309.7" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="342.3" y="218" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.68"><title>GLM 5.3 Flash: 20 of 59 rounds, 3</title></rect>
<text x="372.8" y="233.0" font-size="11" fill="#ffffff" text-anchor="middle">20</text>
<rect x="405.4" y="218" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.44"><title>GLM 5.3 Flash: 13 of 59 rounds, 4</title></rect>
<text x="435.9" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">13</text>
<rect x="468.5" y="218" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.51"><title>GLM 5.3 Flash: 15 of 59 rounds, 5</title></rect>
<text x="499.1" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">15</text>
<rect x="531.6" y="218" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.17"><title>GLM 5.3 Flash: 5 of 59 rounds, 6</title></rect>
<text x="562.2" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5</text>
<rect x="657.9" y="218" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.14"><title>GLM 5.3 Flash: 4 of 59 rounds, lost</title></rect>
<text x="688.4" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4</text>
<text x="0.0" y="259.0" font-size="13" fill="#2c2c2c" text-anchor="start">Gemini 3.8 Flash</text>
<rect x="216.0" y="244" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Gemini 3.8 Flash: 1 of 63 rounds, 1</title></rect>
<text x="246.6" y="259.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="279.1" y="244" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.10"><title>Gemini 3.8 Flash: 3 of 63 rounds, 2</title></rect>
<text x="309.7" y="259.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3</text>
<rect x="342.3" y="244" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.54"><title>Gemini 3.8 Flash: 17 of 63 rounds, 3</title></rect>
<text x="372.8" y="259.0" font-size="11" fill="#2c2c2c" text-anchor="middle">17</text>
<rect x="405.4" y="244" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.73"><title>Gemini 3.8 Flash: 23 of 63 rounds, 4</title></rect>
<text x="435.9" y="259.0" font-size="11" fill="#ffffff" text-anchor="middle">23</text>
<rect x="468.5" y="244" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.38"><title>Gemini 3.8 Flash: 12 of 63 rounds, 5</title></rect>
<text x="499.1" y="259.0" font-size="11" fill="#2c2c2c" text-anchor="middle">12</text>
<rect x="531.6" y="244" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Gemini 3.8 Flash: 2 of 63 rounds, 6</title></rect>
<text x="562.2" y="259.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="657.9" y="244" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.16"><title>Gemini 3.8 Flash: 5 of 63 rounds, lost</title></rect>
<text x="688.4" y="259.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5</text>
<text x="0.0" y="290.0" font-size="12" fill="#8c0000" text-anchor="start" font-weight="700">Ascension 0, the easiest</text>
<text x="0.0" y="313.0" font-size="13" fill="#2c2c2c" text-anchor="start">Gemini 3.8 Flash</text>
<rect x="279.1" y="298" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.08"><title>Gemini 3.8 Flash: 5 of 120 rounds, 2</title></rect>
<text x="309.7" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5</text>
<rect x="342.3" y="298" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.70"><title>Gemini 3.8 Flash: 42 of 120 rounds, 3</title></rect>
<text x="372.8" y="313.0" font-size="11" fill="#ffffff" text-anchor="middle">42</text>
<rect x="405.4" y="298" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.73"><title>Gemini 3.8 Flash: 44 of 120 rounds, 4</title></rect>
<text x="435.9" y="313.0" font-size="11" fill="#ffffff" text-anchor="middle">44</text>
<rect x="468.5" y="298" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.33"><title>Gemini 3.8 Flash: 20 of 120 rounds, 5</title></rect>
<text x="499.1" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">20</text>
<rect x="531.6" y="298" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Gemini 3.8 Flash: 3 of 120 rounds, 6</title></rect>
<text x="562.2" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3</text>
<rect x="594.8" y="298" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.10"><title>Gemini 3.8 Flash: 6 of 120 rounds, no solve</title></rect>
<text x="625.3" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">6</text>
<text x="0.0" y="339.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Sonnet 5.5</text>
<rect x="279.1" y="324" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.13"><title>Claude Sonnet 5.5: 7 of 106 rounds, 2</title></rect>
<text x="309.7" y="339.0" font-size="11" fill="#2c2c2c" text-anchor="middle">7</text>
<rect x="342.3" y="324" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.53"><title>Claude Sonnet 5.5: 28 of 106 rounds, 3</title></rect>
<text x="372.8" y="339.0" font-size="11" fill="#2c2c2c" text-anchor="middle">28</text>
<rect x="405.4" y="324" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.66"><title>Claude Sonnet 5.5: 35 of 106 rounds, 4</title></rect>
<text x="435.9" y="339.0" font-size="11" fill="#ffffff" text-anchor="middle">35</text>
<rect x="468.5" y="324" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.26"><title>Claude Sonnet 5.5: 14 of 106 rounds, 5</title></rect>
<text x="499.1" y="339.0" font-size="11" fill="#2c2c2c" text-anchor="middle">14</text>
<rect x="531.6" y="324" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.11"><title>Claude Sonnet 5.5: 6 of 106 rounds, 6</title></rect>
<text x="562.2" y="339.0" font-size="11" fill="#2c2c2c" text-anchor="middle">6</text>
<rect x="594.8" y="324" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.23"><title>Claude Sonnet 5.5: 12 of 106 rounds, no solve</title></rect>
<text x="625.3" y="339.0" font-size="11" fill="#2c2c2c" text-anchor="middle">12</text>
<rect x="657.9" y="324" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.08"><title>Claude Sonnet 5.5: 4 of 106 rounds, lost</title></rect>
<text x="688.4" y="339.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4</text>
<text x="0.0" y="365.0" font-size="13" fill="#2c2c2c" text-anchor="start">Step 5 Preview</text>
<rect x="342.3" y="350" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Step 5 Preview: 2 of 67 rounds, 3</title></rect>
<text x="372.8" y="365.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="405.4" y="350" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.33"><title>Step 5 Preview: 11 of 67 rounds, 4</title></rect>
<text x="435.9" y="365.0" font-size="11" fill="#2c2c2c" text-anchor="middle">11</text>
<rect x="468.5" y="350" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.39"><title>Step 5 Preview: 13 of 67 rounds, 5</title></rect>
<text x="499.1" y="365.0" font-size="11" fill="#2c2c2c" text-anchor="middle">13</text>
<rect x="531.6" y="350" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.27"><title>Step 5 Preview: 9 of 67 rounds, 6</title></rect>
<text x="562.2" y="365.0" font-size="11" fill="#2c2c2c" text-anchor="middle">9</text>
<rect x="594.8" y="350" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.81"><title>Step 5 Preview: 27 of 67 rounds, no solve</title></rect>
<text x="625.3" y="365.0" font-size="11" fill="#ffffff" text-anchor="middle">27</text>
<rect x="657.9" y="350" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.15"><title>Step 5 Preview: 5 of 67 rounds, lost</title></rect>
<text x="688.4" y="365.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5</text>
<text x="0.0" y="391.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6 Luna</text>
<rect x="342.3" y="376" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.19"><title>GPT-6 Luna: 7 of 75 rounds, 3</title></rect>
<text x="372.8" y="391.0" font-size="11" fill="#2c2c2c" text-anchor="middle">7</text>
<rect x="405.4" y="376" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.59"><title>GPT-6 Luna: 22 of 75 rounds, 4</title></rect>
<text x="435.9" y="391.0" font-size="11" fill="#2c2c2c" text-anchor="middle">22</text>
<rect x="468.5" y="376" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.64"><title>GPT-6 Luna: 24 of 75 rounds, 5</title></rect>
<text x="499.1" y="391.0" font-size="11" fill="#ffffff" text-anchor="middle">24</text>
<rect x="531.6" y="376" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.29"><title>GPT-6 Luna: 11 of 75 rounds, 6</title></rect>
<text x="562.2" y="391.0" font-size="11" fill="#2c2c2c" text-anchor="middle">11</text>
<rect x="594.8" y="376" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.16"><title>GPT-6 Luna: 6 of 75 rounds, no solve</title></rect>
<text x="625.3" y="391.0" font-size="11" fill="#2c2c2c" text-anchor="middle">6</text>
<rect x="657.9" y="376" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.13"><title>GPT-6 Luna: 5 of 75 rounds, lost</title></rect>
<text x="688.4" y="391.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5</text>
<text x="0.0" y="417.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Haiku 5.5</text>
<rect x="279.1" y="402" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.13"><title>Claude Haiku 5.5: 3 of 48 rounds, 2</title></rect>
<text x="309.7" y="417.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3</text>
<rect x="342.3" y="402" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.46"><title>Claude Haiku 5.5: 11 of 48 rounds, 3</title></rect>
<text x="372.8" y="417.0" font-size="11" fill="#2c2c2c" text-anchor="middle">11</text>
<rect x="405.4" y="402" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.38"><title>Claude Haiku 5.5: 9 of 48 rounds, 4</title></rect>
<text x="435.9" y="417.0" font-size="11" fill="#2c2c2c" text-anchor="middle">9</text>
<rect x="468.5" y="402" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.42"><title>Claude Haiku 5.5: 10 of 48 rounds, 5</title></rect>
<text x="499.1" y="417.0" font-size="11" fill="#2c2c2c" text-anchor="middle">10</text>
<rect x="531.6" y="402" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.21"><title>Claude Haiku 5.5: 5 of 48 rounds, 6</title></rect>
<text x="562.2" y="417.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5</text>
<rect x="594.8" y="402" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.21"><title>Claude Haiku 5.5: 5 of 48 rounds, no solve</title></rect>
<text x="625.3" y="417.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5</text>
<rect x="657.9" y="402" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.21"><title>Claude Haiku 5.5: 5 of 48 rounds, lost</title></rect>
<text x="688.4" y="417.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5</text>
<text x="0.0" y="443.0" font-size="13" fill="#2c2c2c" text-anchor="start">Nemotron 3 Ultra</text>
<rect x="279.1" y="428" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Nemotron 3 Ultra: 1 of 47 rounds, 2</title></rect>
<text x="309.7" y="443.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="342.3" y="428" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.43"><title>Nemotron 3 Ultra: 10 of 47 rounds, 3</title></rect>
<text x="372.8" y="443.0" font-size="11" fill="#2c2c2c" text-anchor="middle">10</text>
<rect x="405.4" y="428" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.34"><title>Nemotron 3 Ultra: 8 of 47 rounds, 4</title></rect>
<text x="435.9" y="443.0" font-size="11" fill="#2c2c2c" text-anchor="middle">8</text>
<rect x="468.5" y="428" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.55"><title>Nemotron 3 Ultra: 13 of 47 rounds, 5</title></rect>
<text x="499.1" y="443.0" font-size="11" fill="#2c2c2c" text-anchor="middle">13</text>
<rect x="531.6" y="428" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.17"><title>Nemotron 3 Ultra: 4 of 47 rounds, 6</title></rect>
<text x="562.2" y="443.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4</text>
<rect x="594.8" y="428" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.26"><title>Nemotron 3 Ultra: 6 of 47 rounds, no solve</title></rect>
<text x="625.3" y="443.0" font-size="11" fill="#2c2c2c" text-anchor="middle">6</text>
<rect x="657.9" y="428" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.21"><title>Nemotron 3 Ultra: 5 of 47 rounds, lost</title></rect>
<text x="688.4" y="443.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5</text>
<text x="0.0" y="469.0" font-size="13" fill="#2c2c2c" text-anchor="start">MiniMax M3</text>
<rect x="342.3" y="454" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.30"><title>MiniMax M3: 5 of 33 rounds, 3</title></rect>
<text x="372.8" y="469.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5</text>
<rect x="405.4" y="454" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.55"><title>MiniMax M3: 9 of 33 rounds, 4</title></rect>
<text x="435.9" y="469.0" font-size="11" fill="#2c2c2c" text-anchor="middle">9</text>
<rect x="468.5" y="454" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.61"><title>MiniMax M3: 10 of 33 rounds, 5</title></rect>
<text x="499.1" y="469.0" font-size="11" fill="#ffffff" text-anchor="middle">10</text>
<rect x="531.6" y="454" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.24"><title>MiniMax M3: 4 of 33 rounds, 6</title></rect>
<text x="562.2" y="469.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4</text>
<rect x="657.9" y="454" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.30"><title>MiniMax M3: 5 of 33 rounds, lost</title></rect>
<text x="688.4" y="469.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5</text>
<text x="215.0" y="496.0" font-size="11" fill="#8a8878" text-anchor="start">rounds by how they ended: solved in that many guesses, cleared on score without solving, or lost</text>
</svg>

### Relics

The game has 47 relics, and between them the 25 runs held 43. Each row is a relic and each column a model, with the number of its five runs that held it. No relic is in every run: the most common, Indelible, was held in 11 of 25, and only Collector, First Draft and Green Thumb were picked up by all five players. Luna tried the most, 32, partly because it sells seven of them back every run.

<svg class="results" style="width:100%;height:auto;margin:1em 0" viewBox="0 0 720 1126" role="img" aria-labelledby="relics-title" font-family="inherit">
<title id="relics-title">Every relic in the game against every model, with the number of its five runs the model held it in.</title>
<text x="157.5" y="112" font-size="11" fill="#8a8878" transform="rotate(-50 157.5 112)">Opus A10</text>
<text x="192.5" y="112" font-size="11" fill="#8a8878" transform="rotate(-50 192.5 112)">Sonnet A10</text>
<text x="227.5" y="112" font-size="11" fill="#8a8878" transform="rotate(-50 227.5 112)">GPT-6 Sol A10</text>
<text x="262.5" y="112" font-size="11" fill="#8a8878" transform="rotate(-50 262.5 112)">MiMo-V2.6-Pro A10</text>
<text x="297.5" y="112" font-size="11" fill="#8a8878" transform="rotate(-50 297.5 112)">DeepSeek V4.1 Flash A10</text>
<text x="332.5" y="112" font-size="11" fill="#8a8878" transform="rotate(-50 332.5 112)">GPT-6.1 Sol A10</text>
<text x="367.5" y="112" font-size="11" fill="#8a8878" transform="rotate(-50 367.5 112)">DeepSeek V4 Pro A10</text>
<text x="402.5" y="112" font-size="11" fill="#8a8878" transform="rotate(-50 402.5 112)">GLM A10</text>
<text x="437.5" y="112" font-size="11" fill="#8a8878" transform="rotate(-50 437.5 112)">Gemini A10</text>
<text x="472.5" y="112" font-size="11" fill="#8a8878" transform="rotate(-50 472.5 112)">Gemini A0</text>
<text x="507.5" y="112" font-size="11" fill="#8a8878" transform="rotate(-50 507.5 112)">Sonnet A0</text>
<text x="542.5" y="112" font-size="11" fill="#8a8878" transform="rotate(-50 542.5 112)">Step A0</text>
<text x="577.5" y="112" font-size="11" fill="#8a8878" transform="rotate(-50 577.5 112)">Luna A0</text>
<text x="612.5" y="112" font-size="11" fill="#8a8878" transform="rotate(-50 612.5 112)">Haiku A0</text>
<text x="647.5" y="112" font-size="11" fill="#8a8878" transform="rotate(-50 647.5 112)">Nemotron A0</text>
<text x="682.5" y="112" font-size="11" fill="#8a8878" transform="rotate(-50 682.5 112)">MiniMax A0</text>
<text x="0.0" y="133.0" font-size="12" fill="#2c2c2c" text-anchor="start">Green Thumb</text>
<rect x="141" y="120" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Opus 5.5, ascension 10: Green Thumb held in 1 of 5 runs</title></rect>
<text x="157.5" y="133.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="176" y="120" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Sonnet 5.5, ascension 10: Green Thumb held in 1 of 5 runs</title></rect>
<text x="192.5" y="133.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="211" y="120" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Sol, ascension 10: Green Thumb held in 1 of 5 runs</title></rect>
<text x="227.5" y="133.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="246" y="120" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiMo-V2.6-Pro, ascension 10: Green Thumb held in 1 of 5 runs</title></rect>
<text x="262.5" y="133.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="281" y="120" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>DeepSeek V4.1 Flash, ascension 10: Green Thumb held in 2 of 5 runs</title></rect>
<text x="297.5" y="133.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="316" y="120" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6.1 Sol, ascension 10: Green Thumb held in 1 of 5 runs</title></rect>
<text x="332.5" y="133.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="351" y="120" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>DeepSeek V4 Pro, ascension 10: Green Thumb held in 2 of 5 runs</title></rect>
<text x="367.5" y="133.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="386" y="120" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GLM 5.3 Flash, ascension 10: Green Thumb held in 1 of 5 runs</title></rect>
<text x="402.5" y="133.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="421" y="120" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Gemini 3.8 Flash, ascension 10: Green Thumb held in 2 of 5 runs</title></rect>
<text x="437.5" y="133.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="456" y="120" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 0: never held Green Thumb</title></rect>
<rect x="491" y="120" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Sonnet 5.5, ascension 0: Green Thumb held in 1 of 5 runs</title></rect>
<text x="507.5" y="133.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="526" y="120" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.66"><title>Step 5 Preview, ascension 0: Green Thumb held in 3 of 5 runs</title></rect>
<text x="542.5" y="133.0" font-size="11" fill="#ffffff" text-anchor="middle">3</text>
<rect x="561" y="120" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.66"><title>GPT-6 Luna, ascension 0: Green Thumb held in 3 of 5 runs</title></rect>
<text x="577.5" y="133.0" font-size="11" fill="#ffffff" text-anchor="middle">3</text>
<rect x="596" y="120" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Claude Haiku 5.5, ascension 0: Green Thumb held in 2 of 5 runs</title></rect>
<text x="612.5" y="133.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="631" y="120" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Nemotron 3 Ultra, ascension 0: Green Thumb held in 1 of 5 runs</title></rect>
<text x="647.5" y="133.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="666" y="120" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiniMax M3, ascension 0: Green Thumb held in 1 of 5 runs</title></rect>
<text x="682.5" y="133.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<text x="0.0" y="153.0" font-size="12" fill="#2c2c2c" text-anchor="start">Keystone</text>
<rect x="141" y="140" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Claude Opus 5.5, ascension 10: Keystone held in 2 of 5 runs</title></rect>
<text x="157.5" y="153.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="176" y="140" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Sonnet 5.5, ascension 10: Keystone held in 1 of 5 runs</title></rect>
<text x="192.5" y="153.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="211" y="140" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Sol, ascension 10: Keystone held in 1 of 5 runs</title></rect>
<text x="227.5" y="153.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="246" y="140" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>MiMo-V2.6-Pro, ascension 10: Keystone held in 2 of 5 runs</title></rect>
<text x="262.5" y="153.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="281" y="140" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>DeepSeek V4.1 Flash, ascension 10: Keystone held in 2 of 5 runs</title></rect>
<text x="297.5" y="153.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="316" y="140" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>GPT-6.1 Sol, ascension 10: Keystone held in 2 of 5 runs</title></rect>
<text x="332.5" y="153.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="351" y="140" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>DeepSeek V4 Pro, ascension 10: Keystone held in 1 of 5 runs</title></rect>
<text x="367.5" y="153.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="386" y="140" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GLM 5.3 Flash, ascension 10: never held Keystone</title></rect>
<rect x="421" y="140" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.83"><title>Gemini 3.8 Flash, ascension 10: Keystone held in 4 of 5 runs</title></rect>
<text x="437.5" y="153.0" font-size="11" fill="#ffffff" text-anchor="middle">4</text>
<rect x="456" y="140" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.66"><title>Gemini 3.8 Flash, ascension 0: Keystone held in 3 of 5 runs</title></rect>
<text x="472.5" y="153.0" font-size="11" fill="#ffffff" text-anchor="middle">3</text>
<rect x="491" y="140" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Claude Sonnet 5.5, ascension 0: Keystone held in 2 of 5 runs</title></rect>
<text x="507.5" y="153.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="526" y="140" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Step 5 Preview, ascension 0: never held Keystone</title></rect>
<rect x="561" y="140" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Luna, ascension 0: never held Keystone</title></rect>
<rect x="596" y="140" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Haiku 5.5, ascension 0: never held Keystone</title></rect>
<rect x="631" y="140" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Nemotron 3 Ultra, ascension 0: Keystone held in 2 of 5 runs</title></rect>
<text x="647.5" y="153.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="666" y="140" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiniMax M3, ascension 0: Keystone held in 1 of 5 runs</title></rect>
<text x="682.5" y="153.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<text x="0.0" y="173.0" font-size="12" fill="#2c2c2c" text-anchor="start">Speedrunner</text>
<rect x="141" y="160" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Opus 5.5, ascension 10: never held Speedrunner</title></rect>
<rect x="176" y="160" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 10: never held Speedrunner</title></rect>
<rect x="211" y="160" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Sol, ascension 10: Speedrunner held in 1 of 5 runs</title></rect>
<text x="227.5" y="173.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="246" y="160" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiMo-V2.6-Pro, ascension 10: Speedrunner held in 1 of 5 runs</title></rect>
<text x="262.5" y="173.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="281" y="160" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>DeepSeek V4.1 Flash, ascension 10: Speedrunner held in 2 of 5 runs</title></rect>
<text x="297.5" y="173.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="316" y="160" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6.1 Sol, ascension 10: Speedrunner held in 1 of 5 runs</title></rect>
<text x="332.5" y="173.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="351" y="160" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.66"><title>DeepSeek V4 Pro, ascension 10: Speedrunner held in 3 of 5 runs</title></rect>
<text x="367.5" y="173.0" font-size="11" fill="#ffffff" text-anchor="middle">3</text>
<rect x="386" y="160" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>GLM 5.3 Flash, ascension 10: Speedrunner held in 2 of 5 runs</title></rect>
<text x="402.5" y="173.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="421" y="160" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 10: never held Speedrunner</title></rect>
<rect x="456" y="160" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 0: never held Speedrunner</title></rect>
<rect x="491" y="160" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Claude Sonnet 5.5, ascension 0: Speedrunner held in 2 of 5 runs</title></rect>
<text x="507.5" y="173.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="526" y="160" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Step 5 Preview, ascension 0: Speedrunner held in 1 of 5 runs</title></rect>
<text x="542.5" y="173.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="561" y="160" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="1.00"><title>GPT-6 Luna, ascension 0: Speedrunner held in 5 of 5 runs</title></rect>
<text x="577.5" y="173.0" font-size="11" fill="#ffffff" text-anchor="middle">5</text>
<rect x="596" y="160" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.66"><title>Claude Haiku 5.5, ascension 0: Speedrunner held in 3 of 5 runs</title></rect>
<text x="612.5" y="173.0" font-size="11" fill="#ffffff" text-anchor="middle">3</text>
<rect x="631" y="160" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Nemotron 3 Ultra, ascension 0: Speedrunner held in 2 of 5 runs</title></rect>
<text x="647.5" y="173.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="666" y="160" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiniMax M3, ascension 0: never held Speedrunner</title></rect>
<text x="0.0" y="193.0" font-size="12" fill="#2c2c2c" text-anchor="start">Bloodhound</text>
<rect x="141" y="180" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Opus 5.5, ascension 10: Bloodhound held in 1 of 5 runs</title></rect>
<text x="157.5" y="193.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="176" y="180" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 10: never held Bloodhound</title></rect>
<rect x="211" y="180" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Sol, ascension 10: Bloodhound held in 1 of 5 runs</title></rect>
<text x="227.5" y="193.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="246" y="180" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>MiMo-V2.6-Pro, ascension 10: Bloodhound held in 2 of 5 runs</title></rect>
<text x="262.5" y="193.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="281" y="180" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4.1 Flash, ascension 10: never held Bloodhound</title></rect>
<rect x="316" y="180" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.66"><title>GPT-6.1 Sol, ascension 10: Bloodhound held in 3 of 5 runs</title></rect>
<text x="332.5" y="193.0" font-size="11" fill="#ffffff" text-anchor="middle">3</text>
<rect x="351" y="180" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4 Pro, ascension 10: never held Bloodhound</title></rect>
<rect x="386" y="180" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.83"><title>GLM 5.3 Flash, ascension 10: Bloodhound held in 4 of 5 runs</title></rect>
<text x="402.5" y="193.0" font-size="11" fill="#ffffff" text-anchor="middle">4</text>
<rect x="421" y="180" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Gemini 3.8 Flash, ascension 10: Bloodhound held in 1 of 5 runs</title></rect>
<text x="437.5" y="193.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="456" y="180" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Gemini 3.8 Flash, ascension 0: Bloodhound held in 2 of 5 runs</title></rect>
<text x="472.5" y="193.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="491" y="180" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Sonnet 5.5, ascension 0: Bloodhound held in 1 of 5 runs</title></rect>
<text x="507.5" y="193.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="526" y="180" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Step 5 Preview, ascension 0: Bloodhound held in 1 of 5 runs</title></rect>
<text x="542.5" y="193.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="561" y="180" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.66"><title>GPT-6 Luna, ascension 0: Bloodhound held in 3 of 5 runs</title></rect>
<text x="577.5" y="193.0" font-size="11" fill="#ffffff" text-anchor="middle">3</text>
<rect x="596" y="180" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Haiku 5.5, ascension 0: Bloodhound held in 1 of 5 runs</title></rect>
<text x="612.5" y="193.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="631" y="180" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Nemotron 3 Ultra, ascension 0: Bloodhound held in 1 of 5 runs</title></rect>
<text x="647.5" y="193.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="666" y="180" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiniMax M3, ascension 0: Bloodhound held in 1 of 5 runs</title></rect>
<text x="682.5" y="193.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<text x="0.0" y="213.0" font-size="12" fill="#2c2c2c" text-anchor="start">Fresh Ink</text>
<rect x="141" y="200" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Opus 5.5, ascension 10: never held Fresh Ink</title></rect>
<rect x="176" y="200" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Claude Sonnet 5.5, ascension 10: Fresh Ink held in 2 of 5 runs</title></rect>
<text x="192.5" y="213.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="211" y="200" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Sol, ascension 10: Fresh Ink held in 1 of 5 runs</title></rect>
<text x="227.5" y="213.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="246" y="200" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiMo-V2.6-Pro, ascension 10: Fresh Ink held in 1 of 5 runs</title></rect>
<text x="262.5" y="213.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="281" y="200" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.66"><title>DeepSeek V4.1 Flash, ascension 10: Fresh Ink held in 3 of 5 runs</title></rect>
<text x="297.5" y="213.0" font-size="11" fill="#ffffff" text-anchor="middle">3</text>
<rect x="316" y="200" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6.1 Sol, ascension 10: Fresh Ink held in 1 of 5 runs</title></rect>
<text x="332.5" y="213.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="351" y="200" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4 Pro, ascension 10: never held Fresh Ink</title></rect>
<rect x="386" y="200" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>GLM 5.3 Flash, ascension 10: Fresh Ink held in 2 of 5 runs</title></rect>
<text x="402.5" y="213.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="421" y="200" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 10: never held Fresh Ink</title></rect>
<rect x="456" y="200" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 0: never held Fresh Ink</title></rect>
<rect x="491" y="200" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Claude Sonnet 5.5, ascension 0: Fresh Ink held in 2 of 5 runs</title></rect>
<text x="507.5" y="213.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="526" y="200" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Step 5 Preview, ascension 0: Fresh Ink held in 2 of 5 runs</title></rect>
<text x="542.5" y="213.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="561" y="200" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.83"><title>GPT-6 Luna, ascension 0: Fresh Ink held in 4 of 5 runs</title></rect>
<text x="577.5" y="213.0" font-size="11" fill="#ffffff" text-anchor="middle">4</text>
<rect x="596" y="200" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Claude Haiku 5.5, ascension 0: Fresh Ink held in 2 of 5 runs</title></rect>
<text x="612.5" y="213.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="631" y="200" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Nemotron 3 Ultra, ascension 0: Fresh Ink held in 1 of 5 runs</title></rect>
<text x="647.5" y="213.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="666" y="200" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiniMax M3, ascension 0: never held Fresh Ink</title></rect>
<text x="0.0" y="233.0" font-size="12" fill="#2c2c2c" text-anchor="start">Collector</text>
<rect x="141" y="220" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Claude Opus 5.5, ascension 10: Collector held in 2 of 5 runs</title></rect>
<text x="157.5" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="176" y="220" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.66"><title>Claude Sonnet 5.5, ascension 10: Collector held in 3 of 5 runs</title></rect>
<text x="192.5" y="233.0" font-size="11" fill="#ffffff" text-anchor="middle">3</text>
<rect x="211" y="220" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Sol, ascension 10: Collector held in 1 of 5 runs</title></rect>
<text x="227.5" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="246" y="220" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiMo-V2.6-Pro, ascension 10: Collector held in 1 of 5 runs</title></rect>
<text x="262.5" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="281" y="220" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>DeepSeek V4.1 Flash, ascension 10: Collector held in 1 of 5 runs</title></rect>
<text x="297.5" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="316" y="220" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6.1 Sol, ascension 10: never held Collector</title></rect>
<rect x="351" y="220" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>DeepSeek V4 Pro, ascension 10: Collector held in 2 of 5 runs</title></rect>
<text x="367.5" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="386" y="220" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GLM 5.3 Flash, ascension 10: Collector held in 1 of 5 runs</title></rect>
<text x="402.5" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="421" y="220" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Gemini 3.8 Flash, ascension 10: Collector held in 1 of 5 runs</title></rect>
<text x="437.5" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="456" y="220" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Gemini 3.8 Flash, ascension 0: Collector held in 1 of 5 runs</title></rect>
<text x="472.5" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="491" y="220" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.66"><title>Claude Sonnet 5.5, ascension 0: Collector held in 3 of 5 runs</title></rect>
<text x="507.5" y="233.0" font-size="11" fill="#ffffff" text-anchor="middle">3</text>
<rect x="526" y="220" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Step 5 Preview, ascension 0: Collector held in 1 of 5 runs</title></rect>
<text x="542.5" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="561" y="220" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Luna, ascension 0: Collector held in 1 of 5 runs</title></rect>
<text x="577.5" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="596" y="220" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Haiku 5.5, ascension 0: Collector held in 1 of 5 runs</title></rect>
<text x="612.5" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="631" y="220" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Nemotron 3 Ultra, ascension 0: Collector held in 1 of 5 runs</title></rect>
<text x="647.5" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="666" y="220" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiniMax M3, ascension 0: never held Collector</title></rect>
<text x="0.0" y="253.0" font-size="12" fill="#2c2c2c" text-anchor="start">Indelible</text>
<rect x="141" y="240" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Claude Opus 5.5, ascension 10: Indelible held in 2 of 5 runs</title></rect>
<text x="157.5" y="253.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="176" y="240" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.83"><title>Claude Sonnet 5.5, ascension 10: Indelible held in 4 of 5 runs</title></rect>
<text x="192.5" y="253.0" font-size="11" fill="#ffffff" text-anchor="middle">4</text>
<rect x="211" y="240" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>GPT-6 Sol, ascension 10: Indelible held in 2 of 5 runs</title></rect>
<text x="227.5" y="253.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="246" y="240" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiMo-V2.6-Pro, ascension 10: never held Indelible</title></rect>
<rect x="281" y="240" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>DeepSeek V4.1 Flash, ascension 10: Indelible held in 1 of 5 runs</title></rect>
<text x="297.5" y="253.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="316" y="240" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6.1 Sol, ascension 10: never held Indelible</title></rect>
<rect x="351" y="240" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4 Pro, ascension 10: never held Indelible</title></rect>
<rect x="386" y="240" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GLM 5.3 Flash, ascension 10: Indelible held in 1 of 5 runs</title></rect>
<text x="402.5" y="253.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="421" y="240" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Gemini 3.8 Flash, ascension 10: Indelible held in 1 of 5 runs</title></rect>
<text x="437.5" y="253.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="456" y="240" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Gemini 3.8 Flash, ascension 0: Indelible held in 2 of 5 runs</title></rect>
<text x="472.5" y="253.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="491" y="240" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.83"><title>Claude Sonnet 5.5, ascension 0: Indelible held in 4 of 5 runs</title></rect>
<text x="507.5" y="253.0" font-size="11" fill="#ffffff" text-anchor="middle">4</text>
<rect x="526" y="240" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Step 5 Preview, ascension 0: Indelible held in 1 of 5 runs</title></rect>
<text x="542.5" y="253.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="561" y="240" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Luna, ascension 0: never held Indelible</title></rect>
<rect x="596" y="240" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Haiku 5.5, ascension 0: Indelible held in 1 of 5 runs</title></rect>
<text x="612.5" y="253.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="631" y="240" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Nemotron 3 Ultra, ascension 0: Indelible held in 1 of 5 runs</title></rect>
<text x="647.5" y="253.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="666" y="240" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiniMax M3, ascension 0: never held Indelible</title></rect>
<text x="0.0" y="273.0" font-size="12" fill="#2c2c2c" text-anchor="start">Snowball</text>
<rect x="141" y="260" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Claude Opus 5.5, ascension 10: Snowball held in 2 of 5 runs</title></rect>
<text x="157.5" y="273.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="176" y="260" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 10: never held Snowball</title></rect>
<rect x="211" y="260" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Sol, ascension 10: Snowball held in 1 of 5 runs</title></rect>
<text x="227.5" y="273.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="246" y="260" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiMo-V2.6-Pro, ascension 10: Snowball held in 1 of 5 runs</title></rect>
<text x="262.5" y="273.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="281" y="260" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>DeepSeek V4.1 Flash, ascension 10: Snowball held in 1 of 5 runs</title></rect>
<text x="297.5" y="273.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="316" y="260" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6.1 Sol, ascension 10: Snowball held in 1 of 5 runs</title></rect>
<text x="332.5" y="273.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="351" y="260" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>DeepSeek V4 Pro, ascension 10: Snowball held in 2 of 5 runs</title></rect>
<text x="367.5" y="273.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="386" y="260" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GLM 5.3 Flash, ascension 10: Snowball held in 1 of 5 runs</title></rect>
<text x="402.5" y="273.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="421" y="260" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Gemini 3.8 Flash, ascension 10: Snowball held in 2 of 5 runs</title></rect>
<text x="437.5" y="273.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="456" y="260" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Gemini 3.8 Flash, ascension 0: Snowball held in 1 of 5 runs</title></rect>
<text x="472.5" y="273.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="491" y="260" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Sonnet 5.5, ascension 0: Snowball held in 1 of 5 runs</title></rect>
<text x="507.5" y="273.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="526" y="260" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Step 5 Preview, ascension 0: Snowball held in 2 of 5 runs</title></rect>
<text x="542.5" y="273.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="561" y="260" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>GPT-6 Luna, ascension 0: Snowball held in 2 of 5 runs</title></rect>
<text x="577.5" y="273.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="596" y="260" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Haiku 5.5, ascension 0: Snowball held in 1 of 5 runs</title></rect>
<text x="612.5" y="273.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="631" y="260" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Nemotron 3 Ultra, ascension 0: Snowball held in 1 of 5 runs</title></rect>
<text x="647.5" y="273.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="666" y="260" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiniMax M3, ascension 0: Snowball held in 1 of 5 runs</title></rect>
<text x="682.5" y="273.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<text x="0.0" y="293.0" font-size="12" fill="#2c2c2c" text-anchor="start">Greedy Grammarian</text>
<rect x="141" y="280" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Opus 5.5, ascension 10: never held Greedy Grammarian</title></rect>
<rect x="176" y="280" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.83"><title>Claude Sonnet 5.5, ascension 10: Greedy Grammarian held in 4 of 5 runs</title></rect>
<text x="192.5" y="293.0" font-size="11" fill="#ffffff" text-anchor="middle">4</text>
<rect x="211" y="280" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Sol, ascension 10: never held Greedy Grammarian</title></rect>
<rect x="246" y="280" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.66"><title>MiMo-V2.6-Pro, ascension 10: Greedy Grammarian held in 3 of 5 runs</title></rect>
<text x="262.5" y="293.0" font-size="11" fill="#ffffff" text-anchor="middle">3</text>
<rect x="281" y="280" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>DeepSeek V4.1 Flash, ascension 10: Greedy Grammarian held in 2 of 5 runs</title></rect>
<text x="297.5" y="293.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="316" y="280" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6.1 Sol, ascension 10: Greedy Grammarian held in 1 of 5 runs</title></rect>
<text x="332.5" y="293.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="351" y="280" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>DeepSeek V4 Pro, ascension 10: Greedy Grammarian held in 1 of 5 runs</title></rect>
<text x="367.5" y="293.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="386" y="280" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GLM 5.3 Flash, ascension 10: Greedy Grammarian held in 1 of 5 runs</title></rect>
<text x="402.5" y="293.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="421" y="280" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Gemini 3.8 Flash, ascension 10: Greedy Grammarian held in 1 of 5 runs</title></rect>
<text x="437.5" y="293.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="456" y="280" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Gemini 3.8 Flash, ascension 0: Greedy Grammarian held in 1 of 5 runs</title></rect>
<text x="472.5" y="293.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="491" y="280" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Sonnet 5.5, ascension 0: Greedy Grammarian held in 1 of 5 runs</title></rect>
<text x="507.5" y="293.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="526" y="280" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Step 5 Preview, ascension 0: never held Greedy Grammarian</title></rect>
<rect x="561" y="280" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Luna, ascension 0: Greedy Grammarian held in 1 of 5 runs</title></rect>
<text x="577.5" y="293.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="596" y="280" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Haiku 5.5, ascension 0: Greedy Grammarian held in 1 of 5 runs</title></rect>
<text x="612.5" y="293.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="631" y="280" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Nemotron 3 Ultra, ascension 0: Greedy Grammarian held in 1 of 5 runs</title></rect>
<text x="647.5" y="293.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="666" y="280" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiniMax M3, ascension 0: Greedy Grammarian held in 1 of 5 runs</title></rect>
<text x="682.5" y="293.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<text x="0.0" y="313.0" font-size="12" fill="#2c2c2c" text-anchor="start">Sunk Cost</text>
<rect x="141" y="300" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Opus 5.5, ascension 10: never held Sunk Cost</title></rect>
<rect x="176" y="300" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Sonnet 5.5, ascension 10: Sunk Cost held in 1 of 5 runs</title></rect>
<text x="192.5" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="211" y="300" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Sol, ascension 10: Sunk Cost held in 1 of 5 runs</title></rect>
<text x="227.5" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="246" y="300" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiMo-V2.6-Pro, ascension 10: Sunk Cost held in 1 of 5 runs</title></rect>
<text x="262.5" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="281" y="300" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>DeepSeek V4.1 Flash, ascension 10: Sunk Cost held in 2 of 5 runs</title></rect>
<text x="297.5" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="316" y="300" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6.1 Sol, ascension 10: never held Sunk Cost</title></rect>
<rect x="351" y="300" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>DeepSeek V4 Pro, ascension 10: Sunk Cost held in 1 of 5 runs</title></rect>
<text x="367.5" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="386" y="300" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GLM 5.3 Flash, ascension 10: Sunk Cost held in 1 of 5 runs</title></rect>
<text x="402.5" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="421" y="300" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Gemini 3.8 Flash, ascension 10: Sunk Cost held in 2 of 5 runs</title></rect>
<text x="437.5" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="456" y="300" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Gemini 3.8 Flash, ascension 0: Sunk Cost held in 1 of 5 runs</title></rect>
<text x="472.5" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="491" y="300" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Claude Sonnet 5.5, ascension 0: Sunk Cost held in 2 of 5 runs</title></rect>
<text x="507.5" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="526" y="300" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Step 5 Preview, ascension 0: Sunk Cost held in 2 of 5 runs</title></rect>
<text x="542.5" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="561" y="300" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>GPT-6 Luna, ascension 0: Sunk Cost held in 2 of 5 runs</title></rect>
<text x="577.5" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="596" y="300" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Haiku 5.5, ascension 0: Sunk Cost held in 1 of 5 runs</title></rect>
<text x="612.5" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="631" y="300" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Nemotron 3 Ultra, ascension 0: Sunk Cost held in 1 of 5 runs</title></rect>
<text x="647.5" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="666" y="300" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiniMax M3, ascension 0: Sunk Cost held in 1 of 5 runs</title></rect>
<text x="682.5" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<text x="0.0" y="333.0" font-size="12" fill="#2c2c2c" text-anchor="start">First Draft</text>
<rect x="141" y="320" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Opus 5.5, ascension 10: First Draft held in 1 of 5 runs</title></rect>
<text x="157.5" y="333.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="176" y="320" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Claude Sonnet 5.5, ascension 10: First Draft held in 2 of 5 runs</title></rect>
<text x="192.5" y="333.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="211" y="320" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>GPT-6 Sol, ascension 10: First Draft held in 2 of 5 runs</title></rect>
<text x="227.5" y="333.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="246" y="320" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiMo-V2.6-Pro, ascension 10: never held First Draft</title></rect>
<rect x="281" y="320" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>DeepSeek V4.1 Flash, ascension 10: First Draft held in 1 of 5 runs</title></rect>
<text x="297.5" y="333.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="316" y="320" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6.1 Sol, ascension 10: never held First Draft</title></rect>
<rect x="351" y="320" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>DeepSeek V4 Pro, ascension 10: First Draft held in 1 of 5 runs</title></rect>
<text x="367.5" y="333.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="386" y="320" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GLM 5.3 Flash, ascension 10: First Draft held in 1 of 5 runs</title></rect>
<text x="402.5" y="333.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="421" y="320" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Gemini 3.8 Flash, ascension 10: First Draft held in 1 of 5 runs</title></rect>
<text x="437.5" y="333.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="456" y="320" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 0: never held First Draft</title></rect>
<rect x="491" y="320" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Claude Sonnet 5.5, ascension 0: First Draft held in 2 of 5 runs</title></rect>
<text x="507.5" y="333.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="526" y="320" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Step 5 Preview, ascension 0: First Draft held in 1 of 5 runs</title></rect>
<text x="542.5" y="333.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="561" y="320" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>GPT-6 Luna, ascension 0: First Draft held in 2 of 5 runs</title></rect>
<text x="577.5" y="333.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="596" y="320" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Claude Haiku 5.5, ascension 0: First Draft held in 2 of 5 runs</title></rect>
<text x="612.5" y="333.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="631" y="320" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Nemotron 3 Ultra, ascension 0: First Draft held in 1 of 5 runs</title></rect>
<text x="647.5" y="333.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="666" y="320" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiniMax M3, ascension 0: never held First Draft</title></rect>
<text x="0.0" y="353.0" font-size="12" fill="#2c2c2c" text-anchor="start">Loaded Dice</text>
<rect x="141" y="340" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Opus 5.5, ascension 10: Loaded Dice held in 1 of 5 runs</title></rect>
<text x="157.5" y="353.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="176" y="340" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Claude Sonnet 5.5, ascension 10: Loaded Dice held in 2 of 5 runs</title></rect>
<text x="192.5" y="353.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="211" y="340" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Sol, ascension 10: never held Loaded Dice</title></rect>
<rect x="246" y="340" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>MiMo-V2.6-Pro, ascension 10: Loaded Dice held in 2 of 5 runs</title></rect>
<text x="262.5" y="353.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="281" y="340" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4.1 Flash, ascension 10: never held Loaded Dice</title></rect>
<rect x="316" y="340" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6.1 Sol, ascension 10: Loaded Dice held in 1 of 5 runs</title></rect>
<text x="332.5" y="353.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="351" y="340" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4 Pro, ascension 10: never held Loaded Dice</title></rect>
<rect x="386" y="340" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>GLM 5.3 Flash, ascension 10: Loaded Dice held in 2 of 5 runs</title></rect>
<text x="402.5" y="353.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="421" y="340" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Gemini 3.8 Flash, ascension 10: Loaded Dice held in 1 of 5 runs</title></rect>
<text x="437.5" y="353.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="456" y="340" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Gemini 3.8 Flash, ascension 0: Loaded Dice held in 1 of 5 runs</title></rect>
<text x="472.5" y="353.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="491" y="340" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.66"><title>Claude Sonnet 5.5, ascension 0: Loaded Dice held in 3 of 5 runs</title></rect>
<text x="507.5" y="353.0" font-size="11" fill="#ffffff" text-anchor="middle">3</text>
<rect x="526" y="340" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Step 5 Preview, ascension 0: Loaded Dice held in 2 of 5 runs</title></rect>
<text x="542.5" y="353.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="561" y="340" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Luna, ascension 0: Loaded Dice held in 1 of 5 runs</title></rect>
<text x="577.5" y="353.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="596" y="340" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Haiku 5.5, ascension 0: never held Loaded Dice</title></rect>
<rect x="631" y="340" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Nemotron 3 Ultra, ascension 0: Loaded Dice held in 1 of 5 runs</title></rect>
<text x="647.5" y="353.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="666" y="340" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiniMax M3, ascension 0: never held Loaded Dice</title></rect>
<text x="0.0" y="373.0" font-size="12" fill="#2c2c2c" text-anchor="start">No Maybes</text>
<rect x="141" y="360" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Opus 5.5, ascension 10: No Maybes held in 1 of 5 runs</title></rect>
<text x="157.5" y="373.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="176" y="360" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 10: never held No Maybes</title></rect>
<rect x="211" y="360" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Sol, ascension 10: No Maybes held in 1 of 5 runs</title></rect>
<text x="227.5" y="373.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="246" y="360" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiMo-V2.6-Pro, ascension 10: never held No Maybes</title></rect>
<rect x="281" y="360" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4.1 Flash, ascension 10: never held No Maybes</title></rect>
<rect x="316" y="360" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>GPT-6.1 Sol, ascension 10: No Maybes held in 2 of 5 runs</title></rect>
<text x="332.5" y="373.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="351" y="360" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>DeepSeek V4 Pro, ascension 10: No Maybes held in 1 of 5 runs</title></rect>
<text x="367.5" y="373.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="386" y="360" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GLM 5.3 Flash, ascension 10: never held No Maybes</title></rect>
<rect x="421" y="360" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Gemini 3.8 Flash, ascension 10: No Maybes held in 2 of 5 runs</title></rect>
<text x="437.5" y="373.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="456" y="360" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.66"><title>Gemini 3.8 Flash, ascension 0: No Maybes held in 3 of 5 runs</title></rect>
<text x="472.5" y="373.0" font-size="11" fill="#ffffff" text-anchor="middle">3</text>
<rect x="491" y="360" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.83"><title>Claude Sonnet 5.5, ascension 0: No Maybes held in 4 of 5 runs</title></rect>
<text x="507.5" y="373.0" font-size="11" fill="#ffffff" text-anchor="middle">4</text>
<rect x="526" y="360" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Step 5 Preview, ascension 0: No Maybes held in 1 of 5 runs</title></rect>
<text x="542.5" y="373.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="561" y="360" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Luna, ascension 0: No Maybes held in 1 of 5 runs</title></rect>
<text x="577.5" y="373.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="596" y="360" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Haiku 5.5, ascension 0: No Maybes held in 1 of 5 runs</title></rect>
<text x="612.5" y="373.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="631" y="360" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Nemotron 3 Ultra, ascension 0: never held No Maybes</title></rect>
<rect x="666" y="360" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiniMax M3, ascension 0: never held No Maybes</title></rect>
<text x="0.0" y="393.0" font-size="12" fill="#2c2c2c" text-anchor="start">Cold Open</text>
<rect x="141" y="380" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Opus 5.5, ascension 10: Cold Open held in 1 of 5 runs</title></rect>
<text x="157.5" y="393.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="176" y="380" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Sonnet 5.5, ascension 10: Cold Open held in 1 of 5 runs</title></rect>
<text x="192.5" y="393.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="211" y="380" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Sol, ascension 10: Cold Open held in 1 of 5 runs</title></rect>
<text x="227.5" y="393.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="246" y="380" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiMo-V2.6-Pro, ascension 10: Cold Open held in 1 of 5 runs</title></rect>
<text x="262.5" y="393.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="281" y="380" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4.1 Flash, ascension 10: never held Cold Open</title></rect>
<rect x="316" y="380" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6.1 Sol, ascension 10: Cold Open held in 1 of 5 runs</title></rect>
<text x="332.5" y="393.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="351" y="380" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>DeepSeek V4 Pro, ascension 10: Cold Open held in 1 of 5 runs</title></rect>
<text x="367.5" y="393.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="386" y="380" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>GLM 5.3 Flash, ascension 10: Cold Open held in 2 of 5 runs</title></rect>
<text x="402.5" y="393.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="421" y="380" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 10: never held Cold Open</title></rect>
<rect x="456" y="380" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Gemini 3.8 Flash, ascension 0: Cold Open held in 1 of 5 runs</title></rect>
<text x="472.5" y="393.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="491" y="380" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 0: never held Cold Open</title></rect>
<rect x="526" y="380" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Step 5 Preview, ascension 0: Cold Open held in 1 of 5 runs</title></rect>
<text x="542.5" y="393.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="561" y="380" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Luna, ascension 0: Cold Open held in 1 of 5 runs</title></rect>
<text x="577.5" y="393.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="596" y="380" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Haiku 5.5, ascension 0: never held Cold Open</title></rect>
<rect x="631" y="380" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.66"><title>Nemotron 3 Ultra, ascension 0: Cold Open held in 3 of 5 runs</title></rect>
<text x="647.5" y="393.0" font-size="11" fill="#ffffff" text-anchor="middle">3</text>
<rect x="666" y="380" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiniMax M3, ascension 0: Cold Open held in 1 of 5 runs</title></rect>
<text x="682.5" y="393.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<text x="0.0" y="413.0" font-size="12" fill="#2c2c2c" text-anchor="start">The Long Game</text>
<rect x="141" y="400" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Claude Opus 5.5, ascension 10: The Long Game held in 2 of 5 runs</title></rect>
<text x="157.5" y="413.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="176" y="400" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Sonnet 5.5, ascension 10: The Long Game held in 1 of 5 runs</title></rect>
<text x="192.5" y="413.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="211" y="400" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Sol, ascension 10: never held The Long Game</title></rect>
<rect x="246" y="400" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiMo-V2.6-Pro, ascension 10: The Long Game held in 1 of 5 runs</title></rect>
<text x="262.5" y="413.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="281" y="400" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4.1 Flash, ascension 10: never held The Long Game</title></rect>
<rect x="316" y="400" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6.1 Sol, ascension 10: The Long Game held in 1 of 5 runs</title></rect>
<text x="332.5" y="413.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="351" y="400" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4 Pro, ascension 10: never held The Long Game</title></rect>
<rect x="386" y="400" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GLM 5.3 Flash, ascension 10: The Long Game held in 1 of 5 runs</title></rect>
<text x="402.5" y="413.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="421" y="400" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Gemini 3.8 Flash, ascension 10: The Long Game held in 1 of 5 runs</title></rect>
<text x="437.5" y="413.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="456" y="400" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Gemini 3.8 Flash, ascension 0: The Long Game held in 1 of 5 runs</title></rect>
<text x="472.5" y="413.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="491" y="400" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Sonnet 5.5, ascension 0: The Long Game held in 1 of 5 runs</title></rect>
<text x="507.5" y="413.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="526" y="400" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Step 5 Preview, ascension 0: The Long Game held in 1 of 5 runs</title></rect>
<text x="542.5" y="413.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="561" y="400" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.66"><title>GPT-6 Luna, ascension 0: The Long Game held in 3 of 5 runs</title></rect>
<text x="577.5" y="413.0" font-size="11" fill="#ffffff" text-anchor="middle">3</text>
<rect x="596" y="400" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Haiku 5.5, ascension 0: never held The Long Game</title></rect>
<rect x="631" y="400" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Nemotron 3 Ultra, ascension 0: The Long Game held in 1 of 5 runs</title></rect>
<text x="647.5" y="413.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="666" y="400" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiniMax M3, ascension 0: The Long Game held in 1 of 5 runs</title></rect>
<text x="682.5" y="413.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<text x="0.0" y="433.0" font-size="12" fill="#2c2c2c" text-anchor="start">Vowel Hoarder</text>
<rect x="141" y="420" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Opus 5.5, ascension 10: never held Vowel Hoarder</title></rect>
<rect x="176" y="420" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Claude Sonnet 5.5, ascension 10: Vowel Hoarder held in 2 of 5 runs</title></rect>
<text x="192.5" y="433.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="211" y="420" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Sol, ascension 10: never held Vowel Hoarder</title></rect>
<rect x="246" y="420" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiMo-V2.6-Pro, ascension 10: Vowel Hoarder held in 1 of 5 runs</title></rect>
<text x="262.5" y="433.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="281" y="420" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4.1 Flash, ascension 10: never held Vowel Hoarder</title></rect>
<rect x="316" y="420" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6.1 Sol, ascension 10: Vowel Hoarder held in 1 of 5 runs</title></rect>
<text x="332.5" y="433.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="351" y="420" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>DeepSeek V4 Pro, ascension 10: Vowel Hoarder held in 1 of 5 runs</title></rect>
<text x="367.5" y="433.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="386" y="420" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>GLM 5.3 Flash, ascension 10: Vowel Hoarder held in 2 of 5 runs</title></rect>
<text x="402.5" y="433.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="421" y="420" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 10: never held Vowel Hoarder</title></rect>
<rect x="456" y="420" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Gemini 3.8 Flash, ascension 0: Vowel Hoarder held in 2 of 5 runs</title></rect>
<text x="472.5" y="433.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="491" y="420" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Claude Sonnet 5.5, ascension 0: Vowel Hoarder held in 2 of 5 runs</title></rect>
<text x="507.5" y="433.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="526" y="420" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Step 5 Preview, ascension 0: Vowel Hoarder held in 1 of 5 runs</title></rect>
<text x="542.5" y="433.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="561" y="420" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Luna, ascension 0: Vowel Hoarder held in 1 of 5 runs</title></rect>
<text x="577.5" y="433.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="596" y="420" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Haiku 5.5, ascension 0: Vowel Hoarder held in 1 of 5 runs</title></rect>
<text x="612.5" y="433.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="631" y="420" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Nemotron 3 Ultra, ascension 0: Vowel Hoarder held in 1 of 5 runs</title></rect>
<text x="647.5" y="433.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="666" y="420" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiniMax M3, ascension 0: never held Vowel Hoarder</title></rect>
<text x="0.0" y="453.0" font-size="12" fill="#2c2c2c" text-anchor="start">Hot Streak</text>
<rect x="141" y="440" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Opus 5.5, ascension 10: Hot Streak held in 1 of 5 runs</title></rect>
<text x="157.5" y="453.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="176" y="440" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Sonnet 5.5, ascension 10: Hot Streak held in 1 of 5 runs</title></rect>
<text x="192.5" y="453.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="211" y="440" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Sol, ascension 10: Hot Streak held in 1 of 5 runs</title></rect>
<text x="227.5" y="453.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="246" y="440" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiMo-V2.6-Pro, ascension 10: Hot Streak held in 1 of 5 runs</title></rect>
<text x="262.5" y="453.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="281" y="440" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>DeepSeek V4.1 Flash, ascension 10: Hot Streak held in 1 of 5 runs</title></rect>
<text x="297.5" y="453.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="316" y="440" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6.1 Sol, ascension 10: never held Hot Streak</title></rect>
<rect x="351" y="440" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>DeepSeek V4 Pro, ascension 10: Hot Streak held in 1 of 5 runs</title></rect>
<text x="367.5" y="453.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="386" y="440" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GLM 5.3 Flash, ascension 10: never held Hot Streak</title></rect>
<rect x="421" y="440" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 10: never held Hot Streak</title></rect>
<rect x="456" y="440" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Gemini 3.8 Flash, ascension 0: Hot Streak held in 1 of 5 runs</title></rect>
<text x="472.5" y="453.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="491" y="440" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 0: never held Hot Streak</title></rect>
<rect x="526" y="440" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Step 5 Preview, ascension 0: Hot Streak held in 1 of 5 runs</title></rect>
<text x="542.5" y="453.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="561" y="440" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Luna, ascension 0: Hot Streak held in 1 of 5 runs</title></rect>
<text x="577.5" y="453.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="596" y="440" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Haiku 5.5, ascension 0: never held Hot Streak</title></rect>
<rect x="631" y="440" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Nemotron 3 Ultra, ascension 0: Hot Streak held in 1 of 5 runs</title></rect>
<text x="647.5" y="453.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="666" y="440" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>MiniMax M3, ascension 0: Hot Streak held in 2 of 5 runs</title></rect>
<text x="682.5" y="453.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<text x="0.0" y="473.0" font-size="12" fill="#2c2c2c" text-anchor="start">Scavenger</text>
<rect x="141" y="460" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Opus 5.5, ascension 10: Scavenger held in 1 of 5 runs</title></rect>
<text x="157.5" y="473.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="176" y="460" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Sonnet 5.5, ascension 10: Scavenger held in 1 of 5 runs</title></rect>
<text x="192.5" y="473.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="211" y="460" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Sol, ascension 10: never held Scavenger</title></rect>
<rect x="246" y="460" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiMo-V2.6-Pro, ascension 10: never held Scavenger</title></rect>
<rect x="281" y="460" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4.1 Flash, ascension 10: never held Scavenger</title></rect>
<rect x="316" y="460" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>GPT-6.1 Sol, ascension 10: Scavenger held in 2 of 5 runs</title></rect>
<text x="332.5" y="473.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="351" y="460" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>DeepSeek V4 Pro, ascension 10: Scavenger held in 2 of 5 runs</title></rect>
<text x="367.5" y="473.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="386" y="460" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GLM 5.3 Flash, ascension 10: never held Scavenger</title></rect>
<rect x="421" y="460" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Gemini 3.8 Flash, ascension 10: Scavenger held in 1 of 5 runs</title></rect>
<text x="437.5" y="473.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="456" y="460" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Gemini 3.8 Flash, ascension 0: Scavenger held in 2 of 5 runs</title></rect>
<text x="472.5" y="473.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="491" y="460" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 0: never held Scavenger</title></rect>
<rect x="526" y="460" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Step 5 Preview, ascension 0: never held Scavenger</title></rect>
<rect x="561" y="460" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Luna, ascension 0: Scavenger held in 1 of 5 runs</title></rect>
<text x="577.5" y="473.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="596" y="460" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Haiku 5.5, ascension 0: Scavenger held in 1 of 5 runs</title></rect>
<text x="612.5" y="473.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="631" y="460" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Nemotron 3 Ultra, ascension 0: never held Scavenger</title></rect>
<rect x="666" y="460" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiniMax M3, ascension 0: Scavenger held in 1 of 5 runs</title></rect>
<text x="682.5" y="473.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<text x="0.0" y="493.0" font-size="12" fill="#2c2c2c" text-anchor="start">Head Start</text>
<rect x="141" y="480" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Opus 5.5, ascension 10: Head Start held in 1 of 5 runs</title></rect>
<text x="157.5" y="493.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="176" y="480" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 10: never held Head Start</title></rect>
<rect x="211" y="480" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>GPT-6 Sol, ascension 10: Head Start held in 2 of 5 runs</title></rect>
<text x="227.5" y="493.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="246" y="480" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiMo-V2.6-Pro, ascension 10: Head Start held in 1 of 5 runs</title></rect>
<text x="262.5" y="493.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="281" y="480" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4.1 Flash, ascension 10: never held Head Start</title></rect>
<rect x="316" y="480" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6.1 Sol, ascension 10: never held Head Start</title></rect>
<rect x="351" y="480" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>DeepSeek V4 Pro, ascension 10: Head Start held in 1 of 5 runs</title></rect>
<text x="367.5" y="493.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="386" y="480" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GLM 5.3 Flash, ascension 10: Head Start held in 1 of 5 runs</title></rect>
<text x="402.5" y="493.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="421" y="480" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 10: never held Head Start</title></rect>
<rect x="456" y="480" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 0: never held Head Start</title></rect>
<rect x="491" y="480" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 0: never held Head Start</title></rect>
<rect x="526" y="480" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Step 5 Preview, ascension 0: Head Start held in 2 of 5 runs</title></rect>
<text x="542.5" y="493.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="561" y="480" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>GPT-6 Luna, ascension 0: Head Start held in 2 of 5 runs</title></rect>
<text x="577.5" y="493.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="596" y="480" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Haiku 5.5, ascension 0: never held Head Start</title></rect>
<rect x="631" y="480" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Nemotron 3 Ultra, ascension 0: Head Start held in 1 of 5 runs</title></rect>
<text x="647.5" y="493.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="666" y="480" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiniMax M3, ascension 0: never held Head Start</title></rect>
<text x="0.0" y="513.0" font-size="12" fill="#2c2c2c" text-anchor="start">Lexicographer</text>
<rect x="141" y="500" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Opus 5.5, ascension 10: never held Lexicographer</title></rect>
<rect x="176" y="500" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Sonnet 5.5, ascension 10: Lexicographer held in 1 of 5 runs</title></rect>
<text x="192.5" y="513.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="211" y="500" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Sol, ascension 10: Lexicographer held in 1 of 5 runs</title></rect>
<text x="227.5" y="513.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="246" y="500" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiMo-V2.6-Pro, ascension 10: never held Lexicographer</title></rect>
<rect x="281" y="500" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4.1 Flash, ascension 10: never held Lexicographer</title></rect>
<rect x="316" y="500" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>GPT-6.1 Sol, ascension 10: Lexicographer held in 2 of 5 runs</title></rect>
<text x="332.5" y="513.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="351" y="500" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4 Pro, ascension 10: never held Lexicographer</title></rect>
<rect x="386" y="500" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GLM 5.3 Flash, ascension 10: never held Lexicographer</title></rect>
<rect x="421" y="500" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 10: never held Lexicographer</title></rect>
<rect x="456" y="500" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 0: never held Lexicographer</title></rect>
<rect x="491" y="500" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.66"><title>Claude Sonnet 5.5, ascension 0: Lexicographer held in 3 of 5 runs</title></rect>
<text x="507.5" y="513.0" font-size="11" fill="#ffffff" text-anchor="middle">3</text>
<rect x="526" y="500" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Step 5 Preview, ascension 0: never held Lexicographer</title></rect>
<rect x="561" y="500" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>GPT-6 Luna, ascension 0: Lexicographer held in 2 of 5 runs</title></rect>
<text x="577.5" y="513.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="596" y="500" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Haiku 5.5, ascension 0: Lexicographer held in 1 of 5 runs</title></rect>
<text x="612.5" y="513.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="631" y="500" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Nemotron 3 Ultra, ascension 0: never held Lexicographer</title></rect>
<rect x="666" y="500" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiniMax M3, ascension 0: never held Lexicographer</title></rect>
<text x="0.0" y="533.0" font-size="12" fill="#2c2c2c" text-anchor="start">Second Wind</text>
<rect x="141" y="520" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.83"><title>Claude Opus 5.5, ascension 10: Second Wind held in 4 of 5 runs</title></rect>
<text x="157.5" y="533.0" font-size="11" fill="#ffffff" text-anchor="middle">4</text>
<rect x="176" y="520" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Claude Sonnet 5.5, ascension 10: Second Wind held in 2 of 5 runs</title></rect>
<text x="192.5" y="533.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="211" y="520" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Sol, ascension 10: never held Second Wind</title></rect>
<rect x="246" y="520" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiMo-V2.6-Pro, ascension 10: Second Wind held in 1 of 5 runs</title></rect>
<text x="262.5" y="533.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="281" y="520" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4.1 Flash, ascension 10: never held Second Wind</title></rect>
<rect x="316" y="520" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6.1 Sol, ascension 10: never held Second Wind</title></rect>
<rect x="351" y="520" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>DeepSeek V4 Pro, ascension 10: Second Wind held in 1 of 5 runs</title></rect>
<text x="367.5" y="533.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="386" y="520" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GLM 5.3 Flash, ascension 10: Second Wind held in 1 of 5 runs</title></rect>
<text x="402.5" y="533.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="421" y="520" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 10: never held Second Wind</title></rect>
<rect x="456" y="520" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 0: never held Second Wind</title></rect>
<rect x="491" y="520" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Sonnet 5.5, ascension 0: Second Wind held in 1 of 5 runs</title></rect>
<text x="507.5" y="533.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="526" y="520" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Step 5 Preview, ascension 0: never held Second Wind</title></rect>
<rect x="561" y="520" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Luna, ascension 0: never held Second Wind</title></rect>
<rect x="596" y="520" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Haiku 5.5, ascension 0: never held Second Wind</title></rect>
<rect x="631" y="520" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Nemotron 3 Ultra, ascension 0: never held Second Wind</title></rect>
<rect x="666" y="520" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiniMax M3, ascension 0: never held Second Wind</title></rect>
<text x="0.0" y="553.0" font-size="12" fill="#2c2c2c" text-anchor="start">First Impression</text>
<rect x="141" y="540" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Opus 5.5, ascension 10: First Impression held in 1 of 5 runs</title></rect>
<text x="157.5" y="553.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="176" y="540" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Sonnet 5.5, ascension 10: First Impression held in 1 of 5 runs</title></rect>
<text x="192.5" y="553.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="211" y="540" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Sol, ascension 10: never held First Impression</title></rect>
<rect x="246" y="540" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiMo-V2.6-Pro, ascension 10: First Impression held in 1 of 5 runs</title></rect>
<text x="262.5" y="553.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="281" y="540" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>DeepSeek V4.1 Flash, ascension 10: First Impression held in 1 of 5 runs</title></rect>
<text x="297.5" y="553.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="316" y="540" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6.1 Sol, ascension 10: First Impression held in 1 of 5 runs</title></rect>
<text x="332.5" y="553.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="351" y="540" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>DeepSeek V4 Pro, ascension 10: First Impression held in 1 of 5 runs</title></rect>
<text x="367.5" y="553.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="386" y="540" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GLM 5.3 Flash, ascension 10: never held First Impression</title></rect>
<rect x="421" y="540" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Gemini 3.8 Flash, ascension 10: First Impression held in 1 of 5 runs</title></rect>
<text x="437.5" y="553.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="456" y="540" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Gemini 3.8 Flash, ascension 0: First Impression held in 1 of 5 runs</title></rect>
<text x="472.5" y="553.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="491" y="540" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Sonnet 5.5, ascension 0: First Impression held in 1 of 5 runs</title></rect>
<text x="507.5" y="553.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="526" y="540" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Step 5 Preview, ascension 0: never held First Impression</title></rect>
<rect x="561" y="540" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Luna, ascension 0: never held First Impression</title></rect>
<rect x="596" y="540" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Haiku 5.5, ascension 0: never held First Impression</title></rect>
<rect x="631" y="540" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Nemotron 3 Ultra, ascension 0: never held First Impression</title></rect>
<rect x="666" y="540" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiniMax M3, ascension 0: never held First Impression</title></rect>
<text x="0.0" y="573.0" font-size="12" fill="#2c2c2c" text-anchor="start">Slow Burn</text>
<rect x="141" y="560" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Opus 5.5, ascension 10: never held Slow Burn</title></rect>
<rect x="176" y="560" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 10: never held Slow Burn</title></rect>
<rect x="211" y="560" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>GPT-6 Sol, ascension 10: Slow Burn held in 2 of 5 runs</title></rect>
<text x="227.5" y="573.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="246" y="560" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiMo-V2.6-Pro, ascension 10: never held Slow Burn</title></rect>
<rect x="281" y="560" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>DeepSeek V4.1 Flash, ascension 10: Slow Burn held in 1 of 5 runs</title></rect>
<text x="297.5" y="573.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="316" y="560" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6.1 Sol, ascension 10: never held Slow Burn</title></rect>
<rect x="351" y="560" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4 Pro, ascension 10: never held Slow Burn</title></rect>
<rect x="386" y="560" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GLM 5.3 Flash, ascension 10: never held Slow Burn</title></rect>
<rect x="421" y="560" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 10: never held Slow Burn</title></rect>
<rect x="456" y="560" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 0: never held Slow Burn</title></rect>
<rect x="491" y="560" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 0: never held Slow Burn</title></rect>
<rect x="526" y="560" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Step 5 Preview, ascension 0: Slow Burn held in 2 of 5 runs</title></rect>
<text x="542.5" y="573.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="561" y="560" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>GPT-6 Luna, ascension 0: Slow Burn held in 2 of 5 runs</title></rect>
<text x="577.5" y="573.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="596" y="560" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Haiku 5.5, ascension 0: Slow Burn held in 1 of 5 runs</title></rect>
<text x="612.5" y="573.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="631" y="560" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Nemotron 3 Ultra, ascension 0: never held Slow Burn</title></rect>
<rect x="666" y="560" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiniMax M3, ascension 0: Slow Burn held in 1 of 5 runs</title></rect>
<text x="682.5" y="573.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<text x="0.0" y="593.0" font-size="12" fill="#2c2c2c" text-anchor="start">Stipend</text>
<rect x="141" y="580" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Opus 5.5, ascension 10: Stipend held in 1 of 5 runs</title></rect>
<text x="157.5" y="593.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="176" y="580" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Sonnet 5.5, ascension 10: Stipend held in 1 of 5 runs</title></rect>
<text x="192.5" y="593.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="211" y="580" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Sol, ascension 10: never held Stipend</title></rect>
<rect x="246" y="580" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiMo-V2.6-Pro, ascension 10: Stipend held in 1 of 5 runs</title></rect>
<text x="262.5" y="593.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="281" y="580" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4.1 Flash, ascension 10: never held Stipend</title></rect>
<rect x="316" y="580" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6.1 Sol, ascension 10: never held Stipend</title></rect>
<rect x="351" y="580" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>DeepSeek V4 Pro, ascension 10: Stipend held in 1 of 5 runs</title></rect>
<text x="367.5" y="593.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="386" y="580" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GLM 5.3 Flash, ascension 10: never held Stipend</title></rect>
<rect x="421" y="580" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Gemini 3.8 Flash, ascension 10: Stipend held in 1 of 5 runs</title></rect>
<text x="437.5" y="593.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="456" y="580" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Gemini 3.8 Flash, ascension 0: Stipend held in 1 of 5 runs</title></rect>
<text x="472.5" y="593.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="491" y="580" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 0: never held Stipend</title></rect>
<rect x="526" y="580" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Step 5 Preview, ascension 0: never held Stipend</title></rect>
<rect x="561" y="580" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Luna, ascension 0: Stipend held in 1 of 5 runs</title></rect>
<text x="577.5" y="593.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="596" y="580" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Haiku 5.5, ascension 0: Stipend held in 1 of 5 runs</title></rect>
<text x="612.5" y="593.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="631" y="580" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Nemotron 3 Ultra, ascension 0: Stipend held in 1 of 5 runs</title></rect>
<text x="647.5" y="593.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="666" y="580" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiniMax M3, ascension 0: never held Stipend</title></rect>
<text x="0.0" y="613.0" font-size="12" fill="#2c2c2c" text-anchor="start">The Mint</text>
<rect x="141" y="600" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Opus 5.5, ascension 10: The Mint held in 1 of 5 runs</title></rect>
<text x="157.5" y="613.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="176" y="600" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 10: never held The Mint</title></rect>
<rect x="211" y="600" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Sol, ascension 10: never held The Mint</title></rect>
<rect x="246" y="600" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiMo-V2.6-Pro, ascension 10: The Mint held in 1 of 5 runs</title></rect>
<text x="262.5" y="613.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="281" y="600" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>DeepSeek V4.1 Flash, ascension 10: The Mint held in 1 of 5 runs</title></rect>
<text x="297.5" y="613.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="316" y="600" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6.1 Sol, ascension 10: The Mint held in 1 of 5 runs</title></rect>
<text x="332.5" y="613.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="351" y="600" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4 Pro, ascension 10: never held The Mint</title></rect>
<rect x="386" y="600" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GLM 5.3 Flash, ascension 10: The Mint held in 1 of 5 runs</title></rect>
<text x="402.5" y="613.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="421" y="600" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 10: never held The Mint</title></rect>
<rect x="456" y="600" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 0: never held The Mint</title></rect>
<rect x="491" y="600" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 0: never held The Mint</title></rect>
<rect x="526" y="600" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Step 5 Preview, ascension 0: never held The Mint</title></rect>
<rect x="561" y="600" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Luna, ascension 0: The Mint held in 1 of 5 runs</title></rect>
<text x="577.5" y="613.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="596" y="600" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Haiku 5.5, ascension 0: The Mint held in 1 of 5 runs</title></rect>
<text x="612.5" y="613.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="631" y="600" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Nemotron 3 Ultra, ascension 0: never held The Mint</title></rect>
<rect x="666" y="600" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiniMax M3, ascension 0: The Mint held in 1 of 5 runs</title></rect>
<text x="682.5" y="613.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<text x="0.0" y="633.0" font-size="12" fill="#2c2c2c" text-anchor="start">Twins</text>
<rect x="141" y="620" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Opus 5.5, ascension 10: never held Twins</title></rect>
<rect x="176" y="620" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 10: never held Twins</title></rect>
<rect x="211" y="620" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Sol, ascension 10: never held Twins</title></rect>
<rect x="246" y="620" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>MiMo-V2.6-Pro, ascension 10: Twins held in 2 of 5 runs</title></rect>
<text x="262.5" y="633.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="281" y="620" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4.1 Flash, ascension 10: never held Twins</title></rect>
<rect x="316" y="620" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6.1 Sol, ascension 10: Twins held in 1 of 5 runs</title></rect>
<text x="332.5" y="633.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="351" y="620" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4 Pro, ascension 10: never held Twins</title></rect>
<rect x="386" y="620" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GLM 5.3 Flash, ascension 10: Twins held in 1 of 5 runs</title></rect>
<text x="402.5" y="633.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="421" y="620" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 10: never held Twins</title></rect>
<rect x="456" y="620" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 0: never held Twins</title></rect>
<rect x="491" y="620" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 0: never held Twins</title></rect>
<rect x="526" y="620" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Step 5 Preview, ascension 0: Twins held in 1 of 5 runs</title></rect>
<text x="542.5" y="633.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="561" y="620" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.66"><title>GPT-6 Luna, ascension 0: Twins held in 3 of 5 runs</title></rect>
<text x="577.5" y="633.0" font-size="11" fill="#ffffff" text-anchor="middle">3</text>
<rect x="596" y="620" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Haiku 5.5, ascension 0: never held Twins</title></rect>
<rect x="631" y="620" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Nemotron 3 Ultra, ascension 0: never held Twins</title></rect>
<rect x="666" y="620" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiniMax M3, ascension 0: never held Twins</title></rect>
<text x="0.0" y="653.0" font-size="12" fill="#2c2c2c" text-anchor="start">Anagrammer</text>
<rect x="141" y="640" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Opus 5.5, ascension 10: never held Anagrammer</title></rect>
<rect x="176" y="640" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Claude Sonnet 5.5, ascension 10: Anagrammer held in 2 of 5 runs</title></rect>
<text x="192.5" y="653.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="211" y="640" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Sol, ascension 10: Anagrammer held in 1 of 5 runs</title></rect>
<text x="227.5" y="653.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="246" y="640" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiMo-V2.6-Pro, ascension 10: never held Anagrammer</title></rect>
<rect x="281" y="640" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4.1 Flash, ascension 10: never held Anagrammer</title></rect>
<rect x="316" y="640" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6.1 Sol, ascension 10: never held Anagrammer</title></rect>
<rect x="351" y="640" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>DeepSeek V4 Pro, ascension 10: Anagrammer held in 2 of 5 runs</title></rect>
<text x="367.5" y="653.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="386" y="640" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GLM 5.3 Flash, ascension 10: never held Anagrammer</title></rect>
<rect x="421" y="640" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 10: never held Anagrammer</title></rect>
<rect x="456" y="640" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Gemini 3.8 Flash, ascension 0: Anagrammer held in 2 of 5 runs</title></rect>
<text x="472.5" y="653.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="491" y="640" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 0: never held Anagrammer</title></rect>
<rect x="526" y="640" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Step 5 Preview, ascension 0: never held Anagrammer</title></rect>
<rect x="561" y="640" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Luna, ascension 0: never held Anagrammer</title></rect>
<rect x="596" y="640" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Haiku 5.5, ascension 0: never held Anagrammer</title></rect>
<rect x="631" y="640" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Nemotron 3 Ultra, ascension 0: never held Anagrammer</title></rect>
<rect x="666" y="640" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiniMax M3, ascension 0: never held Anagrammer</title></rect>
<text x="0.0" y="673.0" font-size="12" fill="#2c2c2c" text-anchor="start">Consonant Cluster</text>
<rect x="141" y="660" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Opus 5.5, ascension 10: never held Consonant Cluster</title></rect>
<rect x="176" y="660" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Sonnet 5.5, ascension 10: Consonant Cluster held in 1 of 5 runs</title></rect>
<text x="192.5" y="673.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="211" y="660" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Sol, ascension 10: never held Consonant Cluster</title></rect>
<rect x="246" y="660" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiMo-V2.6-Pro, ascension 10: never held Consonant Cluster</title></rect>
<rect x="281" y="660" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>DeepSeek V4.1 Flash, ascension 10: Consonant Cluster held in 1 of 5 runs</title></rect>
<text x="297.5" y="673.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="316" y="660" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6.1 Sol, ascension 10: never held Consonant Cluster</title></rect>
<rect x="351" y="660" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>DeepSeek V4 Pro, ascension 10: Consonant Cluster held in 1 of 5 runs</title></rect>
<text x="367.5" y="673.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="386" y="660" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GLM 5.3 Flash, ascension 10: never held Consonant Cluster</title></rect>
<rect x="421" y="660" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 10: never held Consonant Cluster</title></rect>
<rect x="456" y="660" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 0: never held Consonant Cluster</title></rect>
<rect x="491" y="660" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 0: never held Consonant Cluster</title></rect>
<rect x="526" y="660" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Step 5 Preview, ascension 0: never held Consonant Cluster</title></rect>
<rect x="561" y="660" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Luna, ascension 0: Consonant Cluster held in 1 of 5 runs</title></rect>
<text x="577.5" y="673.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="596" y="660" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Haiku 5.5, ascension 0: Consonant Cluster held in 1 of 5 runs</title></rect>
<text x="612.5" y="673.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="631" y="660" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Nemotron 3 Ultra, ascension 0: Consonant Cluster held in 1 of 5 runs</title></rect>
<text x="647.5" y="673.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="666" y="660" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiniMax M3, ascension 0: Consonant Cluster held in 1 of 5 runs</title></rect>
<text x="682.5" y="673.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<text x="0.0" y="693.0" font-size="12" fill="#2c2c2c" text-anchor="start">Masochist</text>
<rect x="141" y="680" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Opus 5.5, ascension 10: never held Masochist</title></rect>
<rect x="176" y="680" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Sonnet 5.5, ascension 10: Masochist held in 1 of 5 runs</title></rect>
<text x="192.5" y="693.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="211" y="680" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Sol, ascension 10: Masochist held in 1 of 5 runs</title></rect>
<text x="227.5" y="693.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="246" y="680" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>MiMo-V2.6-Pro, ascension 10: Masochist held in 2 of 5 runs</title></rect>
<text x="262.5" y="693.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="281" y="680" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4.1 Flash, ascension 10: never held Masochist</title></rect>
<rect x="316" y="680" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6.1 Sol, ascension 10: never held Masochist</title></rect>
<rect x="351" y="680" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>DeepSeek V4 Pro, ascension 10: Masochist held in 1 of 5 runs</title></rect>
<text x="367.5" y="693.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="386" y="680" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GLM 5.3 Flash, ascension 10: never held Masochist</title></rect>
<rect x="421" y="680" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 10: never held Masochist</title></rect>
<rect x="456" y="680" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Gemini 3.8 Flash, ascension 0: Masochist held in 1 of 5 runs</title></rect>
<text x="472.5" y="693.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="491" y="680" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 0: never held Masochist</title></rect>
<rect x="526" y="680" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Step 5 Preview, ascension 0: never held Masochist</title></rect>
<rect x="561" y="680" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Luna, ascension 0: Masochist held in 1 of 5 runs</title></rect>
<text x="577.5" y="693.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="596" y="680" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Haiku 5.5, ascension 0: never held Masochist</title></rect>
<rect x="631" y="680" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Nemotron 3 Ultra, ascension 0: never held Masochist</title></rect>
<rect x="666" y="680" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiniMax M3, ascension 0: never held Masochist</title></rect>
<text x="0.0" y="713.0" font-size="12" fill="#2c2c2c" text-anchor="start">The Vault</text>
<rect x="141" y="700" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Claude Opus 5.5, ascension 10: The Vault held in 2 of 5 runs</title></rect>
<text x="157.5" y="713.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="176" y="700" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Sonnet 5.5, ascension 10: The Vault held in 1 of 5 runs</title></rect>
<text x="192.5" y="713.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="211" y="700" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Sol, ascension 10: never held The Vault</title></rect>
<rect x="246" y="700" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiMo-V2.6-Pro, ascension 10: never held The Vault</title></rect>
<rect x="281" y="700" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4.1 Flash, ascension 10: never held The Vault</title></rect>
<rect x="316" y="700" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>GPT-6.1 Sol, ascension 10: The Vault held in 2 of 5 runs</title></rect>
<text x="332.5" y="713.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="351" y="700" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4 Pro, ascension 10: never held The Vault</title></rect>
<rect x="386" y="700" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GLM 5.3 Flash, ascension 10: The Vault held in 1 of 5 runs</title></rect>
<text x="402.5" y="713.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="421" y="700" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 10: never held The Vault</title></rect>
<rect x="456" y="700" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 0: never held The Vault</title></rect>
<rect x="491" y="700" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 0: never held The Vault</title></rect>
<rect x="526" y="700" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Step 5 Preview, ascension 0: never held The Vault</title></rect>
<rect x="561" y="700" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Luna, ascension 0: The Vault held in 1 of 5 runs</title></rect>
<text x="577.5" y="713.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="596" y="700" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Haiku 5.5, ascension 0: never held The Vault</title></rect>
<rect x="631" y="700" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Nemotron 3 Ultra, ascension 0: never held The Vault</title></rect>
<rect x="666" y="700" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiniMax M3, ascension 0: never held The Vault</title></rect>
<text x="0.0" y="733.0" font-size="12" fill="#2c2c2c" text-anchor="start">Second Look</text>
<rect x="141" y="720" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Opus 5.5, ascension 10: never held Second Look</title></rect>
<rect x="176" y="720" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 10: never held Second Look</title></rect>
<rect x="211" y="720" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Sol, ascension 10: never held Second Look</title></rect>
<rect x="246" y="720" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiMo-V2.6-Pro, ascension 10: Second Look held in 1 of 5 runs</title></rect>
<text x="262.5" y="733.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="281" y="720" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4.1 Flash, ascension 10: never held Second Look</title></rect>
<rect x="316" y="720" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6.1 Sol, ascension 10: Second Look held in 1 of 5 runs</title></rect>
<text x="332.5" y="733.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="351" y="720" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4 Pro, ascension 10: never held Second Look</title></rect>
<rect x="386" y="720" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GLM 5.3 Flash, ascension 10: never held Second Look</title></rect>
<rect x="421" y="720" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 10: never held Second Look</title></rect>
<rect x="456" y="720" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Gemini 3.8 Flash, ascension 0: Second Look held in 1 of 5 runs</title></rect>
<text x="472.5" y="733.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="491" y="720" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 0: never held Second Look</title></rect>
<rect x="526" y="720" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Step 5 Preview, ascension 0: Second Look held in 1 of 5 runs</title></rect>
<text x="542.5" y="733.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="561" y="720" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Luna, ascension 0: Second Look held in 1 of 5 runs</title></rect>
<text x="577.5" y="733.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="596" y="720" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Haiku 5.5, ascension 0: never held Second Look</title></rect>
<rect x="631" y="720" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Nemotron 3 Ultra, ascension 0: never held Second Look</title></rect>
<rect x="666" y="720" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiniMax M3, ascension 0: Second Look held in 1 of 5 runs</title></rect>
<text x="682.5" y="733.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<text x="0.0" y="753.0" font-size="12" fill="#2c2c2c" text-anchor="start">Force of Habit</text>
<rect x="141" y="740" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Opus 5.5, ascension 10: Force of Habit held in 1 of 5 runs</title></rect>
<text x="157.5" y="753.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="176" y="740" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 10: never held Force of Habit</title></rect>
<rect x="211" y="740" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Sol, ascension 10: never held Force of Habit</title></rect>
<rect x="246" y="740" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiMo-V2.6-Pro, ascension 10: never held Force of Habit</title></rect>
<rect x="281" y="740" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4.1 Flash, ascension 10: never held Force of Habit</title></rect>
<rect x="316" y="740" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6.1 Sol, ascension 10: Force of Habit held in 1 of 5 runs</title></rect>
<text x="332.5" y="753.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="351" y="740" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4 Pro, ascension 10: never held Force of Habit</title></rect>
<rect x="386" y="740" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GLM 5.3 Flash, ascension 10: Force of Habit held in 1 of 5 runs</title></rect>
<text x="402.5" y="753.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="421" y="740" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 10: never held Force of Habit</title></rect>
<rect x="456" y="740" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 0: never held Force of Habit</title></rect>
<rect x="491" y="740" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 0: never held Force of Habit</title></rect>
<rect x="526" y="740" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Step 5 Preview, ascension 0: Force of Habit held in 1 of 5 runs</title></rect>
<text x="542.5" y="753.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="561" y="740" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Luna, ascension 0: never held Force of Habit</title></rect>
<rect x="596" y="740" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Haiku 5.5, ascension 0: never held Force of Habit</title></rect>
<rect x="631" y="740" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Nemotron 3 Ultra, ascension 0: never held Force of Habit</title></rect>
<rect x="666" y="740" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiniMax M3, ascension 0: Force of Habit held in 1 of 5 runs</title></rect>
<text x="682.5" y="753.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<text x="0.0" y="773.0" font-size="12" fill="#2c2c2c" text-anchor="start">Royalties</text>
<rect x="141" y="760" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Opus 5.5, ascension 10: never held Royalties</title></rect>
<rect x="176" y="760" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Sonnet 5.5, ascension 10: Royalties held in 1 of 5 runs</title></rect>
<text x="192.5" y="773.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="211" y="760" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Sol, ascension 10: never held Royalties</title></rect>
<rect x="246" y="760" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiMo-V2.6-Pro, ascension 10: never held Royalties</title></rect>
<rect x="281" y="760" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4.1 Flash, ascension 10: never held Royalties</title></rect>
<rect x="316" y="760" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6.1 Sol, ascension 10: never held Royalties</title></rect>
<rect x="351" y="760" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4 Pro, ascension 10: never held Royalties</title></rect>
<rect x="386" y="760" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GLM 5.3 Flash, ascension 10: never held Royalties</title></rect>
<rect x="421" y="760" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 10: never held Royalties</title></rect>
<rect x="456" y="760" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 0: never held Royalties</title></rect>
<rect x="491" y="760" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 0: never held Royalties</title></rect>
<rect x="526" y="760" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Step 5 Preview, ascension 0: never held Royalties</title></rect>
<rect x="561" y="760" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.66"><title>GPT-6 Luna, ascension 0: Royalties held in 3 of 5 runs</title></rect>
<text x="577.5" y="773.0" font-size="11" fill="#ffffff" text-anchor="middle">3</text>
<rect x="596" y="760" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Haiku 5.5, ascension 0: never held Royalties</title></rect>
<rect x="631" y="760" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Nemotron 3 Ultra, ascension 0: Royalties held in 1 of 5 runs</title></rect>
<text x="647.5" y="773.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="666" y="760" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiniMax M3, ascension 0: never held Royalties</title></rect>
<text x="0.0" y="793.0" font-size="12" fill="#2c2c2c" text-anchor="start">Doppelgänger</text>
<rect x="141" y="780" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Opus 5.5, ascension 10: never held Doppelgänger</title></rect>
<rect x="176" y="780" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 10: never held Doppelgänger</title></rect>
<rect x="211" y="780" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Sol, ascension 10: never held Doppelgänger</title></rect>
<rect x="246" y="780" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiMo-V2.6-Pro, ascension 10: never held Doppelgänger</title></rect>
<rect x="281" y="780" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4.1 Flash, ascension 10: never held Doppelgänger</title></rect>
<rect x="316" y="780" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6.1 Sol, ascension 10: Doppelgänger held in 1 of 5 runs</title></rect>
<text x="332.5" y="793.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="351" y="780" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4 Pro, ascension 10: never held Doppelgänger</title></rect>
<rect x="386" y="780" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GLM 5.3 Flash, ascension 10: never held Doppelgänger</title></rect>
<rect x="421" y="780" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 10: never held Doppelgänger</title></rect>
<rect x="456" y="780" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 0: never held Doppelgänger</title></rect>
<rect x="491" y="780" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 0: never held Doppelgänger</title></rect>
<rect x="526" y="780" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Step 5 Preview, ascension 0: Doppelgänger held in 1 of 5 runs</title></rect>
<text x="542.5" y="793.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="561" y="780" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Luna, ascension 0: never held Doppelgänger</title></rect>
<rect x="596" y="780" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Haiku 5.5, ascension 0: Doppelgänger held in 1 of 5 runs</title></rect>
<text x="612.5" y="793.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="631" y="780" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Nemotron 3 Ultra, ascension 0: never held Doppelgänger</title></rect>
<rect x="666" y="780" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiniMax M3, ascension 0: Doppelgänger held in 1 of 5 runs</title></rect>
<text x="682.5" y="793.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<text x="0.0" y="813.0" font-size="12" fill="#2c2c2c" text-anchor="start">Pyromaniac</text>
<rect x="141" y="800" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Opus 5.5, ascension 10: Pyromaniac held in 1 of 5 runs</title></rect>
<text x="157.5" y="813.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="176" y="800" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 10: never held Pyromaniac</title></rect>
<rect x="211" y="800" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Sol, ascension 10: never held Pyromaniac</title></rect>
<rect x="246" y="800" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiMo-V2.6-Pro, ascension 10: never held Pyromaniac</title></rect>
<rect x="281" y="800" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4.1 Flash, ascension 10: never held Pyromaniac</title></rect>
<rect x="316" y="800" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6.1 Sol, ascension 10: never held Pyromaniac</title></rect>
<rect x="351" y="800" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4 Pro, ascension 10: never held Pyromaniac</title></rect>
<rect x="386" y="800" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GLM 5.3 Flash, ascension 10: never held Pyromaniac</title></rect>
<rect x="421" y="800" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 10: never held Pyromaniac</title></rect>
<rect x="456" y="800" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 0: never held Pyromaniac</title></rect>
<rect x="491" y="800" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Claude Sonnet 5.5, ascension 0: Pyromaniac held in 2 of 5 runs</title></rect>
<text x="507.5" y="813.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="526" y="800" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Step 5 Preview, ascension 0: never held Pyromaniac</title></rect>
<rect x="561" y="800" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Luna, ascension 0: Pyromaniac held in 1 of 5 runs</title></rect>
<text x="577.5" y="813.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="596" y="800" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Haiku 5.5, ascension 0: never held Pyromaniac</title></rect>
<rect x="631" y="800" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Nemotron 3 Ultra, ascension 0: never held Pyromaniac</title></rect>
<rect x="666" y="800" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiniMax M3, ascension 0: never held Pyromaniac</title></rect>
<text x="0.0" y="833.0" font-size="12" fill="#2c2c2c" text-anchor="start">Reserve</text>
<rect x="141" y="820" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Opus 5.5, ascension 10: never held Reserve</title></rect>
<rect x="176" y="820" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Sonnet 5.5, ascension 10: Reserve held in 1 of 5 runs</title></rect>
<text x="192.5" y="833.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="211" y="820" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Sol, ascension 10: never held Reserve</title></rect>
<rect x="246" y="820" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>MiMo-V2.6-Pro, ascension 10: Reserve held in 1 of 5 runs</title></rect>
<text x="262.5" y="833.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="281" y="820" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4.1 Flash, ascension 10: never held Reserve</title></rect>
<rect x="316" y="820" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6.1 Sol, ascension 10: never held Reserve</title></rect>
<rect x="351" y="820" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4 Pro, ascension 10: never held Reserve</title></rect>
<rect x="386" y="820" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GLM 5.3 Flash, ascension 10: never held Reserve</title></rect>
<rect x="421" y="820" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Gemini 3.8 Flash, ascension 10: Reserve held in 1 of 5 runs</title></rect>
<text x="437.5" y="833.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="456" y="820" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 0: never held Reserve</title></rect>
<rect x="491" y="820" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Sonnet 5.5, ascension 0: Reserve held in 1 of 5 runs</title></rect>
<text x="507.5" y="833.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="526" y="820" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Step 5 Preview, ascension 0: never held Reserve</title></rect>
<rect x="561" y="820" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Luna, ascension 0: never held Reserve</title></rect>
<rect x="596" y="820" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Haiku 5.5, ascension 0: never held Reserve</title></rect>
<rect x="631" y="820" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Nemotron 3 Ultra, ascension 0: never held Reserve</title></rect>
<rect x="666" y="820" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiniMax M3, ascension 0: never held Reserve</title></rect>
<text x="0.0" y="853.0" font-size="12" fill="#2c2c2c" text-anchor="start">Candle</text>
<rect x="141" y="840" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Opus 5.5, ascension 10: never held Candle</title></rect>
<rect x="176" y="840" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 10: never held Candle</title></rect>
<rect x="211" y="840" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Sol, ascension 10: never held Candle</title></rect>
<rect x="246" y="840" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiMo-V2.6-Pro, ascension 10: never held Candle</title></rect>
<rect x="281" y="840" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4.1 Flash, ascension 10: never held Candle</title></rect>
<rect x="316" y="840" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6.1 Sol, ascension 10: never held Candle</title></rect>
<rect x="351" y="840" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4 Pro, ascension 10: never held Candle</title></rect>
<rect x="386" y="840" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GLM 5.3 Flash, ascension 10: Candle held in 1 of 5 runs</title></rect>
<text x="402.5" y="853.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="421" y="840" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 10: never held Candle</title></rect>
<rect x="456" y="840" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 0: never held Candle</title></rect>
<rect x="491" y="840" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 0: never held Candle</title></rect>
<rect x="526" y="840" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Step 5 Preview, ascension 0: Candle held in 1 of 5 runs</title></rect>
<text x="542.5" y="853.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="561" y="840" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Luna, ascension 0: Candle held in 1 of 5 runs</title></rect>
<text x="577.5" y="853.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="596" y="840" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Haiku 5.5, ascension 0: never held Candle</title></rect>
<rect x="631" y="840" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Nemotron 3 Ultra, ascension 0: never held Candle</title></rect>
<rect x="666" y="840" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiniMax M3, ascension 0: never held Candle</title></rect>
<text x="0.0" y="873.0" font-size="12" fill="#2c2c2c" text-anchor="start">Q's Bargain</text>
<rect x="141" y="860" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Opus 5.5, ascension 10: never held Q's Bargain</title></rect>
<rect x="176" y="860" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 10: never held Q's Bargain</title></rect>
<rect x="211" y="860" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Sol, ascension 10: never held Q's Bargain</title></rect>
<rect x="246" y="860" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiMo-V2.6-Pro, ascension 10: never held Q's Bargain</title></rect>
<rect x="281" y="860" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4.1 Flash, ascension 10: never held Q's Bargain</title></rect>
<rect x="316" y="860" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6.1 Sol, ascension 10: never held Q's Bargain</title></rect>
<rect x="351" y="860" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4 Pro, ascension 10: never held Q's Bargain</title></rect>
<rect x="386" y="860" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GLM 5.3 Flash, ascension 10: never held Q's Bargain</title></rect>
<rect x="421" y="860" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 10: never held Q's Bargain</title></rect>
<rect x="456" y="860" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 0: never held Q's Bargain</title></rect>
<rect x="491" y="860" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 0: never held Q's Bargain</title></rect>
<rect x="526" y="860" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Step 5 Preview, ascension 0: Q's Bargain held in 1 of 5 runs</title></rect>
<text x="542.5" y="873.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="561" y="860" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>GPT-6 Luna, ascension 0: Q's Bargain held in 2 of 5 runs</title></rect>
<text x="577.5" y="873.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="596" y="860" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Haiku 5.5, ascension 0: never held Q's Bargain</title></rect>
<rect x="631" y="860" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Nemotron 3 Ultra, ascension 0: never held Q's Bargain</title></rect>
<rect x="666" y="860" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiniMax M3, ascension 0: never held Q's Bargain</title></rect>
<text x="0.0" y="893.0" font-size="12" fill="#2c2c2c" text-anchor="start">Patron</text>
<rect x="141" y="880" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Opus 5.5, ascension 10: never held Patron</title></rect>
<rect x="176" y="880" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 10: never held Patron</title></rect>
<rect x="211" y="880" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Sol, ascension 10: never held Patron</title></rect>
<rect x="246" y="880" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiMo-V2.6-Pro, ascension 10: never held Patron</title></rect>
<rect x="281" y="880" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4.1 Flash, ascension 10: never held Patron</title></rect>
<rect x="316" y="880" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6.1 Sol, ascension 10: never held Patron</title></rect>
<rect x="351" y="880" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4 Pro, ascension 10: never held Patron</title></rect>
<rect x="386" y="880" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GLM 5.3 Flash, ascension 10: never held Patron</title></rect>
<rect x="421" y="880" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 10: never held Patron</title></rect>
<rect x="456" y="880" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 0: never held Patron</title></rect>
<rect x="491" y="880" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 0: never held Patron</title></rect>
<rect x="526" y="880" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Step 5 Preview, ascension 0: never held Patron</title></rect>
<rect x="561" y="880" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Luna, ascension 0: Patron held in 1 of 5 runs</title></rect>
<text x="577.5" y="893.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="596" y="880" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Haiku 5.5, ascension 0: never held Patron</title></rect>
<rect x="631" y="880" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Nemotron 3 Ultra, ascension 0: Patron held in 1 of 5 runs</title></rect>
<text x="647.5" y="893.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="666" y="880" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiniMax M3, ascension 0: never held Patron</title></rect>
<text x="0.0" y="913.0" font-size="12" fill="#2c2c2c" text-anchor="start">Scorched Earth</text>
<rect x="141" y="900" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Opus 5.5, ascension 10: Scorched Earth held in 1 of 5 runs</title></rect>
<text x="157.5" y="913.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="176" y="900" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 10: never held Scorched Earth</title></rect>
<rect x="211" y="900" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Sol, ascension 10: never held Scorched Earth</title></rect>
<rect x="246" y="900" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiMo-V2.6-Pro, ascension 10: never held Scorched Earth</title></rect>
<rect x="281" y="900" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4.1 Flash, ascension 10: never held Scorched Earth</title></rect>
<rect x="316" y="900" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6.1 Sol, ascension 10: never held Scorched Earth</title></rect>
<rect x="351" y="900" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4 Pro, ascension 10: never held Scorched Earth</title></rect>
<rect x="386" y="900" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GLM 5.3 Flash, ascension 10: never held Scorched Earth</title></rect>
<rect x="421" y="900" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 10: never held Scorched Earth</title></rect>
<rect x="456" y="900" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 0: never held Scorched Earth</title></rect>
<rect x="491" y="900" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Sonnet 5.5, ascension 0: Scorched Earth held in 1 of 5 runs</title></rect>
<text x="507.5" y="913.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="526" y="900" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Step 5 Preview, ascension 0: never held Scorched Earth</title></rect>
<rect x="561" y="900" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Luna, ascension 0: never held Scorched Earth</title></rect>
<rect x="596" y="900" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Haiku 5.5, ascension 0: never held Scorched Earth</title></rect>
<rect x="631" y="900" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Nemotron 3 Ultra, ascension 0: never held Scorched Earth</title></rect>
<rect x="666" y="900" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiniMax M3, ascension 0: never held Scorched Earth</title></rect>
<text x="0.0" y="933.0" font-size="12" fill="#2c2c2c" text-anchor="start">Thesaurus</text>
<rect x="141" y="920" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Claude Opus 5.5, ascension 10: Thesaurus held in 2 of 5 runs</title></rect>
<text x="157.5" y="933.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2</text>
<rect x="176" y="920" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 10: never held Thesaurus</title></rect>
<rect x="211" y="920" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Sol, ascension 10: never held Thesaurus</title></rect>
<rect x="246" y="920" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiMo-V2.6-Pro, ascension 10: never held Thesaurus</title></rect>
<rect x="281" y="920" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4.1 Flash, ascension 10: never held Thesaurus</title></rect>
<rect x="316" y="920" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6.1 Sol, ascension 10: never held Thesaurus</title></rect>
<rect x="351" y="920" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4 Pro, ascension 10: never held Thesaurus</title></rect>
<rect x="386" y="920" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GLM 5.3 Flash, ascension 10: never held Thesaurus</title></rect>
<rect x="421" y="920" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 10: never held Thesaurus</title></rect>
<rect x="456" y="920" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 0: never held Thesaurus</title></rect>
<rect x="491" y="920" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 0: never held Thesaurus</title></rect>
<rect x="526" y="920" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Step 5 Preview, ascension 0: never held Thesaurus</title></rect>
<rect x="561" y="920" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Luna, ascension 0: never held Thesaurus</title></rect>
<rect x="596" y="920" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Haiku 5.5, ascension 0: never held Thesaurus</title></rect>
<rect x="631" y="920" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Nemotron 3 Ultra, ascension 0: never held Thesaurus</title></rect>
<rect x="666" y="920" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiniMax M3, ascension 0: never held Thesaurus</title></rect>
<text x="0.0" y="953.0" font-size="12" fill="#2c2c2c" text-anchor="start">Blank Page</text>
<rect x="141" y="940" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Opus 5.5, ascension 10: never held Blank Page</title></rect>
<rect x="176" y="940" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 10: never held Blank Page</title></rect>
<rect x="211" y="940" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Sol, ascension 10: never held Blank Page</title></rect>
<rect x="246" y="940" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiMo-V2.6-Pro, ascension 10: never held Blank Page</title></rect>
<rect x="281" y="940" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4.1 Flash, ascension 10: never held Blank Page</title></rect>
<rect x="316" y="940" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6.1 Sol, ascension 10: never held Blank Page</title></rect>
<rect x="351" y="940" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4 Pro, ascension 10: never held Blank Page</title></rect>
<rect x="386" y="940" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GLM 5.3 Flash, ascension 10: never held Blank Page</title></rect>
<rect x="421" y="940" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 10: never held Blank Page</title></rect>
<rect x="456" y="940" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 0: never held Blank Page</title></rect>
<rect x="491" y="940" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 0: never held Blank Page</title></rect>
<rect x="526" y="940" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Step 5 Preview, ascension 0: never held Blank Page</title></rect>
<rect x="561" y="940" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>GPT-6 Luna, ascension 0: Blank Page held in 1 of 5 runs</title></rect>
<text x="577.5" y="953.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="596" y="940" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Haiku 5.5, ascension 0: never held Blank Page</title></rect>
<rect x="631" y="940" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Nemotron 3 Ultra, ascension 0: never held Blank Page</title></rect>
<rect x="666" y="940" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiniMax M3, ascension 0: never held Blank Page</title></rect>
<text x="0.0" y="973.0" font-size="12" fill="#2c2c2c" text-anchor="start">Carbon Copy</text>
<rect x="141" y="960" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Opus 5.5, ascension 10: never held Carbon Copy</title></rect>
<rect x="176" y="960" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 10: never held Carbon Copy</title></rect>
<rect x="211" y="960" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Sol, ascension 10: never held Carbon Copy</title></rect>
<rect x="246" y="960" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiMo-V2.6-Pro, ascension 10: never held Carbon Copy</title></rect>
<rect x="281" y="960" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4.1 Flash, ascension 10: never held Carbon Copy</title></rect>
<rect x="316" y="960" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6.1 Sol, ascension 10: never held Carbon Copy</title></rect>
<rect x="351" y="960" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4 Pro, ascension 10: never held Carbon Copy</title></rect>
<rect x="386" y="960" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GLM 5.3 Flash, ascension 10: never held Carbon Copy</title></rect>
<rect x="421" y="960" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Gemini 3.8 Flash, ascension 10: Carbon Copy held in 1 of 5 runs</title></rect>
<text x="437.5" y="973.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="456" y="960" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 0: never held Carbon Copy</title></rect>
<rect x="491" y="960" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 0: never held Carbon Copy</title></rect>
<rect x="526" y="960" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Step 5 Preview, ascension 0: never held Carbon Copy</title></rect>
<rect x="561" y="960" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Luna, ascension 0: never held Carbon Copy</title></rect>
<rect x="596" y="960" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Haiku 5.5, ascension 0: never held Carbon Copy</title></rect>
<rect x="631" y="960" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Nemotron 3 Ultra, ascension 0: never held Carbon Copy</title></rect>
<rect x="666" y="960" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiniMax M3, ascension 0: never held Carbon Copy</title></rect>
<text x="0.0" y="993.0" font-size="12" fill="#2c2c2c" text-anchor="start">The Chorus</text>
<rect x="141" y="980" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Opus 5.5, ascension 10: never held The Chorus</title></rect>
<rect x="176" y="980" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 10: never held The Chorus</title></rect>
<rect x="211" y="980" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Sol, ascension 10: never held The Chorus</title></rect>
<rect x="246" y="980" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiMo-V2.6-Pro, ascension 10: never held The Chorus</title></rect>
<rect x="281" y="980" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4.1 Flash, ascension 10: never held The Chorus</title></rect>
<rect x="316" y="980" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6.1 Sol, ascension 10: never held The Chorus</title></rect>
<rect x="351" y="980" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4 Pro, ascension 10: never held The Chorus</title></rect>
<rect x="386" y="980" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GLM 5.3 Flash, ascension 10: never held The Chorus</title></rect>
<rect x="421" y="980" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 10: never held The Chorus</title></rect>
<rect x="456" y="980" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 0: never held The Chorus</title></rect>
<rect x="491" y="980" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Claude Sonnet 5.5, ascension 0: The Chorus held in 1 of 5 runs</title></rect>
<text x="507.5" y="993.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="526" y="980" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Step 5 Preview, ascension 0: never held The Chorus</title></rect>
<rect x="561" y="980" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Luna, ascension 0: never held The Chorus</title></rect>
<rect x="596" y="980" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Haiku 5.5, ascension 0: never held The Chorus</title></rect>
<rect x="631" y="980" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Nemotron 3 Ultra, ascension 0: never held The Chorus</title></rect>
<rect x="666" y="980" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiniMax M3, ascension 0: never held The Chorus</title></rect>
<text x="0.0" y="1013.0" font-size="12" fill="#2c2c2c" text-anchor="start">The Hoarder</text>
<rect x="141" y="1000" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Opus 5.5, ascension 10: never held The Hoarder</title></rect>
<rect x="176" y="1000" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 10: never held The Hoarder</title></rect>
<rect x="211" y="1000" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Sol, ascension 10: never held The Hoarder</title></rect>
<rect x="246" y="1000" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiMo-V2.6-Pro, ascension 10: never held The Hoarder</title></rect>
<rect x="281" y="1000" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4.1 Flash, ascension 10: never held The Hoarder</title></rect>
<rect x="316" y="1000" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6.1 Sol, ascension 10: never held The Hoarder</title></rect>
<rect x="351" y="1000" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4 Pro, ascension 10: never held The Hoarder</title></rect>
<rect x="386" y="1000" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GLM 5.3 Flash, ascension 10: never held The Hoarder</title></rect>
<rect x="421" y="1000" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 10: never held The Hoarder</title></rect>
<rect x="456" y="1000" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 0: never held The Hoarder</title></rect>
<rect x="491" y="1000" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 0: never held The Hoarder</title></rect>
<rect x="526" y="1000" width="33" height="18" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Step 5 Preview, ascension 0: The Hoarder held in 1 of 5 runs</title></rect>
<text x="542.5" y="1013.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1</text>
<rect x="561" y="1000" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Luna, ascension 0: never held The Hoarder</title></rect>
<rect x="596" y="1000" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Haiku 5.5, ascension 0: never held The Hoarder</title></rect>
<rect x="631" y="1000" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Nemotron 3 Ultra, ascension 0: never held The Hoarder</title></rect>
<rect x="666" y="1000" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiniMax M3, ascension 0: never held The Hoarder</title></rect>
<text x="0.0" y="1033.0" font-size="12" fill="#8a8878" text-anchor="start">Alphabetist</text>
<rect x="141" y="1020" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Opus 5.5, ascension 10: never held Alphabetist</title></rect>
<rect x="176" y="1020" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 10: never held Alphabetist</title></rect>
<rect x="211" y="1020" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Sol, ascension 10: never held Alphabetist</title></rect>
<rect x="246" y="1020" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiMo-V2.6-Pro, ascension 10: never held Alphabetist</title></rect>
<rect x="281" y="1020" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4.1 Flash, ascension 10: never held Alphabetist</title></rect>
<rect x="316" y="1020" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6.1 Sol, ascension 10: never held Alphabetist</title></rect>
<rect x="351" y="1020" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4 Pro, ascension 10: never held Alphabetist</title></rect>
<rect x="386" y="1020" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GLM 5.3 Flash, ascension 10: never held Alphabetist</title></rect>
<rect x="421" y="1020" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 10: never held Alphabetist</title></rect>
<rect x="456" y="1020" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 0: never held Alphabetist</title></rect>
<rect x="491" y="1020" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 0: never held Alphabetist</title></rect>
<rect x="526" y="1020" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Step 5 Preview, ascension 0: never held Alphabetist</title></rect>
<rect x="561" y="1020" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Luna, ascension 0: never held Alphabetist</title></rect>
<rect x="596" y="1020" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Haiku 5.5, ascension 0: never held Alphabetist</title></rect>
<rect x="631" y="1020" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Nemotron 3 Ultra, ascension 0: never held Alphabetist</title></rect>
<rect x="666" y="1020" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiniMax M3, ascension 0: never held Alphabetist</title></rect>
<text x="0.0" y="1053.0" font-size="12" fill="#8a8878" text-anchor="start">Compound Interest</text>
<rect x="141" y="1040" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Opus 5.5, ascension 10: never held Compound Interest</title></rect>
<rect x="176" y="1040" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 10: never held Compound Interest</title></rect>
<rect x="211" y="1040" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Sol, ascension 10: never held Compound Interest</title></rect>
<rect x="246" y="1040" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiMo-V2.6-Pro, ascension 10: never held Compound Interest</title></rect>
<rect x="281" y="1040" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4.1 Flash, ascension 10: never held Compound Interest</title></rect>
<rect x="316" y="1040" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6.1 Sol, ascension 10: never held Compound Interest</title></rect>
<rect x="351" y="1040" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>DeepSeek V4 Pro, ascension 10: never held Compound Interest</title></rect>
<rect x="386" y="1040" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GLM 5.3 Flash, ascension 10: never held Compound Interest</title></rect>
<rect x="421" y="1040" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 10: never held Compound Interest</title></rect>
<rect x="456" y="1040" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Gemini 3.8 Flash, ascension 0: never held Compound Interest</title></rect>
<rect x="491" y="1040" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Sonnet 5.5, ascension 0: never held Compound Interest</title></rect>
<rect x="526" y="1040" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Step 5 Preview, ascension 0: never held Compound Interest</title></rect>
<rect x="561" y="1040" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>GPT-6 Luna, ascension 0: never held Compound Interest</title></rect>
<rect x="596" y="1040" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Claude Haiku 5.5, ascension 0: never held Compound Interest</title></rect>
<rect x="631" y="1040" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>Nemotron 3 Ultra, ascension 0: never held Compound Interest</title></rect>
<rect x="666" y="1040" width="33" height="18" rx="2" fill="#e3e1cf" fill-opacity="0.45"><title>MiniMax M3, ascension 0: never held Compound Interest</title></rect>
<text x="0.0" y="1077.0" font-size="12" fill="#8c0000" text-anchor="start" font-weight="700">Different relics held</text>
<text x="157.5" y="1077.0" font-size="11" fill="#8c0000" text-anchor="middle" font-weight="700">23</text>
<text x="192.5" y="1077.0" font-size="11" fill="#8c0000" text-anchor="middle" font-weight="700">24</text>
<text x="227.5" y="1077.0" font-size="11" fill="#8c0000" text-anchor="middle" font-weight="700">18</text>
<text x="262.5" y="1077.0" font-size="11" fill="#8c0000" text-anchor="middle" font-weight="700">23</text>
<text x="297.5" y="1077.0" font-size="11" fill="#8c0000" text-anchor="middle" font-weight="700">15</text>
<text x="332.5" y="1077.0" font-size="11" fill="#8c0000" text-anchor="middle" font-weight="700">21</text>
<text x="367.5" y="1077.0" font-size="11" fill="#8c0000" text-anchor="middle" font-weight="700">20</text>
<text x="402.5" y="1077.0" font-size="11" fill="#8c0000" text-anchor="middle" font-weight="700">21</text>
<text x="437.5" y="1077.0" font-size="11" fill="#8c0000" text-anchor="middle" font-weight="700">17</text>
<text x="472.5" y="1077.0" font-size="11" fill="#8c0000" text-anchor="middle" font-weight="700">19</text>
<text x="507.5" y="1077.0" font-size="11" fill="#8c0000" text-anchor="middle" font-weight="700">22</text>
<text x="542.5" y="1077.0" font-size="11" fill="#8c0000" text-anchor="middle" font-weight="700">24</text>
<text x="577.5" y="1077.0" font-size="11" fill="#8c0000" text-anchor="middle" font-weight="700">32</text>
<text x="612.5" y="1077.0" font-size="11" fill="#8c0000" text-anchor="middle" font-weight="700">19</text>
<text x="647.5" y="1077.0" font-size="11" fill="#8c0000" text-anchor="middle" font-weight="700">21</text>
<text x="682.5" y="1077.0" font-size="11" fill="#8c0000" text-anchor="middle" font-weight="700">16</text>
<text x="0.0" y="1120.0" font-size="11" fill="#8a8878" text-anchor="start">runs, of 5, in which the model held the relic · blank is never · all 47 relics, most-held first</text>
</svg>

### In the shop

Opus is the only model that rerolls the shop to speak of, 3.6 times a run. Luna sells seven relics a run, more than twice anyone else.

<svg class="results" style="width:100%;height:auto;margin:1em 0" viewBox="0 0 720 502" role="img" aria-labelledby="shop-title" font-family="inherit">
<title id="shop-title">Shop actions per run for each model: purchases, relics sold, rerolls and consumables used.</title>
<text x="278.1" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">Purchases</text>
<text x="404.4" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">Relics sold</text>
<text x="530.6" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">Rerolls</text>
<text x="656.9" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">Consumables used</text>
<text x="0.0" y="28.0" font-size="12" fill="#8c0000" text-anchor="start" font-weight="700">Ascension 10, the hardest</text>
<text x="0.0" y="51.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Opus 5.5</text>
<rect x="216.0" y="36" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.61"><title>Claude Opus 5.5: 25.6 purchases per run</title></rect>
<text x="278.1" y="51.0" font-size="11" fill="#ffffff" text-anchor="middle">25.6</text>
<rect x="342.3" y="36" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.34"><title>Claude Opus 5.5: 2.8 relics sold per run</title></rect>
<text x="404.4" y="51.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2.8</text>
<rect x="468.5" y="36" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Claude Opus 5.5: 3.6 rerolls per run</title></rect>
<text x="530.6" y="51.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.6</text>
<rect x="594.8" y="36" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.85"><title>Claude Opus 5.5: 4.0 consumables used per run</title></rect>
<text x="656.9" y="51.0" font-size="11" fill="#ffffff" text-anchor="middle">4.0</text>
<text x="0.0" y="77.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Sonnet 5.5</text>
<rect x="216.0" y="62" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.51"><title>Claude Sonnet 5.5: 21.4 purchases per run</title></rect>
<text x="278.1" y="77.0" font-size="11" fill="#2c2c2c" text-anchor="middle">21.4</text>
<rect x="342.3" y="62" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.34"><title>Claude Sonnet 5.5: 2.8 relics sold per run</title></rect>
<text x="404.4" y="77.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2.8</text>
<rect x="468.5" y="62" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Claude Sonnet 5.5: 0.2 rerolls per run</title></rect>
<text x="530.6" y="77.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.2</text>
<rect x="594.8" y="62" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.64"><title>Claude Sonnet 5.5: 3.0 consumables used per run</title></rect>
<text x="656.9" y="77.0" font-size="11" fill="#ffffff" text-anchor="middle">3.0</text>
<text x="0.0" y="103.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6 Sol</text>
<rect x="216.0" y="88" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.35"><title>GPT-6 Sol: 14.6 purchases per run</title></rect>
<text x="278.1" y="103.0" font-size="11" fill="#2c2c2c" text-anchor="middle">14.6</text>
<rect x="342.3" y="88" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.07"><title>GPT-6 Sol: 0.6 relics sold per run</title></rect>
<text x="404.4" y="103.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.6</text>
<rect x="468.5" y="88" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.38"><title>GPT-6 Sol: 2.8 rerolls per run</title></rect>
<text x="530.6" y="103.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2.8</text>
<rect x="594.8" y="88" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.72"><title>GPT-6 Sol: 3.4 consumables used per run</title></rect>
<text x="656.9" y="103.0" font-size="11" fill="#ffffff" text-anchor="middle">3.4</text>
<text x="0.0" y="129.0" font-size="13" fill="#2c2c2c" text-anchor="start">MiMo-V2.6-Pro</text>
<rect x="216.0" y="114" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.30"><title>MiMo-V2.6-Pro: 12.4 purchases per run</title></rect>
<text x="278.1" y="129.0" font-size="11" fill="#2c2c2c" text-anchor="middle">12.4</text>
<rect x="342.3" y="114" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.22"><title>MiMo-V2.6-Pro: 1.8 relics sold per run</title></rect>
<text x="404.4" y="129.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1.8</text>
<rect x="468.5" y="114" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.11"><title>MiMo-V2.6-Pro: 0.8 rerolls per run</title></rect>
<text x="530.6" y="129.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.8</text>
<rect x="594.8" y="114" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.47"><title>MiMo-V2.6-Pro: 2.2 consumables used per run</title></rect>
<text x="656.9" y="129.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2.2</text>
<text x="0.0" y="155.0" font-size="13" fill="#2c2c2c" text-anchor="start">DeepSeek V4.1 Flash</text>
<rect x="216.0" y="140" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.27"><title>DeepSeek V4.1 Flash: 11.4 purchases per run</title></rect>
<text x="278.1" y="155.0" font-size="11" fill="#2c2c2c" text-anchor="middle">11.4</text>
<rect x="342.3" y="140" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.15"><title>DeepSeek V4.1 Flash: 1.2 relics sold per run</title></rect>
<text x="404.4" y="155.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1.2</text>
<rect x="468.5" y="140" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>DeepSeek V4.1 Flash: 0.4 rerolls per run</title></rect>
<text x="530.6" y="155.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.4</text>
<rect x="594.8" y="140" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.47"><title>DeepSeek V4.1 Flash: 2.2 consumables used per run</title></rect>
<text x="656.9" y="155.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2.2</text>
<text x="0.0" y="181.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6.1 Sol</text>
<rect x="216.0" y="166" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.28"><title>GPT-6.1 Sol: 11.6 purchases per run</title></rect>
<text x="278.1" y="181.0" font-size="11" fill="#2c2c2c" text-anchor="middle">11.6</text>
<rect x="342.3" y="166" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.27"><title>GPT-6.1 Sol: 2.2 relics sold per run</title></rect>
<text x="404.4" y="181.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2.2</text>
<rect x="468.5" y="166" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.14"><title>GPT-6.1 Sol: 1.0 rerolls per run</title></rect>
<text x="530.6" y="181.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1.0</text>
<rect x="594.8" y="166" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.34"><title>GPT-6.1 Sol: 1.6 consumables used per run</title></rect>
<text x="656.9" y="181.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1.6</text>
<text x="0.0" y="207.0" font-size="13" fill="#2c2c2c" text-anchor="start">DeepSeek V4 Pro</text>
<rect x="216.0" y="192" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.37"><title>DeepSeek V4 Pro: 15.4 purchases per run</title></rect>
<text x="278.1" y="207.0" font-size="11" fill="#2c2c2c" text-anchor="middle">15.4</text>
<rect x="342.3" y="192" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.32"><title>DeepSeek V4 Pro: 2.6 relics sold per run</title></rect>
<text x="404.4" y="207.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2.6</text>
<rect x="468.5" y="192" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>DeepSeek V4 Pro: 0.0 rerolls per run</title></rect>
<text x="530.6" y="207.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.0</text>
<rect x="594.8" y="192" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.34"><title>DeepSeek V4 Pro: 1.6 consumables used per run</title></rect>
<text x="656.9" y="207.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1.6</text>
<text x="0.0" y="233.0" font-size="13" fill="#2c2c2c" text-anchor="start">GLM 5.3 Flash</text>
<rect x="216.0" y="218" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.27"><title>GLM 5.3 Flash: 11.4 purchases per run</title></rect>
<text x="278.1" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">11.4</text>
<rect x="342.3" y="218" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.27"><title>GLM 5.3 Flash: 2.2 relics sold per run</title></rect>
<text x="404.4" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2.2</text>
<rect x="468.5" y="218" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>GLM 5.3 Flash: 0.2 rerolls per run</title></rect>
<text x="530.6" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.2</text>
<rect x="594.8" y="218" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.59"><title>GLM 5.3 Flash: 2.8 consumables used per run</title></rect>
<text x="656.9" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2.8</text>
<text x="0.0" y="259.0" font-size="13" fill="#2c2c2c" text-anchor="start">Gemini 3.8 Flash</text>
<rect x="216.0" y="244" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.28"><title>Gemini 3.8 Flash: 11.8 purchases per run</title></rect>
<text x="278.1" y="259.0" font-size="11" fill="#2c2c2c" text-anchor="middle">11.8</text>
<rect x="342.3" y="244" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.07"><title>Gemini 3.8 Flash: 0.6 relics sold per run</title></rect>
<text x="404.4" y="259.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.6</text>
<rect x="468.5" y="244" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.08"><title>Gemini 3.8 Flash: 0.6 rerolls per run</title></rect>
<text x="530.6" y="259.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.6</text>
<rect x="594.8" y="244" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.30"><title>Gemini 3.8 Flash: 1.4 consumables used per run</title></rect>
<text x="656.9" y="259.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1.4</text>
<text x="0.0" y="290.0" font-size="12" fill="#8c0000" text-anchor="start" font-weight="700">Ascension 0, the easiest</text>
<text x="0.0" y="313.0" font-size="13" fill="#2c2c2c" text-anchor="start">Gemini 3.8 Flash</text>
<rect x="216.0" y="298" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.85"><title>Gemini 3.8 Flash: 35.4 purchases per run</title></rect>
<text x="278.1" y="313.0" font-size="11" fill="#ffffff" text-anchor="middle">35.4</text>
<rect x="342.3" y="298" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Gemini 3.8 Flash: 0.4 relics sold per run</title></rect>
<text x="404.4" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.4</text>
<rect x="468.5" y="298" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.85"><title>Gemini 3.8 Flash: 6.2 rerolls per run</title></rect>
<text x="530.6" y="313.0" font-size="11" fill="#ffffff" text-anchor="middle">6.2</text>
<rect x="594.8" y="298" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.68"><title>Gemini 3.8 Flash: 3.2 consumables used per run</title></rect>
<text x="656.9" y="313.0" font-size="11" fill="#ffffff" text-anchor="middle">3.2</text>
<text x="0.0" y="339.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Sonnet 5.5</text>
<rect x="216.0" y="324" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.55"><title>Claude Sonnet 5.5: 23.0 purchases per run</title></rect>
<text x="278.1" y="339.0" font-size="11" fill="#2c2c2c" text-anchor="middle">23.0</text>
<rect x="342.3" y="324" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.22"><title>Claude Sonnet 5.5: 1.8 relics sold per run</title></rect>
<text x="404.4" y="339.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1.8</text>
<rect x="468.5" y="324" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Claude Sonnet 5.5: 0.0 rerolls per run</title></rect>
<text x="530.6" y="339.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.0</text>
<rect x="594.8" y="324" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.38"><title>Claude Sonnet 5.5: 1.8 consumables used per run</title></rect>
<text x="656.9" y="339.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1.8</text>
<text x="0.0" y="365.0" font-size="13" fill="#2c2c2c" text-anchor="start">Step 5 Preview</text>
<rect x="216.0" y="350" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.31"><title>Step 5 Preview: 13.0 purchases per run</title></rect>
<text x="278.1" y="365.0" font-size="11" fill="#2c2c2c" text-anchor="middle">13.0</text>
<rect x="342.3" y="350" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.29"><title>Step 5 Preview: 2.4 relics sold per run</title></rect>
<text x="404.4" y="365.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2.4</text>
<rect x="468.5" y="350" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.14"><title>Step 5 Preview: 1.0 rerolls per run</title></rect>
<text x="530.6" y="365.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1.0</text>
<rect x="594.8" y="350" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.68"><title>Step 5 Preview: 3.2 consumables used per run</title></rect>
<text x="656.9" y="365.0" font-size="11" fill="#ffffff" text-anchor="middle">3.2</text>
<text x="0.0" y="391.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6 Luna</text>
<rect x="216.0" y="376" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.43"><title>GPT-6 Luna: 17.8 purchases per run</title></rect>
<text x="278.1" y="391.0" font-size="11" fill="#2c2c2c" text-anchor="middle">17.8</text>
<rect x="342.3" y="376" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.85"><title>GPT-6 Luna: 7.0 relics sold per run</title></rect>
<text x="404.4" y="391.0" font-size="11" fill="#ffffff" text-anchor="middle">7.0</text>
<rect x="468.5" y="376" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>GPT-6 Luna: 0.4 rerolls per run</title></rect>
<text x="530.6" y="391.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.4</text>
<rect x="594.8" y="376" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.77"><title>GPT-6 Luna: 3.6 consumables used per run</title></rect>
<text x="656.9" y="391.0" font-size="11" fill="#ffffff" text-anchor="middle">3.6</text>
<text x="0.0" y="417.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Haiku 5.5</text>
<rect x="216.0" y="402" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.22"><title>Claude Haiku 5.5: 9.0 purchases per run</title></rect>
<text x="278.1" y="417.0" font-size="11" fill="#2c2c2c" text-anchor="middle">9.0</text>
<rect x="342.3" y="402" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.12"><title>Claude Haiku 5.5: 1.0 relics sold per run</title></rect>
<text x="404.4" y="417.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1.0</text>
<rect x="468.5" y="402" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Claude Haiku 5.5: 0.0 rerolls per run</title></rect>
<text x="530.6" y="417.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.0</text>
<rect x="594.8" y="402" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.26"><title>Claude Haiku 5.5: 1.2 consumables used per run</title></rect>
<text x="656.9" y="417.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1.2</text>
<text x="0.0" y="443.0" font-size="13" fill="#2c2c2c" text-anchor="start">Nemotron 3 Ultra</text>
<rect x="216.0" y="428" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.27"><title>Nemotron 3 Ultra: 11.2 purchases per run</title></rect>
<text x="278.1" y="443.0" font-size="11" fill="#2c2c2c" text-anchor="middle">11.2</text>
<rect x="342.3" y="428" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.22"><title>Nemotron 3 Ultra: 1.8 relics sold per run</title></rect>
<text x="404.4" y="443.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1.8</text>
<rect x="468.5" y="428" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Nemotron 3 Ultra: 0.0 rerolls per run</title></rect>
<text x="530.6" y="443.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.0</text>
<rect x="594.8" y="428" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.42"><title>Nemotron 3 Ultra: 2.0 consumables used per run</title></rect>
<text x="656.9" y="443.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2.0</text>
<text x="0.0" y="469.0" font-size="13" fill="#2c2c2c" text-anchor="start">MiniMax M3</text>
<rect x="216.0" y="454" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.13"><title>MiniMax M3: 5.4 purchases per run</title></rect>
<text x="278.1" y="469.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.4</text>
<rect x="342.3" y="454" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.12"><title>MiniMax M3: 1.0 relics sold per run</title></rect>
<text x="404.4" y="469.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1.0</text>
<rect x="468.5" y="454" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>MiniMax M3: 0.4 rerolls per run</title></rect>
<text x="530.6" y="469.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.4</text>
<rect x="594.8" y="454" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.21"><title>MiniMax M3: 1.0 consumables used per run</title></rect>
<text x="656.9" y="469.0" font-size="11" fill="#2c2c2c" text-anchor="middle">1.0</text>
<text x="215.0" y="496.0" font-size="11" fill="#8a8878" text-anchor="start">per run, averaged over 5 runs · each column shaded against its own highest</text>
</svg>

Opus also has the most to spend. It earned 8.9 gold a round against Sonnet's 6.7 on the same difficulty, and most of the gap is gold paid by the guesses themselves: 1.8 a round against 0.3. It ended its runs with about 7 gold left, having spent the rest. The round payout is smaller at ascension 10, about 3 gold a round against 4, so the ascension 0 rows start ahead.

<svg class="results" style="width:100%;height:auto;margin:1em 0" viewBox="0 0 720 594" role="img" aria-labelledby="gold-title" font-family="inherit">
<title id="gold-title">Gold earned per round cleared by each model, split by source, with gold spent beside it.</title>
<rect x="0.0" y="4" width="12" height="12" rx="2" fill="#787c7e"/>
<text x="17.0" y="14.0" font-size="12" fill="#2c2c2c" text-anchor="start">round payout</text>
<rect x="114.2" y="4" width="12" height="12" rx="2" fill="#538d4e"/>
<text x="131.2" y="14.0" font-size="12" fill="#2c2c2c" text-anchor="start">unused guesses</text>
<rect x="241.6" y="4" width="12" height="12" rx="2" fill="#b59f3b"/>
<text x="258.6" y="14.0" font-size="12" fill="#2c2c2c" text-anchor="start">interest</text>
<rect x="329.4" y="4" width="12" height="12" rx="2" fill="#8c0000"/>
<text x="346.4" y="14.0" font-size="12" fill="#2c2c2c" text-anchor="start">scoring</text>
<rect x="410.6" y="4" width="12" height="12" rx="2" fill="#2f6f8f"/>
<text x="427.6" y="14.0" font-size="12" fill="#2c2c2c" text-anchor="start">sales</text>
<rect x="478.6" y="4" width="12" height="12" rx="2" fill="#6b4c8a"/>
<text x="495.6" y="14.0" font-size="12" fill="#2c2c2c" text-anchor="start">relic payouts</text>
<line x1="215.0" x2="215.0" y1="28" y2="560" stroke="#e3e1cf"/>
<text x="215.0" y="572.0" font-size="11" fill="#8a8878" text-anchor="middle">0</text>
<line x1="275.8" x2="275.8" y1="28" y2="560" stroke="#e3e1cf"/>
<text x="275.8" y="572.0" font-size="11" fill="#8a8878" text-anchor="middle">2</text>
<line x1="336.5" x2="336.5" y1="28" y2="560" stroke="#e3e1cf"/>
<text x="336.5" y="572.0" font-size="11" fill="#8a8878" text-anchor="middle">4</text>
<line x1="397.3" x2="397.3" y1="28" y2="560" stroke="#e3e1cf"/>
<text x="397.3" y="572.0" font-size="11" fill="#8a8878" text-anchor="middle">6</text>
<line x1="458.1" x2="458.1" y1="28" y2="560" stroke="#e3e1cf"/>
<text x="458.1" y="572.0" font-size="11" fill="#8a8878" text-anchor="middle">8</text>
<line x1="518.8" x2="518.8" y1="28" y2="560" stroke="#e3e1cf"/>
<text x="518.8" y="572.0" font-size="11" fill="#8a8878" text-anchor="middle">10</text>
<line x1="579.6" x2="579.6" y1="28" y2="560" stroke="#e3e1cf"/>
<text x="579.6" y="572.0" font-size="11" fill="#8a8878" text-anchor="middle">12</text>
<text x="0.0" y="38.0" font-size="12" fill="#8c0000" text-anchor="start" font-weight="700">Ascension 10, the hardest</text>
<text x="0.0" y="61.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Opus 5.5</text>
<rect x="216.0" y="46" width="87.6" height="20" rx="2" fill="#787c7e"><title>Claude Opus 5.5: 2.95 gold a round from round payout</title></rect>
<text x="259.8" y="60.0" font-size="11" fill="#ffffff" text-anchor="middle">2.9</text>
<rect x="305.6" y="46" width="67.2" height="20" rx="2" fill="#538d4e"><title>Claude Opus 5.5: 2.28 gold a round from unused guesses</title></rect>
<text x="339.2" y="60.0" font-size="11" fill="#ffffff" text-anchor="middle">2.3</text>
<rect x="374.8" y="46" width="37.2" height="20" rx="2" fill="#b59f3b"><title>Claude Opus 5.5: 1.29 gold a round from interest</title></rect>
<text x="393.4" y="60.0" font-size="11" fill="#ffffff" text-anchor="middle">1.3</text>
<rect x="414.0" y="46" width="52.8" height="20" rx="2" fill="#8c0000"><title>Claude Opus 5.5: 1.80 gold a round from scoring</title></rect>
<text x="440.4" y="60.0" font-size="11" fill="#ffffff" text-anchor="middle">1.8</text>
<rect x="468.8" y="46" width="10.5" height="20" rx="2" fill="#2f6f8f"><title>Claude Opus 5.5: 0.41 gold a round from sales</title></rect>
<rect x="481.3" y="46" width="4.3" height="20" rx="2" fill="#6b4c8a"><title>Claude Opus 5.5: 0.21 gold a round from relic payouts</title></rect>
<text x="494.6" y="56.0" font-size="11" fill="#2c2c2c" text-anchor="start">8.9 earned</text>
<text x="494.6" y="68.0" font-size="11" fill="#8a8878" text-anchor="start">8.8 spent</text>
<text x="0.0" y="91.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Sonnet 5.5</text>
<rect x="216.0" y="76" width="85.7" height="20" rx="2" fill="#787c7e"><title>Claude Sonnet 5.5: 2.89 gold a round from round payout</title></rect>
<text x="258.9" y="90.0" font-size="11" fill="#ffffff" text-anchor="middle">2.9</text>
<rect x="303.7" y="76" width="57.8" height="20" rx="2" fill="#538d4e"><title>Claude Sonnet 5.5: 1.97 gold a round from unused guesses</title></rect>
<text x="332.7" y="90.0" font-size="11" fill="#ffffff" text-anchor="middle">2.0</text>
<rect x="363.6" y="76" width="29.9" height="20" rx="2" fill="#b59f3b"><title>Claude Sonnet 5.5: 1.05 gold a round from interest</title></rect>
<text x="378.5" y="90.0" font-size="11" fill="#ffffff" text-anchor="middle">1.1</text>
<rect x="395.5" y="76" width="6.4" height="20" rx="2" fill="#8c0000"><title>Claude Sonnet 5.5: 0.28 gold a round from scoring</title></rect>
<rect x="403.9" y="76" width="7.6" height="20" rx="2" fill="#2f6f8f"><title>Claude Sonnet 5.5: 0.32 gold a round from sales</title></rect>
<rect x="413.5" y="76" width="5.4" height="20" rx="2" fill="#6b4c8a"><title>Claude Sonnet 5.5: 0.24 gold a round from relic payouts</title></rect>
<text x="427.9" y="86.0" font-size="11" fill="#2c2c2c" text-anchor="start">6.7 earned</text>
<text x="427.9" y="98.0" font-size="11" fill="#8a8878" text-anchor="start">6.7 spent</text>
<text x="0.0" y="121.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6 Sol</text>
<rect x="216.0" y="106" width="87.4" height="20" rx="2" fill="#787c7e"><title>GPT-6 Sol: 2.94 gold a round from round payout</title></rect>
<text x="259.7" y="120.0" font-size="11" fill="#ffffff" text-anchor="middle">2.9</text>
<rect x="305.4" y="106" width="63.7" height="20" rx="2" fill="#538d4e"><title>GPT-6 Sol: 2.16 gold a round from unused guesses</title></rect>
<text x="337.2" y="120.0" font-size="11" fill="#ffffff" text-anchor="middle">2.2</text>
<rect x="371.1" y="106" width="9.6" height="20" rx="2" fill="#b59f3b"><title>GPT-6 Sol: 0.38 gold a round from interest</title></rect>
<rect x="382.7" y="106" width="33.3" height="20" rx="2" fill="#8c0000"><title>GPT-6 Sol: 1.16 gold a round from scoring</title></rect>
<text x="399.3" y="120.0" font-size="11" fill="#ffffff" text-anchor="middle">1.2</text>
<rect x="418.0" y="106" width="2.0" height="20" rx="2" fill="#2f6f8f"><title>GPT-6 Sol: 0.13 gold a round from sales</title></rect>
<text x="429.0" y="116.0" font-size="11" fill="#2c2c2c" text-anchor="start">6.8 earned</text>
<text x="429.0" y="128.0" font-size="11" fill="#8a8878" text-anchor="start">6.8 spent</text>
<text x="0.0" y="151.0" font-size="13" fill="#2c2c2c" text-anchor="start">MiMo-V2.6-Pro</text>
<rect x="216.0" y="136" width="85.9" height="20" rx="2" fill="#787c7e"><title>MiMo-V2.6-Pro: 2.89 gold a round from round payout</title></rect>
<text x="258.9" y="150.0" font-size="11" fill="#ffffff" text-anchor="middle">2.9</text>
<rect x="303.9" y="136" width="56.0" height="20" rx="2" fill="#538d4e"><title>MiMo-V2.6-Pro: 1.91 gold a round from unused guesses</title></rect>
<text x="331.9" y="150.0" font-size="11" fill="#ffffff" text-anchor="middle">1.9</text>
<rect x="361.8" y="136" width="1.0" height="20" rx="2" fill="#b59f3b"><title>MiMo-V2.6-Pro: 0.06 gold a round from interest</title></rect>
<rect x="363.7" y="136" width="2.2" height="20" rx="2" fill="#8c0000"><title>MiMo-V2.6-Pro: 0.14 gold a round from scoring</title></rect>
<rect x="367.9" y="136" width="9.7" height="20" rx="2" fill="#2f6f8f"><title>MiMo-V2.6-Pro: 0.38 gold a round from sales</title></rect>
<rect x="379.6" y="136" width="1.0" height="20" rx="2" fill="#6b4c8a"><title>MiMo-V2.6-Pro: 0.03 gold a round from relic payouts</title></rect>
<text x="387.5" y="146.0" font-size="11" fill="#2c2c2c" text-anchor="start">5.4 earned</text>
<text x="387.5" y="158.0" font-size="11" fill="#8a8878" text-anchor="start">5.4 spent</text>
<text x="0.0" y="181.0" font-size="13" fill="#2c2c2c" text-anchor="start">DeepSeek V4.1 Flash</text>
<rect x="216.0" y="166" width="86.7" height="20" rx="2" fill="#787c7e"><title>DeepSeek V4.1 Flash: 2.92 gold a round from round payout</title></rect>
<text x="259.4" y="180.0" font-size="11" fill="#ffffff" text-anchor="middle">2.9</text>
<rect x="304.7" y="166" width="54.4" height="20" rx="2" fill="#538d4e"><title>DeepSeek V4.1 Flash: 1.85 gold a round from unused guesses</title></rect>
<text x="331.9" y="180.0" font-size="11" fill="#ffffff" text-anchor="middle">1.9</text>
<rect x="361.1" y="166" width="1.4" height="20" rx="2" fill="#b59f3b"><title>DeepSeek V4.1 Flash: 0.11 gold a round from interest</title></rect>
<rect x="364.5" y="166" width="5.8" height="20" rx="2" fill="#2f6f8f"><title>DeepSeek V4.1 Flash: 0.26 gold a round from sales</title></rect>
<text x="379.3" y="176.0" font-size="11" fill="#2c2c2c" text-anchor="start">5.1 earned</text>
<text x="379.3" y="188.0" font-size="11" fill="#8a8878" text-anchor="start">5.1 spent</text>
<text x="0.0" y="211.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6.1 Sol</text>
<rect x="216.0" y="196" width="87.6" height="20" rx="2" fill="#787c7e"><title>GPT-6.1 Sol: 2.95 gold a round from round payout</title></rect>
<text x="259.8" y="210.0" font-size="11" fill="#ffffff" text-anchor="middle">3.0</text>
<rect x="305.6" y="196" width="61.8" height="20" rx="2" fill="#538d4e"><title>GPT-6.1 Sol: 2.10 gold a round from unused guesses</title></rect>
<text x="336.5" y="210.0" font-size="11" fill="#ffffff" text-anchor="middle">2.1</text>
<rect x="369.4" y="196" width="3.1" height="20" rx="2" fill="#b59f3b"><title>GPT-6.1 Sol: 0.17 gold a round from interest</title></rect>
<rect x="374.5" y="196" width="21.3" height="20" rx="2" fill="#8c0000"><title>GPT-6.1 Sol: 0.77 gold a round from scoring</title></rect>
<rect x="397.8" y="196" width="11.7" height="20" rx="2" fill="#2f6f8f"><title>GPT-6.1 Sol: 0.45 gold a round from sales</title></rect>
<text x="418.5" y="206.0" font-size="11" fill="#2c2c2c" text-anchor="start">6.4 earned</text>
<text x="418.5" y="218.0" font-size="11" fill="#8a8878" text-anchor="start">6.0 spent</text>
<text x="0.0" y="241.0" font-size="13" fill="#2c2c2c" text-anchor="start">DeepSeek V4 Pro</text>
<rect x="216.0" y="226" width="87.0" height="20" rx="2" fill="#787c7e"><title>DeepSeek V4 Pro: 2.93 gold a round from round payout</title></rect>
<text x="259.5" y="240.0" font-size="11" fill="#ffffff" text-anchor="middle">2.9</text>
<rect x="305.0" y="226" width="66.9" height="20" rx="2" fill="#538d4e"><title>DeepSeek V4 Pro: 2.27 gold a round from unused guesses</title></rect>
<text x="338.4" y="240.0" font-size="11" fill="#ffffff" text-anchor="middle">2.3</text>
<rect x="373.9" y="226" width="11.0" height="20" rx="2" fill="#b59f3b"><title>DeepSeek V4 Pro: 0.43 gold a round from interest</title></rect>
<rect x="386.9" y="226" width="44.7" height="20" rx="2" fill="#8c0000"><title>DeepSeek V4 Pro: 1.54 gold a round from scoring</title></rect>
<text x="409.2" y="240.0" font-size="11" fill="#ffffff" text-anchor="middle">1.5</text>
<rect x="433.6" y="226" width="17.0" height="20" rx="2" fill="#2f6f8f"><title>DeepSeek V4 Pro: 0.63 gold a round from sales</title></rect>
<rect x="452.6" y="226" width="8.9" height="20" rx="2" fill="#6b4c8a"><title>DeepSeek V4 Pro: 0.36 gold a round from relic payouts</title></rect>
<text x="470.4" y="236.0" font-size="11" fill="#2c2c2c" text-anchor="start">8.1 earned</text>
<text x="470.4" y="248.0" font-size="11" fill="#8a8878" text-anchor="start">7.7 spent</text>
<text x="0.0" y="271.0" font-size="13" fill="#2c2c2c" text-anchor="start">GLM 5.3 Flash</text>
<rect x="216.0" y="256" width="86.9" height="20" rx="2" fill="#787c7e"><title>GLM 5.3 Flash: 2.93 gold a round from round payout</title></rect>
<text x="259.5" y="270.0" font-size="11" fill="#ffffff" text-anchor="middle">2.9</text>
<rect x="304.9" y="256" width="55.5" height="20" rx="2" fill="#538d4e"><title>GLM 5.3 Flash: 1.89 gold a round from unused guesses</title></rect>
<text x="332.7" y="270.0" font-size="11" fill="#ffffff" text-anchor="middle">1.9</text>
<rect x="362.4" y="256" width="1.0" height="20" rx="2" fill="#b59f3b"><title>GLM 5.3 Flash: 0.05 gold a round from interest</title></rect>
<rect x="364.1" y="256" width="1.0" height="20" rx="2" fill="#8c0000"><title>GLM 5.3 Flash: 0.07 gold a round from scoring</title></rect>
<rect x="366.3" y="256" width="11.3" height="20" rx="2" fill="#2f6f8f"><title>GLM 5.3 Flash: 0.44 gold a round from sales</title></rect>
<text x="386.5" y="266.0" font-size="11" fill="#2c2c2c" text-anchor="start">5.4 earned</text>
<text x="386.5" y="278.0" font-size="11" fill="#8a8878" text-anchor="start">5.2 spent</text>
<text x="0.0" y="301.0" font-size="13" fill="#2c2c2c" text-anchor="start">Gemini 3.8 Flash</text>
<rect x="216.0" y="286" width="86.5" height="20" rx="2" fill="#787c7e"><title>Gemini 3.8 Flash: 2.91 gold a round from round payout</title></rect>
<text x="259.3" y="300.0" font-size="11" fill="#ffffff" text-anchor="middle">2.9</text>
<rect x="304.5" y="286" width="63.0" height="20" rx="2" fill="#538d4e"><title>Gemini 3.8 Flash: 2.14 gold a round from unused guesses</title></rect>
<text x="336.0" y="300.0" font-size="11" fill="#ffffff" text-anchor="middle">2.1</text>
<rect x="369.5" y="286" width="27.9" height="20" rx="2" fill="#b59f3b"><title>Gemini 3.8 Flash: 0.98 gold a round from interest</title></rect>
<text x="383.4" y="300.0" font-size="11" fill="#ffffff" text-anchor="middle">1.0</text>
<rect x="399.4" y="286" width="28.9" height="20" rx="2" fill="#8c0000"><title>Gemini 3.8 Flash: 1.02 gold a round from scoring</title></rect>
<text x="413.8" y="300.0" font-size="11" fill="#ffffff" text-anchor="middle">1.0</text>
<rect x="430.3" y="286" width="1.1" height="20" rx="2" fill="#2f6f8f"><title>Gemini 3.8 Flash: 0.10 gold a round from sales</title></rect>
<rect x="433.4" y="286" width="1.1" height="20" rx="2" fill="#6b4c8a"><title>Gemini 3.8 Flash: 0.10 gold a round from relic payouts</title></rect>
<text x="443.6" y="296.0" font-size="11" fill="#2c2c2c" text-anchor="start">7.3 earned</text>
<text x="443.6" y="308.0" font-size="11" fill="#8a8878" text-anchor="start">6.1 spent</text>
<text x="0.0" y="336.0" font-size="12" fill="#8c0000" text-anchor="start" font-weight="700">Ascension 0, the easiest</text>
<text x="0.0" y="359.0" font-size="13" fill="#2c2c2c" text-anchor="start">Gemini 3.8 Flash</text>
<rect x="216.0" y="344" width="119.5" height="20" rx="2" fill="#787c7e"><title>Gemini 3.8 Flash: 4.00 gold a round from round payout</title></rect>
<text x="275.8" y="358.0" font-size="11" fill="#ffffff" text-anchor="middle">4.0</text>
<rect x="337.5" y="344" width="60.5" height="20" rx="2" fill="#538d4e"><title>Gemini 3.8 Flash: 2.06 gold a round from unused guesses</title></rect>
<text x="367.8" y="358.0" font-size="11" fill="#ffffff" text-anchor="middle">2.1</text>
<rect x="400.1" y="344" width="100.8" height="20" rx="2" fill="#b59f3b"><title>Gemini 3.8 Flash: 3.38 gold a round from interest</title></rect>
<text x="450.5" y="358.0" font-size="11" fill="#ffffff" text-anchor="middle">3.4</text>
<rect x="502.9" y="344" width="72.2" height="20" rx="2" fill="#8c0000"><title>Gemini 3.8 Flash: 2.44 gold a round from scoring</title></rect>
<text x="539.0" y="358.0" font-size="11" fill="#ffffff" text-anchor="middle">2.4</text>
<rect x="577.1" y="344" width="1.0" height="20" rx="2" fill="#2f6f8f"><title>Gemini 3.8 Flash: 0.03 gold a round from sales</title></rect>
<rect x="578.1" y="344" width="5.1" height="20" rx="2" fill="#6b4c8a"><title>Gemini 3.8 Flash: 0.23 gold a round from relic payouts</title></rect>
<text x="592.2" y="354.0" font-size="11" fill="#2c2c2c" text-anchor="start">12.2 earned</text>
<text x="592.2" y="366.0" font-size="11" fill="#8a8878" text-anchor="start">9.7 spent</text>
<text x="0.0" y="389.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Sonnet 5.5</text>
<rect x="216.0" y="374" width="117.5" height="20" rx="2" fill="#787c7e"><title>Claude Sonnet 5.5: 3.93 gold a round from round payout</title></rect>
<text x="274.7" y="388.0" font-size="11" fill="#ffffff" text-anchor="middle">3.9</text>
<rect x="335.5" y="374" width="54.3" height="20" rx="2" fill="#538d4e"><title>Claude Sonnet 5.5: 1.85 gold a round from unused guesses</title></rect>
<text x="362.6" y="388.0" font-size="11" fill="#ffffff" text-anchor="middle">1.9</text>
<rect x="391.8" y="374" width="40.9" height="20" rx="2" fill="#b59f3b"><title>Claude Sonnet 5.5: 1.41 gold a round from interest</title></rect>
<text x="412.2" y="388.0" font-size="11" fill="#ffffff" text-anchor="middle">1.4</text>
<rect x="434.7" y="374" width="18.6" height="20" rx="2" fill="#8c0000"><title>Claude Sonnet 5.5: 0.68 gold a round from scoring</title></rect>
<rect x="455.2" y="374" width="5.1" height="20" rx="2" fill="#2f6f8f"><title>Claude Sonnet 5.5: 0.24 gold a round from sales</title></rect>
<text x="469.4" y="384.0" font-size="11" fill="#2c2c2c" text-anchor="start">8.1 earned</text>
<text x="469.4" y="396.0" font-size="11" fill="#8a8878" text-anchor="start">7.3 spent</text>
<text x="0.0" y="419.0" font-size="13" fill="#2c2c2c" text-anchor="start">Step 5 Preview</text>
<rect x="216.0" y="404" width="118.1" height="20" rx="2" fill="#787c7e"><title>Step 5 Preview: 3.95 gold a round from round payout</title></rect>
<text x="275.0" y="418.0" font-size="11" fill="#ffffff" text-anchor="middle">4.0</text>
<rect x="336.1" y="404" width="18.1" height="20" rx="2" fill="#538d4e"><title>Step 5 Preview: 0.66 gold a round from unused guesses</title></rect>
<rect x="356.2" y="404" width="2.9" height="20" rx="2" fill="#b59f3b"><title>Step 5 Preview: 0.16 gold a round from interest</title></rect>
<rect x="361.1" y="404" width="12.2" height="20" rx="2" fill="#2f6f8f"><title>Step 5 Preview: 0.47 gold a round from sales</title></rect>
<text x="382.3" y="414.0" font-size="11" fill="#2c2c2c" text-anchor="start">5.2 earned</text>
<text x="382.3" y="426.0" font-size="11" fill="#8a8878" text-anchor="start">5.2 spent</text>
<text x="0.0" y="449.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6 Luna</text>
<rect x="216.0" y="434" width="117.8" height="20" rx="2" fill="#787c7e"><title>GPT-6 Luna: 3.94 gold a round from round payout</title></rect>
<text x="274.9" y="448.0" font-size="11" fill="#ffffff" text-anchor="middle">3.9</text>
<rect x="335.8" y="434" width="35.8" height="20" rx="2" fill="#538d4e"><title>GPT-6 Luna: 1.24 gold a round from unused guesses</title></rect>
<text x="353.7" y="448.0" font-size="11" fill="#ffffff" text-anchor="middle">1.2</text>
<rect x="373.6" y="434" width="24.9" height="20" rx="2" fill="#b59f3b"><title>GPT-6 Luna: 0.89 gold a round from interest</title></rect>
<text x="386.0" y="448.0" font-size="11" fill="#ffffff" text-anchor="middle">0.9</text>
<rect x="400.5" y="434" width="36.6" height="20" rx="2" fill="#2f6f8f"><title>GPT-6 Luna: 1.27 gold a round from sales</title></rect>
<text x="418.8" y="448.0" font-size="11" fill="#ffffff" text-anchor="middle">1.3</text>
<rect x="439.1" y="434" width="1.0" height="20" rx="2" fill="#6b4c8a"><title>GPT-6 Luna: 0.09 gold a round from relic payouts</title></rect>
<text x="448.7" y="444.0" font-size="11" fill="#2c2c2c" text-anchor="start">7.4 earned</text>
<text x="448.7" y="456.0" font-size="11" fill="#8a8878" text-anchor="start">6.6 spent</text>
<text x="0.0" y="479.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Haiku 5.5</text>
<rect x="216.0" y="464" width="116.0" height="20" rx="2" fill="#787c7e"><title>Claude Haiku 5.5: 3.88 gold a round from round payout</title></rect>
<text x="274.0" y="478.0" font-size="11" fill="#ffffff" text-anchor="middle">3.9</text>
<rect x="334.0" y="464" width="49.6" height="20" rx="2" fill="#538d4e"><title>Claude Haiku 5.5: 1.70 gold a round from unused guesses</title></rect>
<text x="358.8" y="478.0" font-size="11" fill="#ffffff" text-anchor="middle">1.7</text>
<rect x="385.6" y="464" width="17.1" height="20" rx="2" fill="#b59f3b"><title>Claude Haiku 5.5: 0.63 gold a round from interest</title></rect>
<rect x="404.7" y="464" width="1.0" height="20" rx="2" fill="#8c0000"><title>Claude Haiku 5.5: 0.05 gold a round from scoring</title></rect>
<rect x="406.1" y="464" width="5.8" height="20" rx="2" fill="#2f6f8f"><title>Claude Haiku 5.5: 0.26 gold a round from sales</title></rect>
<text x="420.9" y="474.0" font-size="11" fill="#2c2c2c" text-anchor="start">6.5 earned</text>
<text x="420.9" y="486.0" font-size="11" fill="#8a8878" text-anchor="start">5.4 spent</text>
<text x="0.0" y="509.0" font-size="13" fill="#2c2c2c" text-anchor="start">Nemotron 3 Ultra</text>
<rect x="216.0" y="494" width="115.9" height="20" rx="2" fill="#787c7e"><title>Nemotron 3 Ultra: 3.88 gold a round from round payout</title></rect>
<text x="274.0" y="508.0" font-size="11" fill="#ffffff" text-anchor="middle">3.9</text>
<rect x="333.9" y="494" width="42.1" height="20" rx="2" fill="#538d4e"><title>Nemotron 3 Ultra: 1.45 gold a round from unused guesses</title></rect>
<text x="355.0" y="508.0" font-size="11" fill="#ffffff" text-anchor="middle">1.5</text>
<rect x="378.1" y="494" width="12.5" height="20" rx="2" fill="#b59f3b"><title>Nemotron 3 Ultra: 0.48 gold a round from interest</title></rect>
<rect x="392.5" y="494" width="16.8" height="20" rx="2" fill="#8c0000"><title>Nemotron 3 Ultra: 0.62 gold a round from scoring</title></rect>
<rect x="411.3" y="494" width="11.7" height="20" rx="2" fill="#2f6f8f"><title>Nemotron 3 Ultra: 0.45 gold a round from sales</title></rect>
<rect x="425.1" y="494" width="5.2" height="20" rx="2" fill="#6b4c8a"><title>Nemotron 3 Ultra: 0.24 gold a round from relic payouts</title></rect>
<text x="439.3" y="504.0" font-size="11" fill="#2c2c2c" text-anchor="start">7.1 earned</text>
<text x="439.3" y="516.0" font-size="11" fill="#8a8878" text-anchor="start">7.2 spent</text>
<text x="0.0" y="539.0" font-size="13" fill="#2c2c2c" text-anchor="start">MiniMax M3</text>
<rect x="216.0" y="524" width="117.4" height="20" rx="2" fill="#787c7e"><title>MiniMax M3: 3.93 gold a round from round payout</title></rect>
<text x="274.7" y="538.0" font-size="11" fill="#ffffff" text-anchor="middle">3.9</text>
<rect x="335.4" y="524" width="44.7" height="20" rx="2" fill="#538d4e"><title>MiniMax M3: 1.54 gold a round from unused guesses</title></rect>
<text x="357.7" y="538.0" font-size="11" fill="#ffffff" text-anchor="middle">1.5</text>
<rect x="382.0" y="524" width="24.0" height="20" rx="2" fill="#b59f3b"><title>MiniMax M3: 0.86 gold a round from interest</title></rect>
<text x="394.1" y="538.0" font-size="11" fill="#ffffff" text-anchor="middle">0.9</text>
<rect x="408.1" y="524" width="2.3" height="20" rx="2" fill="#8c0000"><title>MiniMax M3: 0.14 gold a round from scoring</title></rect>
<rect x="412.4" y="524" width="13.2" height="20" rx="2" fill="#2f6f8f"><title>MiniMax M3: 0.50 gold a round from sales</title></rect>
<text x="434.6" y="534.0" font-size="11" fill="#2c2c2c" text-anchor="start">7.0 earned</text>
<text x="434.6" y="546.0" font-size="11" fill="#8a8878" text-anchor="start">5.1 spent</text>
<text x="215.0" y="588.0" font-size="11" fill="#8a8878" text-anchor="start">gold earned per round cleared, by where it came from</text>
</svg>

### Guesses and gold by stage

The two are linked: every guess left unused pays gold when the round is cleared. Opus's income peaks in stages 4 and 5, at 11 to 12 gold a round, while Sonnet's on the same difficulty never passes 8.

<svg class="results" style="width:100%;height:auto;margin:1em 0" viewBox="0 0 720 1014" role="img" aria-labelledby="stages-title" font-family="inherit">
<title id="stages-title">Guesses per cleared round and gold earned per cleared round in each of the eight stages, for each model.</title>
<text x="0.0" y="12.0" font-size="13" fill="#2c2c2c" text-anchor="start" font-weight="700">Guesses</text>
<text x="246.6" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">stage 1</text>
<text x="309.7" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">stage 2</text>
<text x="372.8" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">stage 3</text>
<text x="435.9" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">stage 4</text>
<text x="499.1" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">stage 5</text>
<text x="562.2" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">stage 6</text>
<text x="625.3" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">stage 7</text>
<text x="688.4" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">stage 8</text>
<text x="0.0" y="28.0" font-size="12" fill="#8c0000" text-anchor="start" font-weight="700">Ascension 10, the hardest</text>
<text x="0.0" y="51.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Opus 5.5</text>
<rect x="216.0" y="36" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.33"><title>Claude Opus 5.5, stage 1: 3.64 guesses per cleared round over 14 rounds</title></rect>
<text x="246.6" y="51.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.6</text>
<rect x="279.1" y="36" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.26"><title>Claude Opus 5.5, stage 2: 3.42 guesses per cleared round over 12 rounds</title></rect>
<text x="309.7" y="51.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.4</text>
<rect x="342.3" y="36" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.36"><title>Claude Opus 5.5, stage 3: 3.75 guesses per cleared round over 12 rounds</title></rect>
<text x="372.8" y="51.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.8</text>
<rect x="405.4" y="36" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.31"><title>Claude Opus 5.5, stage 4: 3.58 guesses per cleared round over 12 rounds</title></rect>
<text x="435.9" y="51.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.6</text>
<rect x="468.5" y="36" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.36"><title>Claude Opus 5.5, stage 5: 3.75 guesses per cleared round over 12 rounds</title></rect>
<text x="499.1" y="51.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.8</text>
<rect x="531.6" y="36" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.29"><title>Claude Opus 5.5, stage 6: 3.50 guesses per cleared round over 12 rounds</title></rect>
<text x="562.2" y="51.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.5</text>
<rect x="594.8" y="36" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.31"><title>Claude Opus 5.5, stage 7: 3.58 guesses per cleared round over 12 rounds</title></rect>
<text x="625.3" y="51.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.6</text>
<rect x="657.9" y="36" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.27"><title>Claude Opus 5.5, stage 8: 3.45 guesses per cleared round over 11 rounds</title></rect>
<text x="688.4" y="51.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.5</text>
<text x="0.0" y="77.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Sonnet 5.5</text>
<rect x="216.0" y="62" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.54"><title>Claude Sonnet 5.5, stage 1: 4.40 guesses per cleared round over 15 rounds</title></rect>
<text x="246.6" y="77.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.4</text>
<rect x="279.1" y="62" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.26"><title>Claude Sonnet 5.5, stage 2: 3.40 guesses per cleared round over 15 rounds</title></rect>
<text x="309.7" y="77.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.4</text>
<rect x="342.3" y="62" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.45"><title>Claude Sonnet 5.5, stage 3: 4.07 guesses per cleared round over 15 rounds</title></rect>
<text x="372.8" y="77.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.1</text>
<rect x="405.4" y="62" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.39"><title>Claude Sonnet 5.5, stage 4: 3.87 guesses per cleared round over 15 rounds</title></rect>
<text x="435.9" y="77.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.9</text>
<rect x="468.5" y="62" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.35"><title>Claude Sonnet 5.5, stage 5: 3.73 guesses per cleared round over 15 rounds</title></rect>
<text x="499.1" y="77.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.7</text>
<rect x="531.6" y="62" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.38"><title>Claude Sonnet 5.5, stage 6: 3.82 guesses per cleared round over 11 rounds</title></rect>
<text x="562.2" y="77.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.8</text>
<rect x="594.8" y="62" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.54"><title>Claude Sonnet 5.5, stage 7: 4.38 guesses per cleared round over 8 rounds</title></rect>
<text x="625.3" y="77.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.4</text>
<rect x="657.9" y="62" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.36"><title>Claude Sonnet 5.5, stage 8: 3.75 guesses per cleared round over 4 rounds</title></rect>
<text x="688.4" y="77.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.8</text>
<text x="0.0" y="103.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6 Sol</text>
<rect x="216.0" y="88" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.35"><title>GPT-6 Sol, stage 1: 3.71 guesses per cleared round over 14 rounds</title></rect>
<text x="246.6" y="103.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.7</text>
<rect x="279.1" y="88" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.31"><title>GPT-6 Sol, stage 2: 3.58 guesses per cleared round over 12 rounds</title></rect>
<text x="309.7" y="103.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.6</text>
<rect x="342.3" y="88" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.48"><title>GPT-6 Sol, stage 3: 4.18 guesses per cleared round over 11 rounds</title></rect>
<text x="372.8" y="103.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.2</text>
<rect x="405.4" y="88" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.33"><title>GPT-6 Sol, stage 4: 3.67 guesses per cleared round over 9 rounds</title></rect>
<text x="435.9" y="103.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.7</text>
<rect x="468.5" y="88" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.43"><title>GPT-6 Sol, stage 5: 4.00 guesses per cleared round over 9 rounds</title></rect>
<text x="499.1" y="103.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.0</text>
<rect x="531.6" y="88" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.27"><title>GPT-6 Sol, stage 6: 3.43 guesses per cleared round over 7 rounds</title></rect>
<text x="562.2" y="103.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.4</text>
<rect x="594.8" y="88" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.52"><title>GPT-6 Sol, stage 7: 4.33 guesses per cleared round over 3 rounds</title></rect>
<text x="625.3" y="103.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.3</text>
<rect x="657.9" y="88" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.14"><title>GPT-6 Sol, stage 8: 3.00 guesses per cleared round over 3 rounds</title></rect>
<text x="688.4" y="103.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.0</text>
<text x="0.0" y="129.0" font-size="13" fill="#2c2c2c" text-anchor="start">MiMo-V2.6-Pro</text>
<rect x="216.0" y="114" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.45"><title>MiMo-V2.6-Pro, stage 1: 4.07 guesses per cleared round over 15 rounds</title></rect>
<text x="246.6" y="129.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.1</text>
<rect x="279.1" y="114" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.35"><title>MiMo-V2.6-Pro, stage 2: 3.71 guesses per cleared round over 14 rounds</title></rect>
<text x="309.7" y="129.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.7</text>
<rect x="342.3" y="114" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.45"><title>MiMo-V2.6-Pro, stage 3: 4.08 guesses per cleared round over 12 rounds</title></rect>
<text x="372.8" y="129.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.1</text>
<rect x="405.4" y="114" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.51"><title>MiMo-V2.6-Pro, stage 4: 4.27 guesses per cleared round over 11 rounds</title></rect>
<text x="435.9" y="129.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.3</text>
<rect x="468.5" y="114" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.39"><title>MiMo-V2.6-Pro, stage 5: 3.86 guesses per cleared round over 7 rounds</title></rect>
<text x="499.1" y="129.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.9</text>
<rect x="531.6" y="114" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.52"><title>MiMo-V2.6-Pro, stage 6: 4.33 guesses per cleared round over 3 rounds</title></rect>
<text x="562.2" y="129.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.3</text>
<rect x="594.8" y="114" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.43"><title>MiMo-V2.6-Pro, stage 7: 4.00 guesses per cleared round over 3 rounds</title></rect>
<text x="625.3" y="129.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.0</text>
<text x="0.0" y="155.0" font-size="13" fill="#2c2c2c" text-anchor="start">DeepSeek V4.1 Flash</text>
<rect x="216.0" y="140" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.53"><title>DeepSeek V4.1 Flash, stage 1: 4.36 guesses per cleared round over 14 rounds</title></rect>
<text x="246.6" y="155.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.4</text>
<rect x="279.1" y="140" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.38"><title>DeepSeek V4.1 Flash, stage 2: 3.83 guesses per cleared round over 12 rounds</title></rect>
<text x="309.7" y="155.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.8</text>
<rect x="342.3" y="140" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.52"><title>DeepSeek V4.1 Flash, stage 3: 4.33 guesses per cleared round over 12 rounds</title></rect>
<text x="372.8" y="155.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.3</text>
<rect x="405.4" y="140" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.56"><title>DeepSeek V4.1 Flash, stage 4: 4.45 guesses per cleared round over 11 rounds</title></rect>
<text x="435.9" y="155.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.5</text>
<rect x="468.5" y="140" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.40"><title>DeepSeek V4.1 Flash, stage 5: 3.89 guesses per cleared round over 9 rounds</title></rect>
<text x="499.1" y="155.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.9</text>
<rect x="531.6" y="140" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.14"><title>DeepSeek V4.1 Flash, stage 6: 3.00 guesses per cleared round over 4 rounds</title></rect>
<text x="562.2" y="155.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.0</text>
<text x="0.0" y="181.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6.1 Sol</text>
<rect x="216.0" y="166" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.39"><title>GPT-6.1 Sol, stage 1: 3.86 guesses per cleared round over 14 rounds</title></rect>
<text x="246.6" y="181.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.9</text>
<rect x="279.1" y="166" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.33"><title>GPT-6.1 Sol, stage 2: 3.67 guesses per cleared round over 12 rounds</title></rect>
<text x="309.7" y="181.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.7</text>
<rect x="342.3" y="166" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.45"><title>GPT-6.1 Sol, stage 3: 4.09 guesses per cleared round over 11 rounds</title></rect>
<text x="372.8" y="181.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.1</text>
<rect x="405.4" y="166" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.36"><title>GPT-6.1 Sol, stage 4: 3.75 guesses per cleared round over 8 rounds</title></rect>
<text x="435.9" y="181.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.8</text>
<rect x="468.5" y="166" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.33"><title>GPT-6.1 Sol, stage 5: 3.67 guesses per cleared round over 6 rounds</title></rect>
<text x="499.1" y="181.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.7</text>
<rect x="531.6" y="166" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.29"><title>GPT-6.1 Sol, stage 6: 3.50 guesses per cleared round over 6 rounds</title></rect>
<text x="562.2" y="181.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.5</text>
<rect x="594.8" y="166" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.62"><title>GPT-6.1 Sol, stage 7: 4.67 guesses per cleared round over 3 rounds</title></rect>
<text x="625.3" y="181.0" font-size="11" fill="#ffffff" text-anchor="middle">4.7</text>
<text x="0.0" y="207.0" font-size="13" fill="#2c2c2c" text-anchor="start">DeepSeek V4 Pro</text>
<rect x="216.0" y="192" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.35"><title>DeepSeek V4 Pro, stage 1: 3.73 guesses per cleared round over 15 rounds</title></rect>
<text x="246.6" y="207.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.7</text>
<rect x="279.1" y="192" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.26"><title>DeepSeek V4 Pro, stage 2: 3.42 guesses per cleared round over 12 rounds</title></rect>
<text x="309.7" y="207.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.4</text>
<rect x="342.3" y="192" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.43"><title>DeepSeek V4 Pro, stage 3: 4.00 guesses per cleared round over 12 rounds</title></rect>
<text x="372.8" y="207.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.0</text>
<rect x="405.4" y="192" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.26"><title>DeepSeek V4 Pro, stage 4: 3.40 guesses per cleared round over 10 rounds</title></rect>
<text x="435.9" y="207.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.4</text>
<rect x="468.5" y="192" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.43"><title>DeepSeek V4 Pro, stage 5: 4.00 guesses per cleared round over 5 rounds</title></rect>
<text x="499.1" y="207.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.0</text>
<rect x="531.6" y="192" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.43"><title>DeepSeek V4 Pro, stage 6: 4.00 guesses per cleared round over 2 rounds</title></rect>
<text x="562.2" y="207.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.0</text>
<text x="0.0" y="233.0" font-size="13" fill="#2c2c2c" text-anchor="start">GLM 5.3 Flash</text>
<rect x="216.0" y="218" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.45"><title>GLM 5.3 Flash, stage 1: 4.07 guesses per cleared round over 14 rounds</title></rect>
<text x="246.6" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.1</text>
<rect x="279.1" y="218" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.43"><title>GLM 5.3 Flash, stage 2: 4.00 guesses per cleared round over 12 rounds</title></rect>
<text x="309.7" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.0</text>
<rect x="342.3" y="218" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.27"><title>GLM 5.3 Flash, stage 3: 3.45 guesses per cleared round over 11 rounds</title></rect>
<text x="372.8" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.5</text>
<rect x="405.4" y="218" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.52"><title>GLM 5.3 Flash, stage 4: 4.33 guesses per cleared round over 6 rounds</title></rect>
<text x="435.9" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.3</text>
<rect x="468.5" y="218" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.71"><title>GLM 5.3 Flash, stage 5: 5.00 guesses per cleared round over 3 rounds</title></rect>
<text x="499.1" y="233.0" font-size="11" fill="#ffffff" text-anchor="middle">5.0</text>
<rect x="531.6" y="218" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.33"><title>GLM 5.3 Flash, stage 6: 3.67 guesses per cleared round over 3 rounds</title></rect>
<text x="562.2" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.7</text>
<rect x="594.8" y="218" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.62"><title>GLM 5.3 Flash, stage 7: 4.67 guesses per cleared round over 3 rounds</title></rect>
<text x="625.3" y="233.0" font-size="11" fill="#ffffff" text-anchor="middle">4.7</text>
<rect x="657.9" y="218" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.43"><title>GLM 5.3 Flash, stage 8: 4.00 guesses per cleared round over 3 rounds</title></rect>
<text x="688.4" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.0</text>
<text x="0.0" y="259.0" font-size="13" fill="#2c2c2c" text-anchor="start">Gemini 3.8 Flash</text>
<rect x="216.0" y="244" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.35"><title>Gemini 3.8 Flash, stage 1: 3.73 guesses per cleared round over 15 rounds</title></rect>
<text x="246.6" y="259.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.7</text>
<rect x="279.1" y="244" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.35"><title>Gemini 3.8 Flash, stage 2: 3.71 guesses per cleared round over 14 rounds</title></rect>
<text x="309.7" y="259.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.7</text>
<rect x="342.3" y="244" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.46"><title>Gemini 3.8 Flash, stage 3: 4.10 guesses per cleared round over 10 rounds</title></rect>
<text x="372.8" y="259.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.1</text>
<rect x="405.4" y="244" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.43"><title>Gemini 3.8 Flash, stage 4: 4.00 guesses per cleared round over 6 rounds</title></rect>
<text x="435.9" y="259.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.0</text>
<rect x="468.5" y="244" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.24"><title>Gemini 3.8 Flash, stage 5: 3.33 guesses per cleared round over 6 rounds</title></rect>
<text x="499.1" y="259.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.3</text>
<rect x="531.6" y="244" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.54"><title>Gemini 3.8 Flash, stage 6: 4.40 guesses per cleared round over 5 rounds</title></rect>
<text x="562.2" y="259.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.4</text>
<rect x="594.8" y="244" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.29"><title>Gemini 3.8 Flash, stage 7: 3.50 guesses per cleared round over 2 rounds</title></rect>
<text x="625.3" y="259.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.5</text>
<text x="0.0" y="290.0" font-size="12" fill="#8c0000" text-anchor="start" font-weight="700">Ascension 0, the easiest</text>
<text x="0.0" y="313.0" font-size="13" fill="#2c2c2c" text-anchor="start">Gemini 3.8 Flash</text>
<rect x="216.0" y="298" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.37"><title>Gemini 3.8 Flash, stage 1: 3.80 guesses per cleared round over 15 rounds</title></rect>
<text x="246.6" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.8</text>
<rect x="279.1" y="298" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.45"><title>Gemini 3.8 Flash, stage 2: 4.07 guesses per cleared round over 15 rounds</title></rect>
<text x="309.7" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.1</text>
<rect x="342.3" y="298" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.30"><title>Gemini 3.8 Flash, stage 3: 3.53 guesses per cleared round over 15 rounds</title></rect>
<text x="372.8" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.5</text>
<rect x="405.4" y="298" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.47"><title>Gemini 3.8 Flash, stage 4: 4.13 guesses per cleared round over 15 rounds</title></rect>
<text x="435.9" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.1</text>
<rect x="468.5" y="298" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.31"><title>Gemini 3.8 Flash, stage 5: 3.60 guesses per cleared round over 15 rounds</title></rect>
<text x="499.1" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.6</text>
<rect x="531.6" y="298" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.41"><title>Gemini 3.8 Flash, stage 6: 3.93 guesses per cleared round over 15 rounds</title></rect>
<text x="562.2" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.9</text>
<rect x="594.8" y="298" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.39"><title>Gemini 3.8 Flash, stage 7: 3.87 guesses per cleared round over 15 rounds</title></rect>
<text x="625.3" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.9</text>
<rect x="657.9" y="298" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.33"><title>Gemini 3.8 Flash, stage 8: 3.67 guesses per cleared round over 15 rounds</title></rect>
<text x="688.4" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.7</text>
<text x="0.0" y="339.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Sonnet 5.5</text>
<rect x="216.0" y="324" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.52"><title>Claude Sonnet 5.5, stage 1: 4.33 guesses per cleared round over 15 rounds</title></rect>
<text x="246.6" y="339.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.3</text>
<rect x="279.1" y="324" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.52"><title>Claude Sonnet 5.5, stage 2: 4.33 guesses per cleared round over 15 rounds</title></rect>
<text x="309.7" y="339.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.3</text>
<rect x="342.3" y="324" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Claude Sonnet 5.5, stage 3: 4.20 guesses per cleared round over 15 rounds</title></rect>
<text x="372.8" y="339.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.2</text>
<rect x="405.4" y="324" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.33"><title>Claude Sonnet 5.5, stage 4: 3.67 guesses per cleared round over 15 rounds</title></rect>
<text x="435.9" y="339.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.7</text>
<rect x="468.5" y="324" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.39"><title>Claude Sonnet 5.5, stage 5: 3.86 guesses per cleared round over 14 rounds</title></rect>
<text x="499.1" y="339.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.9</text>
<rect x="531.6" y="324" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.17"><title>Claude Sonnet 5.5, stage 6: 3.08 guesses per cleared round over 12 rounds</title></rect>
<text x="562.2" y="339.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.1</text>
<rect x="594.8" y="324" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.49"><title>Claude Sonnet 5.5, stage 7: 4.20 guesses per cleared round over 10 rounds</title></rect>
<text x="625.3" y="339.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.2</text>
<rect x="657.9" y="324" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.57"><title>Claude Sonnet 5.5, stage 8: 4.50 guesses per cleared round over 6 rounds</title></rect>
<text x="688.4" y="339.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.5</text>
<text x="0.0" y="365.0" font-size="13" fill="#2c2c2c" text-anchor="start">Step 5 Preview</text>
<rect x="216.0" y="350" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.74"><title>Step 5 Preview, stage 1: 5.08 guesses per cleared round over 13 rounds</title></rect>
<text x="246.6" y="365.0" font-size="11" fill="#ffffff" text-anchor="middle">5.1</text>
<rect x="279.1" y="350" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.79"><title>Step 5 Preview, stage 2: 5.25 guesses per cleared round over 12 rounds</title></rect>
<text x="309.7" y="365.0" font-size="11" fill="#ffffff" text-anchor="middle">5.3</text>
<rect x="342.3" y="350" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.79"><title>Step 5 Preview, stage 3: 5.25 guesses per cleared round over 12 rounds</title></rect>
<text x="372.8" y="365.0" font-size="11" fill="#ffffff" text-anchor="middle">5.3</text>
<rect x="405.4" y="350" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.81"><title>Step 5 Preview, stage 4: 5.33 guesses per cleared round over 12 rounds</title></rect>
<text x="435.9" y="365.0" font-size="11" fill="#ffffff" text-anchor="middle">5.3</text>
<rect x="468.5" y="350" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.87"><title>Step 5 Preview, stage 5: 5.55 guesses per cleared round over 11 rounds</title></rect>
<text x="499.1" y="365.0" font-size="11" fill="#ffffff" text-anchor="middle">5.5</text>
<rect x="531.6" y="350" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="1.00"><title>Step 5 Preview, stage 6: 6.00 guesses per cleared round over 2 rounds</title></rect>
<text x="562.2" y="365.0" font-size="11" fill="#ffffff" text-anchor="middle">6.0</text>
<text x="0.0" y="391.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6 Luna</text>
<rect x="216.0" y="376" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.53"><title>GPT-6 Luna, stage 1: 4.36 guesses per cleared round over 14 rounds</title></rect>
<text x="246.6" y="391.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.4</text>
<rect x="279.1" y="376" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.64"><title>GPT-6 Luna, stage 2: 4.75 guesses per cleared round over 12 rounds</title></rect>
<text x="309.7" y="391.0" font-size="11" fill="#ffffff" text-anchor="middle">4.8</text>
<rect x="342.3" y="376" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.69"><title>GPT-6 Luna, stage 3: 4.92 guesses per cleared round over 12 rounds</title></rect>
<text x="372.8" y="391.0" font-size="11" fill="#ffffff" text-anchor="middle">4.9</text>
<rect x="405.4" y="376" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.55"><title>GPT-6 Luna, stage 4: 4.42 guesses per cleared round over 12 rounds</title></rect>
<text x="435.9" y="391.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.4</text>
<rect x="468.5" y="376" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.75"><title>GPT-6 Luna, stage 5: 5.11 guesses per cleared round over 9 rounds</title></rect>
<text x="499.1" y="391.0" font-size="11" fill="#ffffff" text-anchor="middle">5.1</text>
<rect x="531.6" y="376" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.67"><title>GPT-6 Luna, stage 6: 4.83 guesses per cleared round over 6 rounds</title></rect>
<text x="562.2" y="391.0" font-size="11" fill="#ffffff" text-anchor="middle">4.8</text>
<rect x="594.8" y="376" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.81"><title>GPT-6 Luna, stage 7: 5.33 guesses per cleared round over 3 rounds</title></rect>
<text x="625.3" y="391.0" font-size="11" fill="#ffffff" text-anchor="middle">5.3</text>
<rect x="657.9" y="376" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.43"><title>GPT-6 Luna, stage 8: 4.00 guesses per cleared round over 2 rounds</title></rect>
<text x="688.4" y="391.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.0</text>
<text x="0.0" y="417.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Haiku 5.5</text>
<rect x="216.0" y="402" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.60"><title>Claude Haiku 5.5, stage 1: 4.62 guesses per cleared round over 13 rounds</title></rect>
<text x="246.6" y="417.0" font-size="11" fill="#ffffff" text-anchor="middle">4.6</text>
<rect x="279.1" y="402" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.56"><title>Claude Haiku 5.5, stage 2: 4.44 guesses per cleared round over 9 rounds</title></rect>
<text x="309.7" y="417.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.4</text>
<rect x="342.3" y="402" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.33"><title>Claude Haiku 5.5, stage 3: 3.67 guesses per cleared round over 9 rounds</title></rect>
<text x="372.8" y="417.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.7</text>
<rect x="405.4" y="402" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.43"><title>Claude Haiku 5.5, stage 4: 4.00 guesses per cleared round over 8 rounds</title></rect>
<text x="435.9" y="417.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.0</text>
<rect x="468.5" y="402" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.57"><title>Claude Haiku 5.5, stage 5: 4.50 guesses per cleared round over 4 rounds</title></rect>
<text x="499.1" y="417.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.5</text>
<text x="0.0" y="443.0" font-size="13" fill="#2c2c2c" text-anchor="start">Nemotron 3 Ultra</text>
<rect x="216.0" y="428" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.52"><title>Nemotron 3 Ultra, stage 1: 4.31 guesses per cleared round over 13 rounds</title></rect>
<text x="246.6" y="443.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.3</text>
<rect x="279.1" y="428" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.63"><title>Nemotron 3 Ultra, stage 2: 4.71 guesses per cleared round over 7 rounds</title></rect>
<text x="309.7" y="443.0" font-size="11" fill="#ffffff" text-anchor="middle">4.7</text>
<rect x="342.3" y="428" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.38"><title>Nemotron 3 Ultra, stage 3: 3.83 guesses per cleared round over 6 rounds</title></rect>
<text x="372.8" y="443.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.8</text>
<rect x="405.4" y="428" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.71"><title>Nemotron 3 Ultra, stage 4: 5.00 guesses per cleared round over 5 rounds</title></rect>
<text x="435.9" y="443.0" font-size="11" fill="#ffffff" text-anchor="middle">5.0</text>
<rect x="468.5" y="428" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.90"><title>Nemotron 3 Ultra, stage 5: 5.67 guesses per cleared round over 3 rounds</title></rect>
<text x="499.1" y="443.0" font-size="11" fill="#ffffff" text-anchor="middle">5.7</text>
<rect x="531.6" y="428" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.43"><title>Nemotron 3 Ultra, stage 6: 4.00 guesses per cleared round over 3 rounds</title></rect>
<text x="562.2" y="443.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.0</text>
<rect x="594.8" y="428" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.43"><title>Nemotron 3 Ultra, stage 7: 4.00 guesses per cleared round over 3 rounds</title></rect>
<text x="625.3" y="443.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.0</text>
<rect x="657.9" y="428" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.86"><title>Nemotron 3 Ultra, stage 8: 5.50 guesses per cleared round over 2 rounds</title></rect>
<text x="688.4" y="443.0" font-size="11" fill="#ffffff" text-anchor="middle">5.5</text>
<text x="0.0" y="469.0" font-size="13" fill="#2c2c2c" text-anchor="start">MiniMax M3</text>
<rect x="216.0" y="454" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.61"><title>MiniMax M3, stage 1: 4.63 guesses per cleared round over 8 rounds</title></rect>
<text x="246.6" y="469.0" font-size="11" fill="#ffffff" text-anchor="middle">4.6</text>
<rect x="279.1" y="454" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.52"><title>MiniMax M3, stage 2: 4.33 guesses per cleared round over 6 rounds</title></rect>
<text x="309.7" y="469.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.3</text>
<rect x="342.3" y="454" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.57"><title>MiniMax M3, stage 3: 4.50 guesses per cleared round over 6 rounds</title></rect>
<text x="372.8" y="469.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.5</text>
<rect x="405.4" y="454" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.49"><title>MiniMax M3, stage 4: 4.20 guesses per cleared round over 5 rounds</title></rect>
<text x="435.9" y="469.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.2</text>
<rect x="468.5" y="454" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.62"><title>MiniMax M3, stage 5: 4.67 guesses per cleared round over 3 rounds</title></rect>
<text x="499.1" y="469.0" font-size="11" fill="#ffffff" text-anchor="middle">4.7</text>
<text x="215.0" y="496.0" font-size="11" fill="#8a8878" text-anchor="start">guesses per cleared round in each stage · blank where no run got that far</text>
<text x="0.0" y="524.0" font-size="13" fill="#2c2c2c" text-anchor="start" font-weight="700">Gold</text>
<text x="246.6" y="524.0" font-size="11" fill="#8a8878" text-anchor="middle">stage 1</text>
<text x="309.7" y="524.0" font-size="11" fill="#8a8878" text-anchor="middle">stage 2</text>
<text x="372.8" y="524.0" font-size="11" fill="#8a8878" text-anchor="middle">stage 3</text>
<text x="435.9" y="524.0" font-size="11" fill="#8a8878" text-anchor="middle">stage 4</text>
<text x="499.1" y="524.0" font-size="11" fill="#8a8878" text-anchor="middle">stage 5</text>
<text x="562.2" y="524.0" font-size="11" fill="#8a8878" text-anchor="middle">stage 6</text>
<text x="625.3" y="524.0" font-size="11" fill="#8a8878" text-anchor="middle">stage 7</text>
<text x="688.4" y="524.0" font-size="11" fill="#8a8878" text-anchor="middle">stage 8</text>
<text x="0.0" y="540.0" font-size="12" fill="#8c0000" text-anchor="start" font-weight="700">Ascension 10, the hardest</text>
<text x="0.0" y="563.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Opus 5.5</text>
<rect x="216.0" y="548" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.18"><title>Claude Opus 5.5, stage 1: 5.6 gold earned per cleared round over 14 rounds</title></rect>
<text x="246.6" y="563.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.6</text>
<rect x="279.1" y="548" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.34"><title>Claude Opus 5.5, stage 2: 7.1 gold earned per cleared round over 12 rounds</title></rect>
<text x="309.7" y="563.0" font-size="11" fill="#2c2c2c" text-anchor="middle">7.1</text>
<rect x="342.3" y="548" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.44"><title>Claude Opus 5.5, stage 3: 8.0 gold earned per cleared round over 12 rounds</title></rect>
<text x="372.8" y="563.0" font-size="11" fill="#2c2c2c" text-anchor="middle">8.0</text>
<rect x="405.4" y="548" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.75"><title>Claude Opus 5.5, stage 4: 10.8 gold earned per cleared round over 12 rounds</title></rect>
<text x="435.9" y="563.0" font-size="11" fill="#ffffff" text-anchor="middle">10.8</text>
<rect x="468.5" y="548" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.94"><title>Claude Opus 5.5, stage 5: 12.4 gold earned per cleared round over 12 rounds</title></rect>
<text x="499.1" y="563.0" font-size="11" fill="#ffffff" text-anchor="middle">12.4</text>
<rect x="531.6" y="548" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.69"><title>Claude Opus 5.5, stage 6: 10.2 gold earned per cleared round over 12 rounds</title></rect>
<text x="562.2" y="563.0" font-size="11" fill="#ffffff" text-anchor="middle">10.2</text>
<rect x="594.8" y="548" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.59"><title>Claude Opus 5.5, stage 7: 9.3 gold earned per cleared round over 12 rounds</title></rect>
<text x="625.3" y="563.0" font-size="11" fill="#2c2c2c" text-anchor="middle">9.3</text>
<rect x="657.9" y="548" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.52"><title>Claude Opus 5.5, stage 8: 8.6 gold earned per cleared round over 11 rounds</title></rect>
<text x="688.4" y="563.0" font-size="11" fill="#2c2c2c" text-anchor="middle">8.6</text>
<text x="0.0" y="589.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Sonnet 5.5</text>
<rect x="216.0" y="574" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.08"><title>Claude Sonnet 5.5, stage 1: 4.7 gold earned per cleared round over 15 rounds</title></rect>
<text x="246.6" y="589.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.7</text>
<rect x="279.1" y="574" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.26"><title>Claude Sonnet 5.5, stage 2: 6.3 gold earned per cleared round over 15 rounds</title></rect>
<text x="309.7" y="589.0" font-size="11" fill="#2c2c2c" text-anchor="middle">6.3</text>
<rect x="342.3" y="574" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.39"><title>Claude Sonnet 5.5, stage 3: 7.5 gold earned per cleared round over 15 rounds</title></rect>
<text x="372.8" y="589.0" font-size="11" fill="#2c2c2c" text-anchor="middle">7.5</text>
<rect x="405.4" y="574" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.42"><title>Claude Sonnet 5.5, stage 4: 7.8 gold earned per cleared round over 15 rounds</title></rect>
<text x="435.9" y="589.0" font-size="11" fill="#2c2c2c" text-anchor="middle">7.8</text>
<rect x="468.5" y="574" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.39"><title>Claude Sonnet 5.5, stage 5: 7.5 gold earned per cleared round over 15 rounds</title></rect>
<text x="499.1" y="589.0" font-size="11" fill="#2c2c2c" text-anchor="middle">7.5</text>
<rect x="531.6" y="574" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.33"><title>Claude Sonnet 5.5, stage 6: 7.0 gold earned per cleared round over 11 rounds</title></rect>
<text x="562.2" y="589.0" font-size="11" fill="#2c2c2c" text-anchor="middle">7.0</text>
<rect x="594.8" y="574" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.29"><title>Claude Sonnet 5.5, stage 7: 6.6 gold earned per cleared round over 8 rounds</title></rect>
<text x="625.3" y="589.0" font-size="11" fill="#2c2c2c" text-anchor="middle">6.6</text>
<rect x="657.9" y="574" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.17"><title>Claude Sonnet 5.5, stage 8: 5.5 gold earned per cleared round over 4 rounds</title></rect>
<text x="688.4" y="589.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.5</text>
<text x="0.0" y="615.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6 Sol</text>
<rect x="216.0" y="600" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.13"><title>GPT-6 Sol, stage 1: 5.2 gold earned per cleared round over 14 rounds</title></rect>
<text x="246.6" y="615.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.2</text>
<rect x="279.1" y="600" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.17"><title>GPT-6 Sol, stage 2: 5.5 gold earned per cleared round over 12 rounds</title></rect>
<text x="309.7" y="615.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.5</text>
<rect x="342.3" y="600" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.16"><title>GPT-6 Sol, stage 3: 5.5 gold earned per cleared round over 11 rounds</title></rect>
<text x="372.8" y="615.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.5</text>
<rect x="405.4" y="600" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.35"><title>GPT-6 Sol, stage 4: 7.1 gold earned per cleared round over 9 rounds</title></rect>
<text x="435.9" y="615.0" font-size="11" fill="#2c2c2c" text-anchor="middle">7.1</text>
<rect x="468.5" y="600" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.58"><title>GPT-6 Sol, stage 5: 9.2 gold earned per cleared round over 9 rounds</title></rect>
<text x="499.1" y="615.0" font-size="11" fill="#2c2c2c" text-anchor="middle">9.2</text>
<rect x="531.6" y="600" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.40"><title>GPT-6 Sol, stage 6: 7.6 gold earned per cleared round over 7 rounds</title></rect>
<text x="562.2" y="615.0" font-size="11" fill="#2c2c2c" text-anchor="middle">7.6</text>
<rect x="594.8" y="600" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.96"><title>GPT-6 Sol, stage 7: 12.7 gold earned per cleared round over 3 rounds</title></rect>
<text x="625.3" y="615.0" font-size="11" fill="#ffffff" text-anchor="middle">12.7</text>
<rect x="657.9" y="600" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.44"><title>GPT-6 Sol, stage 8: 8.0 gold earned per cleared round over 3 rounds</title></rect>
<text x="688.4" y="615.0" font-size="11" fill="#2c2c2c" text-anchor="middle">8.0</text>
<text x="0.0" y="641.0" font-size="13" fill="#2c2c2c" text-anchor="start">MiMo-V2.6-Pro</text>
<rect x="216.0" y="626" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.13"><title>MiMo-V2.6-Pro, stage 1: 5.1 gold earned per cleared round over 15 rounds</title></rect>
<text x="246.6" y="641.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.1</text>
<rect x="279.1" y="626" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.15"><title>MiMo-V2.6-Pro, stage 2: 5.4 gold earned per cleared round over 14 rounds</title></rect>
<text x="309.7" y="641.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.4</text>
<rect x="342.3" y="626" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.22"><title>MiMo-V2.6-Pro, stage 3: 6.0 gold earned per cleared round over 12 rounds</title></rect>
<text x="372.8" y="641.0" font-size="11" fill="#2c2c2c" text-anchor="middle">6.0</text>
<rect x="405.4" y="626" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.09"><title>MiMo-V2.6-Pro, stage 4: 4.8 gold earned per cleared round over 11 rounds</title></rect>
<text x="435.9" y="641.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.8</text>
<rect x="468.5" y="626" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.21"><title>MiMo-V2.6-Pro, stage 5: 5.9 gold earned per cleared round over 7 rounds</title></rect>
<text x="499.1" y="641.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.9</text>
<rect x="531.6" y="626" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.19"><title>MiMo-V2.6-Pro, stage 6: 5.7 gold earned per cleared round over 3 rounds</title></rect>
<text x="562.2" y="641.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.7</text>
<rect x="594.8" y="626" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.19"><title>MiMo-V2.6-Pro, stage 7: 5.7 gold earned per cleared round over 3 rounds</title></rect>
<text x="625.3" y="641.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.7</text>
<text x="0.0" y="667.0" font-size="13" fill="#2c2c2c" text-anchor="start">DeepSeek V4.1 Flash</text>
<rect x="216.0" y="652" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>DeepSeek V4.1 Flash, stage 1: 4.6 gold earned per cleared round over 14 rounds</title></rect>
<text x="246.6" y="667.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.6</text>
<rect x="279.1" y="652" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.15"><title>DeepSeek V4.1 Flash, stage 2: 5.3 gold earned per cleared round over 12 rounds</title></rect>
<text x="309.7" y="667.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.3</text>
<rect x="342.3" y="652" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.14"><title>DeepSeek V4.1 Flash, stage 3: 5.3 gold earned per cleared round over 12 rounds</title></rect>
<text x="372.8" y="667.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.3</text>
<rect x="405.4" y="652" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.11"><title>DeepSeek V4.1 Flash, stage 4: 5.0 gold earned per cleared round over 11 rounds</title></rect>
<text x="435.9" y="667.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.0</text>
<rect x="468.5" y="652" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.19"><title>DeepSeek V4.1 Flash, stage 5: 5.7 gold earned per cleared round over 9 rounds</title></rect>
<text x="499.1" y="667.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.7</text>
<rect x="531.6" y="652" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.17"><title>DeepSeek V4.1 Flash, stage 6: 5.5 gold earned per cleared round over 4 rounds</title></rect>
<text x="562.2" y="667.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.5</text>
<text x="0.0" y="693.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6.1 Sol</text>
<rect x="216.0" y="678" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.21"><title>GPT-6.1 Sol, stage 1: 5.9 gold earned per cleared round over 14 rounds</title></rect>
<text x="246.6" y="693.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.9</text>
<rect x="279.1" y="678" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.19"><title>GPT-6.1 Sol, stage 2: 5.8 gold earned per cleared round over 12 rounds</title></rect>
<text x="309.7" y="693.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.8</text>
<rect x="342.3" y="678" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.40"><title>GPT-6.1 Sol, stage 3: 7.6 gold earned per cleared round over 11 rounds</title></rect>
<text x="372.8" y="693.0" font-size="11" fill="#2c2c2c" text-anchor="middle">7.6</text>
<rect x="405.4" y="678" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.14"><title>GPT-6.1 Sol, stage 4: 5.3 gold earned per cleared round over 8 rounds</title></rect>
<text x="435.9" y="693.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.3</text>
<rect x="468.5" y="678" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.39"><title>GPT-6.1 Sol, stage 5: 7.5 gold earned per cleared round over 6 rounds</title></rect>
<text x="499.1" y="693.0" font-size="11" fill="#2c2c2c" text-anchor="middle">7.5</text>
<rect x="531.6" y="678" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.33"><title>GPT-6.1 Sol, stage 6: 7.0 gold earned per cleared round over 6 rounds</title></rect>
<text x="562.2" y="693.0" font-size="11" fill="#2c2c2c" text-anchor="middle">7.0</text>
<rect x="594.8" y="678" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.30"><title>GPT-6.1 Sol, stage 7: 6.7 gold earned per cleared round over 3 rounds</title></rect>
<text x="625.3" y="693.0" font-size="11" fill="#2c2c2c" text-anchor="middle">6.7</text>
<text x="0.0" y="719.0" font-size="13" fill="#2c2c2c" text-anchor="start">DeepSeek V4 Pro</text>
<rect x="216.0" y="704" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.17"><title>DeepSeek V4 Pro, stage 1: 5.5 gold earned per cleared round over 15 rounds</title></rect>
<text x="246.6" y="719.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.5</text>
<rect x="279.1" y="704" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.43"><title>DeepSeek V4 Pro, stage 2: 7.8 gold earned per cleared round over 12 rounds</title></rect>
<text x="309.7" y="719.0" font-size="11" fill="#2c2c2c" text-anchor="middle">7.8</text>
<rect x="342.3" y="704" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.55"><title>DeepSeek V4 Pro, stage 3: 8.9 gold earned per cleared round over 12 rounds</title></rect>
<text x="372.8" y="719.0" font-size="11" fill="#2c2c2c" text-anchor="middle">8.9</text>
<rect x="405.4" y="704" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.76"><title>DeepSeek V4 Pro, stage 4: 10.8 gold earned per cleared round over 10 rounds</title></rect>
<text x="435.9" y="719.0" font-size="11" fill="#ffffff" text-anchor="middle">10.8</text>
<rect x="468.5" y="704" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.44"><title>DeepSeek V4 Pro, stage 5: 8.0 gold earned per cleared round over 5 rounds</title></rect>
<text x="499.1" y="719.0" font-size="11" fill="#2c2c2c" text-anchor="middle">8.0</text>
<rect x="531.6" y="704" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.89"><title>DeepSeek V4 Pro, stage 6: 12.0 gold earned per cleared round over 2 rounds</title></rect>
<text x="562.2" y="719.0" font-size="11" fill="#ffffff" text-anchor="middle">12.0</text>
<text x="0.0" y="745.0" font-size="13" fill="#2c2c2c" text-anchor="start">GLM 5.3 Flash</text>
<rect x="216.0" y="730" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.12"><title>GLM 5.3 Flash, stage 1: 5.1 gold earned per cleared round over 14 rounds</title></rect>
<text x="246.6" y="745.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.1</text>
<rect x="279.1" y="730" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.13"><title>GLM 5.3 Flash, stage 2: 5.2 gold earned per cleared round over 12 rounds</title></rect>
<text x="309.7" y="745.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.2</text>
<rect x="342.3" y="730" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.33"><title>GLM 5.3 Flash, stage 3: 7.0 gold earned per cleared round over 11 rounds</title></rect>
<text x="372.8" y="745.0" font-size="11" fill="#2c2c2c" text-anchor="middle">7.0</text>
<rect x="405.4" y="730" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.17"><title>GLM 5.3 Flash, stage 4: 5.5 gold earned per cleared round over 6 rounds</title></rect>
<text x="435.9" y="745.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.5</text>
<rect x="468.5" y="730" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.07"><title>GLM 5.3 Flash, stage 5: 4.7 gold earned per cleared round over 3 rounds</title></rect>
<text x="499.1" y="745.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.7</text>
<rect x="531.6" y="730" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.07"><title>GLM 5.3 Flash, stage 6: 4.7 gold earned per cleared round over 3 rounds</title></rect>
<text x="562.2" y="745.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.7</text>
<rect x="594.8" y="730" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>GLM 5.3 Flash, stage 7: 4.3 gold earned per cleared round over 3 rounds</title></rect>
<text x="625.3" y="745.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.3</text>
<rect x="657.9" y="730" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>GLM 5.3 Flash, stage 8: 4.0 gold earned per cleared round over 3 rounds</title></rect>
<text x="688.4" y="745.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.0</text>
<text x="0.0" y="771.0" font-size="13" fill="#2c2c2c" text-anchor="start">Gemini 3.8 Flash</text>
<rect x="216.0" y="756" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.21"><title>Gemini 3.8 Flash, stage 1: 5.9 gold earned per cleared round over 15 rounds</title></rect>
<text x="246.6" y="771.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.9</text>
<rect x="279.1" y="756" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.32"><title>Gemini 3.8 Flash, stage 2: 6.9 gold earned per cleared round over 14 rounds</title></rect>
<text x="309.7" y="771.0" font-size="11" fill="#2c2c2c" text-anchor="middle">6.9</text>
<rect x="342.3" y="756" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.48"><title>Gemini 3.8 Flash, stage 3: 8.3 gold earned per cleared round over 10 rounds</title></rect>
<text x="372.8" y="771.0" font-size="11" fill="#2c2c2c" text-anchor="middle">8.3</text>
<rect x="405.4" y="756" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.41"><title>Gemini 3.8 Flash, stage 4: 7.7 gold earned per cleared round over 6 rounds</title></rect>
<text x="435.9" y="771.0" font-size="11" fill="#2c2c2c" text-anchor="middle">7.7</text>
<rect x="468.5" y="756" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.57"><title>Gemini 3.8 Flash, stage 5: 9.2 gold earned per cleared round over 6 rounds</title></rect>
<text x="499.1" y="771.0" font-size="11" fill="#2c2c2c" text-anchor="middle">9.2</text>
<rect x="531.6" y="756" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.36"><title>Gemini 3.8 Flash, stage 6: 7.2 gold earned per cleared round over 5 rounds</title></rect>
<text x="562.2" y="771.0" font-size="11" fill="#2c2c2c" text-anchor="middle">7.2</text>
<rect x="594.8" y="756" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.50"><title>Gemini 3.8 Flash, stage 7: 8.5 gold earned per cleared round over 2 rounds</title></rect>
<text x="625.3" y="771.0" font-size="11" fill="#2c2c2c" text-anchor="middle">8.5</text>
<text x="0.0" y="802.0" font-size="12" fill="#8c0000" text-anchor="start" font-weight="700">Ascension 0, the easiest</text>
<text x="0.0" y="825.0" font-size="13" fill="#2c2c2c" text-anchor="start">Gemini 3.8 Flash</text>
<rect x="216.0" y="810" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.44"><title>Gemini 3.8 Flash, stage 1: 7.9 gold earned per cleared round over 15 rounds</title></rect>
<text x="246.6" y="825.0" font-size="11" fill="#2c2c2c" text-anchor="middle">7.9</text>
<rect x="279.1" y="810" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.72"><title>Gemini 3.8 Flash, stage 2: 10.5 gold earned per cleared round over 15 rounds</title></rect>
<text x="309.7" y="825.0" font-size="11" fill="#ffffff" text-anchor="middle">10.5</text>
<rect x="342.3" y="810" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="1.00"><title>Gemini 3.8 Flash, stage 3: 13.3 gold earned per cleared round over 15 rounds</title></rect>
<text x="372.8" y="825.0" font-size="11" fill="#ffffff" text-anchor="middle">13.3</text>
<rect x="405.4" y="810" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.96"><title>Gemini 3.8 Flash, stage 4: 12.6 gold earned per cleared round over 15 rounds</title></rect>
<text x="435.9" y="825.0" font-size="11" fill="#ffffff" text-anchor="middle">12.6</text>
<rect x="468.5" y="810" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="1.00"><title>Gemini 3.8 Flash, stage 5: 13.5 gold earned per cleared round over 15 rounds</title></rect>
<text x="499.1" y="825.0" font-size="11" fill="#ffffff" text-anchor="middle">13.5</text>
<rect x="531.6" y="810" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="1.00"><title>Gemini 3.8 Flash, stage 6: 14.0 gold earned per cleared round over 15 rounds</title></rect>
<text x="562.2" y="825.0" font-size="11" fill="#ffffff" text-anchor="middle">14.0</text>
<rect x="594.8" y="810" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.92"><title>Gemini 3.8 Flash, stage 7: 12.3 gold earned per cleared round over 15 rounds</title></rect>
<text x="625.3" y="825.0" font-size="11" fill="#ffffff" text-anchor="middle">12.3</text>
<rect x="657.9" y="810" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="1.00"><title>Gemini 3.8 Flash, stage 8: 13.1 gold earned per cleared round over 15 rounds</title></rect>
<text x="688.4" y="825.0" font-size="11" fill="#ffffff" text-anchor="middle">13.1</text>
<text x="0.0" y="851.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Sonnet 5.5</text>
<rect x="216.0" y="836" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.19"><title>Claude Sonnet 5.5, stage 1: 5.7 gold earned per cleared round over 15 rounds</title></rect>
<text x="246.6" y="851.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.7</text>
<rect x="279.1" y="836" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.20"><title>Claude Sonnet 5.5, stage 2: 5.8 gold earned per cleared round over 15 rounds</title></rect>
<text x="309.7" y="851.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.8</text>
<rect x="342.3" y="836" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.24"><title>Claude Sonnet 5.5, stage 3: 6.1 gold earned per cleared round over 15 rounds</title></rect>
<text x="372.8" y="851.0" font-size="11" fill="#2c2c2c" text-anchor="middle">6.1</text>
<rect x="405.4" y="836" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.47"><title>Claude Sonnet 5.5, stage 4: 8.2 gold earned per cleared round over 15 rounds</title></rect>
<text x="435.9" y="851.0" font-size="11" fill="#2c2c2c" text-anchor="middle">8.2</text>
<rect x="468.5" y="836" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.68"><title>Claude Sonnet 5.5, stage 5: 10.1 gold earned per cleared round over 14 rounds</title></rect>
<text x="499.1" y="851.0" font-size="11" fill="#ffffff" text-anchor="middle">10.1</text>
<rect x="531.6" y="836" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.69"><title>Claude Sonnet 5.5, stage 6: 10.3 gold earned per cleared round over 12 rounds</title></rect>
<text x="562.2" y="851.0" font-size="11" fill="#ffffff" text-anchor="middle">10.3</text>
<rect x="594.8" y="836" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.77"><title>Claude Sonnet 5.5, stage 7: 10.9 gold earned per cleared round over 10 rounds</title></rect>
<text x="625.3" y="851.0" font-size="11" fill="#ffffff" text-anchor="middle">10.9</text>
<rect x="657.9" y="836" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.76"><title>Claude Sonnet 5.5, stage 8: 10.8 gold earned per cleared round over 6 rounds</title></rect>
<text x="688.4" y="851.0" font-size="11" fill="#ffffff" text-anchor="middle">10.8</text>
<text x="0.0" y="877.0" font-size="13" fill="#2c2c2c" text-anchor="start">Step 5 Preview</text>
<rect x="216.0" y="862" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.10"><title>Step 5 Preview, stage 1: 4.9 gold earned per cleared round over 13 rounds</title></rect>
<text x="246.6" y="877.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.9</text>
<rect x="279.1" y="862" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.08"><title>Step 5 Preview, stage 2: 4.8 gold earned per cleared round over 12 rounds</title></rect>
<text x="309.7" y="877.0" font-size="11" fill="#2c2c2c" text-anchor="middle">4.8</text>
<rect x="342.3" y="862" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.20"><title>Step 5 Preview, stage 3: 5.8 gold earned per cleared round over 12 rounds</title></rect>
<text x="372.8" y="877.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.8</text>
<rect x="405.4" y="862" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.13"><title>Step 5 Preview, stage 4: 5.2 gold earned per cleared round over 12 rounds</title></rect>
<text x="435.9" y="877.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.2</text>
<rect x="468.5" y="862" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.21"><title>Step 5 Preview, stage 5: 5.9 gold earned per cleared round over 11 rounds</title></rect>
<text x="499.1" y="877.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.9</text>
<rect x="531.6" y="862" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Step 5 Preview, stage 6: 3.5 gold earned per cleared round over 2 rounds</title></rect>
<text x="562.2" y="877.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.5</text>
<text x="0.0" y="903.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6 Luna</text>
<rect x="216.0" y="888" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.19"><title>GPT-6 Luna, stage 1: 5.7 gold earned per cleared round over 14 rounds</title></rect>
<text x="246.6" y="903.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.7</text>
<rect x="279.1" y="888" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.21"><title>GPT-6 Luna, stage 2: 5.9 gold earned per cleared round over 12 rounds</title></rect>
<text x="309.7" y="903.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.9</text>
<rect x="342.3" y="888" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.40"><title>GPT-6 Luna, stage 3: 7.6 gold earned per cleared round over 12 rounds</title></rect>
<text x="372.8" y="903.0" font-size="11" fill="#2c2c2c" text-anchor="middle">7.6</text>
<rect x="405.4" y="888" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.41"><title>GPT-6 Luna, stage 4: 7.7 gold earned per cleared round over 12 rounds</title></rect>
<text x="435.9" y="903.0" font-size="11" fill="#2c2c2c" text-anchor="middle">7.7</text>
<rect x="468.5" y="888" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.56"><title>GPT-6 Luna, stage 5: 9.0 gold earned per cleared round over 9 rounds</title></rect>
<text x="499.1" y="903.0" font-size="11" fill="#2c2c2c" text-anchor="middle">9.0</text>
<rect x="531.6" y="888" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.48"><title>GPT-6 Luna, stage 6: 8.3 gold earned per cleared round over 6 rounds</title></rect>
<text x="562.2" y="903.0" font-size="11" fill="#2c2c2c" text-anchor="middle">8.3</text>
<rect x="594.8" y="888" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.70"><title>GPT-6 Luna, stage 7: 10.3 gold earned per cleared round over 3 rounds</title></rect>
<text x="625.3" y="903.0" font-size="11" fill="#ffffff" text-anchor="middle">10.3</text>
<rect x="657.9" y="888" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.89"><title>GPT-6 Luna, stage 8: 12.0 gold earned per cleared round over 2 rounds</title></rect>
<text x="688.4" y="903.0" font-size="11" fill="#ffffff" text-anchor="middle">12.0</text>
<text x="0.0" y="929.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Haiku 5.5</text>
<rect x="216.0" y="914" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.15"><title>Claude Haiku 5.5, stage 1: 5.3 gold earned per cleared round over 13 rounds</title></rect>
<text x="246.6" y="929.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.3</text>
<rect x="279.1" y="914" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.25"><title>Claude Haiku 5.5, stage 2: 6.2 gold earned per cleared round over 9 rounds</title></rect>
<text x="309.7" y="929.0" font-size="11" fill="#2c2c2c" text-anchor="middle">6.2</text>
<rect x="342.3" y="914" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.30"><title>Claude Haiku 5.5, stage 3: 6.7 gold earned per cleared round over 9 rounds</title></rect>
<text x="372.8" y="929.0" font-size="11" fill="#2c2c2c" text-anchor="middle">6.7</text>
<rect x="405.4" y="914" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.38"><title>Claude Haiku 5.5, stage 4: 7.4 gold earned per cleared round over 8 rounds</title></rect>
<text x="435.9" y="929.0" font-size="11" fill="#2c2c2c" text-anchor="middle">7.4</text>
<rect x="468.5" y="914" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.56"><title>Claude Haiku 5.5, stage 5: 9.0 gold earned per cleared round over 4 rounds</title></rect>
<text x="499.1" y="929.0" font-size="11" fill="#2c2c2c" text-anchor="middle">9.0</text>
<text x="0.0" y="955.0" font-size="13" fill="#2c2c2c" text-anchor="start">Nemotron 3 Ultra</text>
<rect x="216.0" y="940" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.21"><title>Nemotron 3 Ultra, stage 1: 5.8 gold earned per cleared round over 13 rounds</title></rect>
<text x="246.6" y="955.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.8</text>
<rect x="279.1" y="940" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.27"><title>Nemotron 3 Ultra, stage 2: 6.4 gold earned per cleared round over 7 rounds</title></rect>
<text x="309.7" y="955.0" font-size="11" fill="#2c2c2c" text-anchor="middle">6.4</text>
<rect x="342.3" y="940" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.44"><title>Nemotron 3 Ultra, stage 3: 8.0 gold earned per cleared round over 6 rounds</title></rect>
<text x="372.8" y="955.0" font-size="11" fill="#2c2c2c" text-anchor="middle">8.0</text>
<rect x="405.4" y="940" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.36"><title>Nemotron 3 Ultra, stage 4: 7.2 gold earned per cleared round over 5 rounds</title></rect>
<text x="435.9" y="955.0" font-size="11" fill="#2c2c2c" text-anchor="middle">7.2</text>
<rect x="468.5" y="940" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.48"><title>Nemotron 3 Ultra, stage 5: 8.3 gold earned per cleared round over 3 rounds</title></rect>
<text x="499.1" y="955.0" font-size="11" fill="#2c2c2c" text-anchor="middle">8.3</text>
<rect x="531.6" y="940" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.63"><title>Nemotron 3 Ultra, stage 6: 9.7 gold earned per cleared round over 3 rounds</title></rect>
<text x="562.2" y="955.0" font-size="11" fill="#ffffff" text-anchor="middle">9.7</text>
<rect x="594.8" y="940" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.41"><title>Nemotron 3 Ultra, stage 7: 7.7 gold earned per cleared round over 3 rounds</title></rect>
<text x="625.3" y="955.0" font-size="11" fill="#2c2c2c" text-anchor="middle">7.7</text>
<rect x="657.9" y="940" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.50"><title>Nemotron 3 Ultra, stage 8: 8.5 gold earned per cleared round over 2 rounds</title></rect>
<text x="688.4" y="955.0" font-size="11" fill="#2c2c2c" text-anchor="middle">8.5</text>
<text x="0.0" y="981.0" font-size="13" fill="#2c2c2c" text-anchor="start">MiniMax M3</text>
<rect x="216.0" y="966" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.22"><title>MiniMax M3, stage 1: 6.0 gold earned per cleared round over 8 rounds</title></rect>
<text x="246.6" y="981.0" font-size="11" fill="#2c2c2c" text-anchor="middle">6.0</text>
<rect x="279.1" y="966" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.20"><title>MiniMax M3, stage 2: 5.8 gold earned per cleared round over 6 rounds</title></rect>
<text x="309.7" y="981.0" font-size="11" fill="#2c2c2c" text-anchor="middle">5.8</text>
<rect x="342.3" y="966" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.41"><title>MiniMax M3, stage 3: 7.7 gold earned per cleared round over 6 rounds</title></rect>
<text x="372.8" y="981.0" font-size="11" fill="#2c2c2c" text-anchor="middle">7.7</text>
<rect x="405.4" y="966" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.60"><title>MiniMax M3, stage 4: 9.4 gold earned per cleared round over 5 rounds</title></rect>
<text x="435.9" y="981.0" font-size="11" fill="#ffffff" text-anchor="middle">9.4</text>
<rect x="468.5" y="966" width="61.1" height="22" rx="2" fill="#8c0000" fill-opacity="0.26"><title>MiniMax M3, stage 5: 6.3 gold earned per cleared round over 3 rounds</title></rect>
<text x="499.1" y="981.0" font-size="11" fill="#2c2c2c" text-anchor="middle">6.3</text>
<text x="215.0" y="1008.0" font-size="11" fill="#8a8878" text-anchor="start">gold earned per cleared round in each stage, from every source</text>
</svg>

## Cost and speed

A run of Opus at ascension 10 cost about four dollars and took about eleven minutes. Runs are not all the same length, though, so the fair unit is the round: every round a model played, the one it died on included.

<svg class="results" style="width:100%;height:auto;margin:1em 0" viewBox="0 0 720 384" role="img" aria-labelledby="cost-title" font-family="inherit">
<title id="cost-title">Cost per round played, in US cents, for each model that was priced.</title>
<text x="0.0" y="14.0" font-size="12" fill="#8c0000" text-anchor="start" font-weight="700">Ascension 10, the hardest</text>
<text x="0.0" y="35.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Opus 5.5</text>
<rect x="215" y="24" width="355.0" height="14" rx="2" fill="#8c0000"><title>Claude Opus 5.5: 20.6 cents per round</title></rect>
<text x="578.0" y="35.0" font-size="12" fill="#2c2c2c" text-anchor="start">20.6¢</text>
<text x="0.0" y="59.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Sonnet 5.5</text>
<rect x="215" y="48" width="232.6" height="14" rx="2" fill="#8c0000"><title>Claude Sonnet 5.5: 13.5 cents per round</title></rect>
<text x="455.6" y="59.0" font-size="12" fill="#2c2c2c" text-anchor="start">13.5¢</text>
<text x="0.0" y="83.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6 Sol</text>
<rect x="215" y="72" width="156.8" height="14" rx="2" fill="#8c0000"><title>GPT-6 Sol: 9.1 cents per round</title></rect>
<text x="379.8" y="83.0" font-size="12" fill="#2c2c2c" text-anchor="start">9.1¢</text>
<text x="0.0" y="107.0" font-size="13" fill="#2c2c2c" text-anchor="start">MiMo-V2.6-Pro</text>
<rect x="215" y="96" width="105.1" height="14" rx="2" fill="#8c0000"><title>MiMo-V2.6-Pro: 6.1 cents per round</title></rect>
<text x="328.1" y="107.0" font-size="12" fill="#2c2c2c" text-anchor="start">6.1¢</text>
<text x="0.0" y="131.0" font-size="13" fill="#2c2c2c" text-anchor="start">DeepSeek V4.1 Flash</text>
<rect x="215" y="120" width="53.4" height="14" rx="2" fill="#8c0000"><title>DeepSeek V4.1 Flash: 3.1 cents per round</title></rect>
<text x="276.4" y="131.0" font-size="12" fill="#2c2c2c" text-anchor="start">3.1¢</text>
<text x="0.0" y="155.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6.1 Sol</text>
<rect x="215" y="144" width="74.1" height="14" rx="2" fill="#8c0000"><title>GPT-6.1 Sol: 4.3 cents per round</title></rect>
<text x="297.1" y="155.0" font-size="12" fill="#2c2c2c" text-anchor="start">4.3¢</text>
<text x="0.0" y="179.0" font-size="13" fill="#2c2c2c" text-anchor="start">DeepSeek V4 Pro</text>
<rect x="215" y="168" width="101.7" height="14" rx="2" fill="#8c0000"><title>DeepSeek V4 Pro: 5.9 cents per round</title></rect>
<text x="324.7" y="179.0" font-size="12" fill="#2c2c2c" text-anchor="start">5.9¢</text>
<text x="0.0" y="203.0" font-size="13" fill="#2c2c2c" text-anchor="start">GLM 5.3 Flash</text>
<rect x="215" y="192" width="79.3" height="14" rx="2" fill="#8c0000"><title>GLM 5.3 Flash: 4.6 cents per round</title></rect>
<text x="302.3" y="203.0" font-size="12" fill="#2c2c2c" text-anchor="start">4.6¢</text>
<text x="0.0" y="227.0" font-size="13" fill="#2c2c2c" text-anchor="start">Gemini 3.8 Flash</text>
<rect x="215" y="216" width="125.8" height="14" rx="2" fill="#8c0000"><title>Gemini 3.8 Flash: 7.3 cents per round</title></rect>
<text x="348.8" y="227.0" font-size="12" fill="#2c2c2c" text-anchor="start">7.3¢</text>
<text x="0.0" y="258.0" font-size="12" fill="#8c0000" text-anchor="start" font-weight="700">Ascension 0, the easiest</text>
<text x="0.0" y="279.0" font-size="13" fill="#2c2c2c" text-anchor="start">Gemini 3.8 Flash</text>
<rect x="215" y="268" width="144.8" height="14" rx="2" fill="#8c0000"><title>Gemini 3.8 Flash: 8.4 cents per round</title></rect>
<text x="367.8" y="279.0" font-size="12" fill="#2c2c2c" text-anchor="start">8.4¢</text>
<text x="0.0" y="303.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6 Luna</text>
<rect x="215" y="292" width="10.3" height="14" rx="2" fill="#8c0000"><title>GPT-6 Luna: 0.6 cents per round</title></rect>
<text x="233.3" y="303.0" font-size="12" fill="#2c2c2c" text-anchor="start">0.6¢</text>
<text x="0.0" y="327.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Haiku 5.5</text>
<rect x="215" y="316" width="6.9" height="14" rx="2" fill="#8c0000"><title>Claude Haiku 5.5: 0.4 cents per round</title></rect>
<text x="229.9" y="327.0" font-size="12" fill="#2c2c2c" text-anchor="start">0.4¢</text>
<text x="0.0" y="351.0" font-size="13" fill="#2c2c2c" text-anchor="start">MiniMax M3</text>
<rect x="215" y="340" width="68.9" height="14" rx="2" fill="#8c0000"><title>MiniMax M3: 4.0 cents per round</title></rect>
<text x="291.9" y="351.0" font-size="12" fill="#2c2c2c" text-anchor="start">4.0¢</text>
<text x="215.0" y="378.0" font-size="11" fill="#8a8878" text-anchor="start">US cents per round played</text>
</svg>

<svg class="results" style="width:100%;height:auto;margin:1em 0" viewBox="0 0 720 456" role="img" aria-labelledby="speed-title" font-family="inherit">
<title id="speed-title">Seconds of play per round for each model.</title>
<text x="0.0" y="14.0" font-size="12" fill="#8c0000" text-anchor="start" font-weight="700">Ascension 10, the hardest</text>
<text x="0.0" y="35.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Opus 5.5</text>
<rect x="215" y="24" width="26.5" height="14" rx="2" fill="#8c0000"><title>Claude Opus 5.5: 34 s per round</title></rect>
<text x="249.5" y="35.0" font-size="12" fill="#2c2c2c" text-anchor="start">34 s</text>
<text x="0.0" y="59.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Sonnet 5.5</text>
<rect x="215" y="48" width="13.3" height="14" rx="2" fill="#8c0000"><title>Claude Sonnet 5.5: 17 s per round</title></rect>
<text x="236.3" y="59.0" font-size="12" fill="#2c2c2c" text-anchor="start">17 s</text>
<text x="0.0" y="83.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6 Sol</text>
<rect x="215" y="72" width="15.6" height="14" rx="2" fill="#8c0000"><title>GPT-6 Sol: 20 s per round</title></rect>
<text x="238.6" y="83.0" font-size="12" fill="#2c2c2c" text-anchor="start">20 s</text>
<text x="0.0" y="107.0" font-size="13" fill="#2c2c2c" text-anchor="start">MiMo-V2.6-Pro</text>
<rect x="215" y="96" width="297.3" height="14" rx="2" fill="#8c0000"><title>MiMo-V2.6-Pro: 381 s per round</title></rect>
<text x="520.3" y="107.0" font-size="12" fill="#2c2c2c" text-anchor="start">381 s</text>
<text x="0.0" y="131.0" font-size="13" fill="#2c2c2c" text-anchor="start">DeepSeek V4.1 Flash</text>
<rect x="215" y="120" width="81.9" height="14" rx="2" fill="#8c0000"><title>DeepSeek V4.1 Flash: 105 s per round</title></rect>
<text x="304.9" y="131.0" font-size="12" fill="#2c2c2c" text-anchor="start">105 s</text>
<text x="0.0" y="155.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6.1 Sol</text>
<rect x="215" y="144" width="14.8" height="14" rx="2" fill="#8c0000"><title>GPT-6.1 Sol: 19 s per round</title></rect>
<text x="237.8" y="155.0" font-size="12" fill="#2c2c2c" text-anchor="start">19 s</text>
<text x="0.0" y="179.0" font-size="13" fill="#2c2c2c" text-anchor="start">DeepSeek V4 Pro</text>
<rect x="215" y="168" width="158.4" height="14" rx="2" fill="#8c0000"><title>DeepSeek V4 Pro: 203 s per round</title></rect>
<text x="381.4" y="179.0" font-size="12" fill="#2c2c2c" text-anchor="start">203 s</text>
<text x="0.0" y="203.0" font-size="13" fill="#2c2c2c" text-anchor="start">GLM 5.3 Flash</text>
<rect x="215" y="192" width="94.4" height="14" rx="2" fill="#8c0000"><title>GLM 5.3 Flash: 121 s per round</title></rect>
<text x="317.4" y="203.0" font-size="12" fill="#2c2c2c" text-anchor="start">121 s</text>
<text x="0.0" y="227.0" font-size="13" fill="#2c2c2c" text-anchor="start">Gemini 3.8 Flash</text>
<rect x="215" y="216" width="42.9" height="14" rx="2" fill="#8c0000"><title>Gemini 3.8 Flash: 55 s per round</title></rect>
<text x="265.9" y="227.0" font-size="12" fill="#2c2c2c" text-anchor="start">55 s</text>
<text x="0.0" y="258.0" font-size="12" fill="#8c0000" text-anchor="start" font-weight="700">Ascension 0, the easiest</text>
<text x="0.0" y="279.0" font-size="13" fill="#2c2c2c" text-anchor="start">Gemini 3.8 Flash</text>
<rect x="215" y="268" width="25.0" height="14" rx="2" fill="#8c0000"><title>Gemini 3.8 Flash: 32 s per round</title></rect>
<text x="248.0" y="279.0" font-size="12" fill="#2c2c2c" text-anchor="start">32 s</text>
<text x="0.0" y="303.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Sonnet 5.5</text>
<rect x="215" y="292" width="21.1" height="14" rx="2" fill="#8c0000"><title>Claude Sonnet 5.5: ~27 s per round</title></rect>
<text x="244.1" y="303.0" font-size="12" fill="#2c2c2c" text-anchor="start">~27 s</text>
<text x="0.0" y="327.0" font-size="13" fill="#2c2c2c" text-anchor="start">Step 5 Preview</text>
<rect x="215" y="316" width="355.0" height="14" rx="2" fill="#8c0000"><title>Step 5 Preview: 455 s per round</title></rect>
<text x="578.0" y="327.0" font-size="12" fill="#2c2c2c" text-anchor="start">455 s</text>
<text x="0.0" y="351.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6 Luna</text>
<rect x="215" y="340" width="32.0" height="14" rx="2" fill="#8c0000"><title>GPT-6 Luna: 41 s per round</title></rect>
<text x="255.0" y="351.0" font-size="12" fill="#2c2c2c" text-anchor="start">41 s</text>
<text x="0.0" y="375.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Haiku 5.5</text>
<rect x="215" y="364" width="15.6" height="14" rx="2" fill="#8c0000"><title>Claude Haiku 5.5: 20 s per round</title></rect>
<text x="238.6" y="375.0" font-size="12" fill="#2c2c2c" text-anchor="start">20 s</text>
<text x="0.0" y="399.0" font-size="13" fill="#2c2c2c" text-anchor="start">Nemotron 3 Ultra</text>
<rect x="215" y="388" width="217.7" height="14" rx="2" fill="#8c0000"><title>Nemotron 3 Ultra: 279 s per round</title></rect>
<text x="440.7" y="399.0" font-size="12" fill="#2c2c2c" text-anchor="start">279 s</text>
<text x="0.0" y="423.0" font-size="13" fill="#2c2c2c" text-anchor="start">MiniMax M3</text>
<rect x="215" y="412" width="96.0" height="14" rx="2" fill="#8c0000"><title>MiniMax M3: 123 s per round</title></rect>
<text x="319.0" y="423.0" font-size="12" fill="#2c2c2c" text-anchor="start">123 s</text>
<text x="215.0" y="450.0" font-size="11" fill="#8a8878" text-anchor="start">seconds per round played, shop included, waits excluded · ~ from wall time, not measured</text>
</svg>

The cost bars are not one price list. The Claude figures are what Claude Code reported at API list prices, the Luna and Haiku figures are what OpenCode logged for its Go plan, which is far cheaper per token, and the Sol and Gemini figures are what OpenCode logged at OpenRouter's prices. Read them within a harness, not across. Sonnet plays a round in half the time Opus takes, and Haiku in half the time Luna takes.

## What it says about the game

I built the benchmark in part to balance the game. A model that plays cold, on fixed seeds, is a playtester I can run on demand, and these results are a rough check on where the difficulty sits.

**Opus plays like a strong human, and wins about as often as one should.** Ascension 10 is meant for a skilled player to win about 40% of the time. Opus won three of five, which at five seeds is in that neighbourhood.

**It did not win the same way twice.** Its three wins were three different builds, and the run that fell one round short was a fourth:

| Seed | Result | Built around |
| ---- | ------ | ------------ |
| 1 | won | Pyromaniac and Scorched Earth: burn the alphabet, get paid per letter burned |
| 2 | won | Snowball, Indelible and The Vault: a multiplier that grows with every green tile |
| 5 | won | Keystone, Snowball and Thesaurus: multipliers stacked on the middle tile and on shape levels |
| 3 | 23 of 24 | Force of Habit: keep playing words of the same shape |

Across the five runs it held 23 different relics. Its words were as varied, 305 different ones in 358 guesses: no one build is carrying the game, and a player good enough to read the shop can win with what the shop offers. Sonnet is the contrast. It took Greedy Grammarian, which pays for gray tiles, in four of its five runs.

**The difficulty is spread across the run, not saved for the end.** Sonnet is a slightly weaker player than Opus, and where it died is the useful part. Its four losses at ascension 10 came at rounds 16, 18, 21 and 23, in stages 6, 7 and 8, on bosses and ordinary rounds alike. If every loss came in the final round, the first twenty-three would be a formality. They are not: each round past the midpoint is a real chance to lose.

**The last rule is the sharpest.** Ascension 9 is ascension 10 without its final rule: a round that reaches its target with the word unfound is survived, though it pays no gold. GPT-6.1 Sol did not win at ascension 10, so I had it play the same five seeds one level down.

| Seed | Ascension 10 | Ascension 9 |
| ---- | ------------ | ----------- |
| 1 | 11 rounds, 164 thousand | won, 4.5 million |
| 2 | 8 rounds, 55 thousand | won, 2.1 million |
| 3 | 21 rounds, 637 thousand | 20 rounds, 1.6 million |
| 4 | 2 rounds, 2 thousand | 2 rounds, 2 thousand |
| 5 | 18 rounds, 1.1 million | won, 4.7 million |

It won three of five, and its median score went from 164 thousand to 2.1 million. Across those three wins it failed to find the word in 3 of 72 rounds. At ascension 10 any one of those ends the run. Its builds could carry a run to the end; the occasional word it could not find is what stopped them. Five seeds a side cannot size the gap, though. Seed 5 won at ascension 9 with every word solved, and the same seed at ascension 10 died after 18 rounds.

Lifting the rule did nothing for a model that was not losing to it. DeepSeek V4.1 Flash lost four of its five runs at ascension 10 with the word solved and the score short. On the same seeds at ascension 9 it cleared 7, 17, 14, 5 and 4 rounds, against 11, 17, 16, 2 and 16. GLM 5.3 Flash came out about even: 14, 23, 4, 2 and 8 rounds, against 11, 10, 8, 2 and a win. On seed 2 it survived three rounds with the word unfound and reached the last stage. The seed it had won at ascension 10 ended after 8 rounds. The rule only costs a player whose build is already strong enough to win, and of these three that was one.

## Watch them play

Every run is recorded as its list of moves, so any of them can be replayed through the real game. These are at the game's ×2 speed, with its own sound.

* [Opus, ascension 10, seed 5](https://www.youtube.com/watch?v=iayM2a_q9eg): the highest-scoring win, 6.5 million points
* [Opus, ascension 10, seed 4](https://www.youtube.com/watch?v=a2fVOav6bKg): the 36-second loss
* [Sonnet, ascension 0, seed 5](https://www.youtube.com/watch?v=Zp8Y4W7u8hE): the TAXIS run
* [Gemini, ascension 0, seed 2](https://www.youtube.com/watch?v=R2jsTuhsbXU): the highest-scoring of its five wins, 5.1 million points
* [Sol, ascension 10, seed 5](https://www.youtube.com/watch?v=ey_7Feq_gns): its one win, 4.4 million points
* [Luna, ascension 0, seed 3](https://www.youtube.com/watch?v=5zj1K5oiwkE): JAZZY, QUEER and EERIE to the final round

Playlists of all five seeds: [Opus, ascension 10](https://www.youtube.com/playlist?list=PLNpeJ3c8LSSI), [Sonnet, ascension 10](https://www.youtube.com/playlist?list=PLPbdCMsQ0cyw), [Sonnet, ascension 0](https://www.youtube.com/playlist?list=PLZIqSWoqu3A8), [Sol, ascension 10](https://www.youtube.com/playlist?list=PLbGZ69vm1-0M), [Gemini, ascension 10](https://www.youtube.com/playlist?list=PLKtGosf5vcbQ), [GPT-6.1 Sol, ascension 10](https://www.youtube.com/playlist?list=PLZnD_aFf73II), [GLM 5.3 Flash, ascension 10](https://www.youtube.com/playlist?list=PLDzRfb6K_o6M), [DeepSeek V4.1 Flash, ascension 10](https://www.youtube.com/playlist?list=PLZmqrZmb2p4g), [MiMo-V2.6-Pro, ascension 10](https://www.youtube.com/playlist?list=PLeDG6SXiQnNo), [DeepSeek V4 Pro, ascension 10](https://www.youtube.com/playlist?list=PLKbSnPmJE0kU), [Gemini, ascension 0](https://www.youtube.com/playlist?list=PLQcOUbLko88Y), [Luna, ascension 0](https://www.youtube.com/playlist?list=PLARlXXTMTM3Y), [Haiku, ascension 0](https://www.youtube.com/playlist?list=PLKEUYwl-L6-o), [Step 5 Preview, ascension 0](https://www.youtube.com/playlist?list=PLM9YnyAIwvoc) and [Nemotron 3 Ultra, ascension 0](https://www.youtube.com/playlist?list=PLPU68aAT2rX0).

Every run by final score. Each dot is a run and links to its video.

<svg class="results" style="width:100%;height:auto;margin:1em 0" viewBox="0 0 720 538" role="img" aria-labelledby="score-title" font-family="inherit">
<title id="score-title">Final score of each of five runs for each model, on a log scale.</title>
<line x1="215.0" x2="215.0" y1="6" y2="504" stroke="#e3e1cf"/>
<text x="215.0" y="516.0" font-size="11" fill="#8a8878" text-anchor="middle">1K</text>
<line x1="336.3" x2="336.3" y1="6" y2="504" stroke="#e3e1cf"/>
<text x="336.3" y="516.0" font-size="11" fill="#8a8878" text-anchor="middle">10K</text>
<line x1="457.5" x2="457.5" y1="6" y2="504" stroke="#e3e1cf"/>
<text x="457.5" y="516.0" font-size="11" fill="#8a8878" text-anchor="middle">100K</text>
<line x1="578.8" x2="578.8" y1="6" y2="504" stroke="#e3e1cf"/>
<text x="578.8" y="516.0" font-size="11" fill="#8a8878" text-anchor="middle">1M</text>
<line x1="700.0" x2="700.0" y1="6" y2="504" stroke="#e3e1cf"/>
<text x="700.0" y="516.0" font-size="11" fill="#8a8878" text-anchor="middle">10M</text>
<text x="0.0" y="14.0" font-size="12" fill="#8c0000" text-anchor="start" font-weight="700">Ascension 10, the hardest</text>
<text x="0.0" y="40.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Opus 5.5</text>
<line x1="215" x2="700" y1="36" y2="36" stroke="#e3e1cf"/>
<a href="https://www.youtube.com/watch?v=adrTWq4DkTw"><circle cx="650.1" cy="36.0" r="5.5" fill="#8c0000" stroke="#fbfaef" stroke-width="2" paint-order="stroke"><title>Claude Opus 5.5, seed 1: 3,873,095 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=vwdLLUmtw48"><circle cx="644.6" cy="36.0" r="5.5" fill="#8c0000" stroke="#fbfaef" stroke-width="2" paint-order="stroke"><title>Claude Opus 5.5, seed 2: 3,489,091 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=qTJvwRW1SEg"><circle cx="599.9" cy="36.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Claude Opus 5.5, seed 3: 1,492,971 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=a2fVOav6bKg"><circle cx="259.9" cy="36.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Claude Opus 5.5, seed 4: 2,348 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=iayM2a_q9eg"><circle cx="677.7" cy="36.0" r="5.5" fill="#8c0000" stroke="#fbfaef" stroke-width="2" paint-order="stroke"><title>Claude Opus 5.5, seed 5: 6,551,698 points (watch)</title></circle></a>
<text x="0.0" y="68.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Sonnet 5.5</text>
<line x1="215" x2="700" y1="64" y2="64" stroke="#e3e1cf"/>
<a href="https://www.youtube.com/watch?v=yrusditZPE4"><circle cx="504.0" cy="64.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Claude Sonnet 5.5, seed 1: 241,878 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=GrOcnkQdwDU"><circle cx="533.7" cy="64.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Claude Sonnet 5.5, seed 2: 425,178 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=UH9l_1GaPm0"><circle cx="553.6" cy="64.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Claude Sonnet 5.5, seed 3: 620,789 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=p6WOduxnL5A"><circle cx="589.8" cy="64.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Claude Sonnet 5.5, seed 4: 1,232,555 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=U8igROuMQtQ"><circle cx="641.0" cy="64.0" r="5.5" fill="#8c0000" stroke="#fbfaef" stroke-width="2" paint-order="stroke"><title>Claude Sonnet 5.5, seed 5: 3,264,282 points (watch)</title></circle></a>
<text x="0.0" y="96.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6 Sol</text>
<line x1="215" x2="700" y1="92" y2="92" stroke="#e3e1cf"/>
<a href="https://www.youtube.com/watch?v=Jbg0F-wpENg"><circle cx="523.7" cy="92.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GPT-6 Sol, seed 1: 351,490 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=y8eHdJXrx2E"><circle cx="536.0" cy="92.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GPT-6 Sol, seed 2: 443,748 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=aK1txCGgeyo"><circle cx="409.9" cy="92.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GPT-6 Sol, seed 3: 40,532 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=kPTtJhUeqk8"><circle cx="257.1" cy="92.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GPT-6 Sol, seed 4: 2,224 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=ey_7Feq_gns"><circle cx="656.6" cy="92.0" r="5.5" fill="#8c0000" stroke="#fbfaef" stroke-width="2" paint-order="stroke"><title>GPT-6 Sol, seed 5: 4,388,081 points (watch)</title></circle></a>
<text x="0.0" y="124.0" font-size="13" fill="#2c2c2c" text-anchor="start">MiMo-V2.6-Pro</text>
<line x1="215" x2="700" y1="120" y2="120" stroke="#e3e1cf"/>
<a href="https://www.youtube.com/watch?v=PVj7my88Uqs"><circle cx="563.7" cy="120.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>MiMo-V2.6-Pro, seed 1: 751,730 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=1vtdj-A3FOU"><circle cx="382.5" cy="120.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>MiMo-V2.6-Pro, seed 2: 24,078 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=ZdRVXAKdwT0"><circle cx="455.8" cy="120.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>MiMo-V2.6-Pro, seed 3: 96,733 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=vD9t4aaMHwk"><circle cx="526.8" cy="120.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>MiMo-V2.6-Pro, seed 4: 372,866 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=pwt9r1NLz1k"><circle cx="510.9" cy="120.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>MiMo-V2.6-Pro, seed 5: 275,474 points (watch)</title></circle></a>
<text x="0.0" y="152.0" font-size="13" fill="#2c2c2c" text-anchor="start">DeepSeek V4.1 Flash</text>
<line x1="215" x2="700" y1="148" y2="148" stroke="#e3e1cf"/>
<a href="https://www.youtube.com/watch?v=yDI7jf9qUkE"><circle cx="430.0" cy="148.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>DeepSeek V4.1 Flash, seed 1: 59,266 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=K5uVfhW6PdA"><circle cx="523.5" cy="148.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>DeepSeek V4.1 Flash, seed 2: 350,065 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=0Lq3hE_nVW0"><circle cx="511.3" cy="148.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>DeepSeek V4.1 Flash, seed 3: 277,886 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=Z3mdoXoRwkA"><circle cx="258.4" cy="148.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>DeepSeek V4.1 Flash, seed 4: 2,279 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=9hojXwaQMQI"><circle cx="508.2" cy="148.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>DeepSeek V4.1 Flash, seed 5: 262,140 points (watch)</title></circle></a>
<text x="0.0" y="180.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6.1 Sol</text>
<line x1="215" x2="700" y1="176" y2="176" stroke="#e3e1cf"/>
<a href="https://www.youtube.com/watch?v=erikjgW8lt4"><circle cx="483.4" cy="176.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GPT-6.1 Sol, seed 1: 163,688 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=53FE_meAP1g"><circle cx="426.4" cy="176.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GPT-6.1 Sol, seed 2: 55,385 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=jD37Wuu29Lg"><circle cx="555.0" cy="176.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GPT-6.1 Sol, seed 3: 637,062 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=nDLUAjcKKEs"><circle cx="244.4" cy="176.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GPT-6.1 Sol, seed 4: 1,749 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=AR95WKHPWTo"><circle cx="584.6" cy="176.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GPT-6.1 Sol, seed 5: 1,118,355 points (watch)</title></circle></a>
<text x="0.0" y="208.0" font-size="13" fill="#2c2c2c" text-anchor="start">DeepSeek V4 Pro</text>
<line x1="215" x2="700" y1="204" y2="204" stroke="#e3e1cf"/>
<a href="https://www.youtube.com/watch?v=g97u_BuKtfI"><circle cx="459.8" cy="204.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>DeepSeek V4 Pro, seed 1: 104,477 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=Oql4Mgik6Ps"><circle cx="450.1" cy="204.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>DeepSeek V4 Pro, seed 2: 86,925 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=W_zHf3ygGWs"><circle cx="311.9" cy="204.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>DeepSeek V4 Pro, seed 3: 6,298 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=rEKWM1S5Qmo"><circle cx="492.3" cy="204.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>DeepSeek V4 Pro, seed 4: 193,611 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=RlobHTVNY_U"><circle cx="525.6" cy="204.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>DeepSeek V4 Pro, seed 5: 364,607 points (watch)</title></circle></a>
<text x="0.0" y="236.0" font-size="13" fill="#2c2c2c" text-anchor="start">GLM 5.3 Flash</text>
<line x1="215" x2="700" y1="232" y2="232" stroke="#e3e1cf"/>
<a href="https://www.youtube.com/watch?v=U4_qGKCounw"><circle cx="468.9" cy="232.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GLM 5.3 Flash, seed 1: 124,271 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=NuywloorlgE"><circle cx="452.8" cy="232.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GLM 5.3 Flash, seed 2: 91,395 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=YRuqONkt800"><circle cx="454.6" cy="232.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GLM 5.3 Flash, seed 3: 94,582 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=8ySYrb62vkE"><circle cx="241.7" cy="232.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GLM 5.3 Flash, seed 4: 1,661 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=hBWIqTIDLE0"><circle cx="607.8" cy="232.0" r="5.5" fill="#8c0000" stroke="#fbfaef" stroke-width="2" paint-order="stroke"><title>GLM 5.3 Flash, seed 5: 1,734,821 points (watch)</title></circle></a>
<text x="0.0" y="264.0" font-size="13" fill="#2c2c2c" text-anchor="start">Gemini 3.8 Flash</text>
<line x1="215" x2="700" y1="260" y2="260" stroke="#e3e1cf"/>
<a href="https://www.youtube.com/watch?v=0R70-cRIg6g"><circle cx="515.7" cy="260.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Gemini 3.8 Flash, seed 1: 302,122 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=VWbtI_bBIJI"><circle cx="377.0" cy="260.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Gemini 3.8 Flash, seed 2: 21,680 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=pyWffPw-YWw"><circle cx="423.2" cy="260.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Gemini 3.8 Flash, seed 3: 52,176 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=jx_FDosJzD4"><circle cx="568.9" cy="260.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Gemini 3.8 Flash, seed 4: 828,777 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=Xs2nELVIfV4"><circle cx="388.7" cy="260.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Gemini 3.8 Flash, seed 5: 27,068 points (watch)</title></circle></a>
<text x="0.0" y="294.0" font-size="12" fill="#8c0000" text-anchor="start" font-weight="700">Ascension 0, the easiest</text>
<text x="0.0" y="320.0" font-size="13" fill="#2c2c2c" text-anchor="start">Gemini 3.8 Flash</text>
<line x1="215" x2="700" y1="316" y2="316" stroke="#e3e1cf"/>
<a href="https://www.youtube.com/watch?v=49SKHzw9LDM"><circle cx="654.3" cy="316.0" r="5.5" fill="#8c0000" stroke="#fbfaef" stroke-width="2" paint-order="stroke"><title>Gemini 3.8 Flash, seed 1: 4,198,512 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=R2jsTuhsbXU"><circle cx="664.2" cy="316.0" r="5.5" fill="#8c0000" stroke="#fbfaef" stroke-width="2" paint-order="stroke"><title>Gemini 3.8 Flash, seed 2: 5,069,690 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=DuzftCWhnjY"><circle cx="627.3" cy="316.0" r="5.5" fill="#8c0000" stroke="#fbfaef" stroke-width="2" paint-order="stroke"><title>Gemini 3.8 Flash, seed 3: 2,514,385 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=mOlavEqeURk"><circle cx="639.9" cy="316.0" r="5.5" fill="#8c0000" stroke="#fbfaef" stroke-width="2" paint-order="stroke"><title>Gemini 3.8 Flash, seed 4: 3,193,387 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=nJocTunBu2o"><circle cx="649.1" cy="316.0" r="5.5" fill="#8c0000" stroke="#fbfaef" stroke-width="2" paint-order="stroke"><title>Gemini 3.8 Flash, seed 5: 3,805,591 points (watch)</title></circle></a>
<text x="0.0" y="348.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Sonnet 5.5</text>
<line x1="215" x2="700" y1="344" y2="344" stroke="#e3e1cf"/>
<a href="https://www.youtube.com/watch?v=T5hgQKiSg1w"><circle cx="578.3" cy="344.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Claude Sonnet 5.5, seed 1: 991,301 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=tqtyQ6L5mvw"><circle cx="623.8" cy="344.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Claude Sonnet 5.5, seed 2: 2,352,722 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=y2chT1UjSYs"><circle cx="559.7" cy="344.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Claude Sonnet 5.5, seed 3: 697,070 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=RlzB8TtLrtc"><circle cx="499.2" cy="344.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Claude Sonnet 5.5, seed 4: 220,820 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=Zp8Y4W7u8hE"><circle cx="609.0" cy="344.0" r="5.5" fill="#8c0000" stroke="#fbfaef" stroke-width="2" paint-order="stroke"><title>Claude Sonnet 5.5, seed 5: 1,777,015 points (watch)</title></circle></a>
<text x="0.0" y="376.0" font-size="13" fill="#2c2c2c" text-anchor="start">Step 5 Preview</text>
<line x1="215" x2="700" y1="372" y2="372" stroke="#e3e1cf"/>
<a href="https://www.youtube.com/watch?v=E4CgtemAkD8"><circle cx="504.0" cy="372.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Step 5 Preview, seed 1: 241,668 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=kAo7-11T-YA"><circle cx="511.8" cy="372.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Step 5 Preview, seed 2: 280,571 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=MGGPMgselgs"><circle cx="473.0" cy="372.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Step 5 Preview, seed 3: 134,164 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=AUelOBa-WTI"><circle cx="215.0" cy="372.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Step 5 Preview, seed 4: 990 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=vy6FkmqC4rE"><circle cx="503.0" cy="372.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Step 5 Preview, seed 5: 237,290 points (watch)</title></circle></a>
<text x="0.0" y="404.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6 Luna</text>
<line x1="215" x2="700" y1="400" y2="400" stroke="#e3e1cf"/>
<a href="https://www.youtube.com/watch?v=zzGGdhAlKUg"><circle cx="486.8" cy="400.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GPT-6 Luna, seed 1: 174,437 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=2Y5m6Y5lT_k"><circle cx="490.2" cy="400.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GPT-6 Luna, seed 2: 186,222 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=5zj1K5oiwkE"><circle cx="605.2" cy="400.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GPT-6 Luna, seed 3: 1,652,069 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=C3l3Dze_Ols"><circle cx="254.5" cy="400.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GPT-6 Luna, seed 4: 2,116 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=tIKdDAuWXis"><circle cx="551.7" cy="400.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GPT-6 Luna, seed 5: 598,395 points (watch)</title></circle></a>
<text x="0.0" y="432.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Haiku 5.5</text>
<line x1="215" x2="700" y1="428" y2="428" stroke="#e3e1cf"/>
<a href="https://www.youtube.com/watch?v=-nUz-qL6yNY"><circle cx="501.7" cy="428.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Claude Haiku 5.5, seed 1: 231,696 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=G2O1qn5KZr8"><circle cx="456.1" cy="428.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Claude Haiku 5.5, seed 2: 97,385 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=Ytxe7EFLmwQ"><circle cx="492.1" cy="428.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Claude Haiku 5.5, seed 3: 193,064 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=bH6YjQL5pG4"><circle cx="221.4" cy="428.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Claude Haiku 5.5, seed 4: 1,130 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=64A30oLI-ZM"><circle cx="238.7" cy="428.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Claude Haiku 5.5, seed 5: 1,568 points (watch)</title></circle></a>
<text x="0.0" y="460.0" font-size="13" fill="#2c2c2c" text-anchor="start">Nemotron 3 Ultra</text>
<line x1="215" x2="700" y1="456" y2="456" stroke="#e3e1cf"/>
<a href="https://www.youtube.com/watch?v=m7tC_lDiXxQ"><circle cx="226.0" cy="456.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Nemotron 3 Ultra, seed 1: 1,232 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=WofXv42yKPU"><circle cx="598.7" cy="456.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Nemotron 3 Ultra, seed 2: 1,460,912 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=fBCMfZwrqaI"><circle cx="464.3" cy="456.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Nemotron 3 Ultra, seed 3: 113,743 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=3Pmo7xsy2s0"><circle cx="243.9" cy="456.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Nemotron 3 Ultra, seed 4: 1,732 points (watch)</title></circle></a>
<a href="https://www.youtube.com/watch?v=etKycxsho6E"><circle cx="298.2" cy="456.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Nemotron 3 Ultra, seed 5: 4,851 points (watch)</title></circle></a>
<text x="0.0" y="488.0" font-size="13" fill="#2c2c2c" text-anchor="start">MiniMax M3</text>
<line x1="215" x2="700" y1="484" y2="484" stroke="#e3e1cf"/>
<circle cx="215.0" cy="484.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>MiniMax M3, seed 1: 0 points</title></circle>
<circle cx="449.9" cy="484.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>MiniMax M3, seed 2: 86,634 points</title></circle>
<circle cx="215.0" cy="484.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>MiniMax M3, seed 3: 0 points</title></circle>
<circle cx="253.5" cy="484.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>MiniMax M3, seed 4: 2,079 points</title></circle>
<circle cx="504.3" cy="484.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>MiniMax M3, seed 5: 243,319 points</title></circle>
<text x="215.0" y="532.0" font-size="11" fill="#8a8878" text-anchor="start">final score of each run, log scale · ● won   ○ lost</text>
</svg>
