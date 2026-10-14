---
og-title: "5 Wild Bench, a word-game roguelike LLM benchmark"
title-prefix: "Blog"
date: 2026-10-10
nav_blog: true
---

# 5 Wild Bench, a word-game roguelike LLM benchmark

[5 Wild](https://5-wild.com) is a free-to-play word-guessing roguelike I made combining [Wordle](https://www.nytimes.com/games/wordle) with [Balatro](https://www.playbalatro.com/)'s scoring and shop mechanics.

I had fourteen LLMs play five runs. One run involves solving 24 Wordle boards. The nine largest models played the hardest difficulty which a skilled Wordle × Balatro player should win about 30% of the time. Seven played the easiest, which the average Balatro player should win most of the time.

Two of my favorite LLM benchmarks are the [RuneScape Bench](https://maxbittker.github.io/runebench/) and Anthropic's [Project Vend](https://www.anthropic.com/research/project-vend-1). I like the idea of a gamified benchmark versus some abstract, academic test that I haven't conceptualized. I don't care much if a model scores a perfect SAT if they can't execute on a long-horizon task, adapting to conditions as they change, and evaluating immediate options against long-term objectives.

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

Each dot represents a run and clicking one opens its replay in YouTube.

<figure style="margin:1em 0">
<iframe src="https://www.youtube-nocookie.com/embed/iayM2a_q9eg?start=100" title="Claude Opus 5.5 winning a run of 5 Wild at ascension 10" loading="lazy" allow="accelerometer; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" allowfullscreen style="width:100%;aspect-ratio:16/9;border:0"></iframe>
</figure>

## What stood out

* **There seems to be some signal.** Opus 5.5 is clearly a special model with the strongest performance consistently. Gemini 3.8 Flash won all five of its runs on the easiest difficulty while struggling with the stricter rules imposed at higher difficulties while Sonnet 5.5 was able to adapt. Sol seemed to underperform compared to expectations but it struggled uniquely with solving rounds and performed closer to expectations on easier difficulties where failing to solve rounds is forgiven more.
* **Each model had its own style.** Gemini opened with CRANE in 100 of its 120 rounds on ascension 0 helping it to win 5/5 runs. At ascension 10, CRANE can only be played once a run and Gemini's ability fell apart. Opus won three times with three different builds and was able to convert successful guesses into income generating builds. Luna leaned into the game's scoring mechanic opening with high-scoring words like JAZZY, FUZZY, and EERIE and had the most diverse use of word shapes. Sonnet won one run by [playing TAXIS 26 guesses in a row](#the-strategies-they-found). Sol had the highest rate of DISTINCT words allowing it to solve rounds at a high rate without necessarily translating that into points.
* **Forcing a win condition was not effective.** The most effective style was that which combined many and avoided over-indexing on any particular scoring lever. Opus 5.5's wins aren't noticeably distinct on paper and nothing, save guessing ability and economy, really stands out as a clear indicator for how it won. While other models clearly preferred one of a handful of dimensions such as letter rarity, word shape, or buying and selling from the shop, the winning solution seems to take what is given, find the best micro-optimizations, and diversify. In other words, investing in insurance, economy, and scaling over immediate returns.
* **Neither solving nor economy alone won.** Opus 5.5 had the lowest average number of guesses per round allowing it to increase its income and supercharge its scoring engine. The biggest contrast with Sonnet was its inability to convert points into income causing its scoring engine to die out in the later rounds. Every model with a low economy died out in the mid to late game. Conversely, GLM 5.3 Flash and DeepSeek v4 Pro generated plenty of economy but failed to convert it into scoring points.

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

Average guesses per cleared round, where lower is better and six is the limit. "Solved" counts the rounds cleared by finding the word, as opposed to reaching the target without it; at ascension 10, this is expected to match as solving each round is required.

## The game

5 Wild takes its board from Wordle. You get six guesses to find a five-letter word, with green, yellow and gray tiles hinting at the solution. Everything around the board comes from Balatro. Each guess scores points and each round has a target score to beat. Less common letters are worth more (like Scrabble), a green tile multiplies them, and a gray tile does nothing. Similar to Balatro's poker hands, each word has a "shape" which describes its composition. For example, if each letter is unique, that word's shape is DISTINCT but if a letter repeats, such as the P's in HAPPY, its shape is TWINNED. Rarer shapes yield more points. Between rounds, a shop sells relics (Balatro's jokers) which change how a guess scores. A run is eight stages of three rounds each, and the third round of every stage is a boss that changes one of the rules.

The game is a constant push-and-pull between using guesses to score points and solving the round. Solving the round greatly increases the final score, and solving early yields even more. Many of the relics scale around guesses, so spending guesses trying to scale your build may result in missing out on points.

Difficulty levels are called ascensions and they stack like Slay the Spire's, so ascension 10 plays with every rule below it. These are the rules that matter most here.

* Every guess must reuse the letters you've found (Wordle's hard mode)
* No word can be played twice in the same run
* Income is reduced, relics slots are reduced by 1, and target points scale faster
* Every round must be solved correctly while reaching the point target goal

## The setup

The game's engine is deterministic given a seed and a list of actions. I originally built a spectator mode for the game so I could livestream agents playing via an [MCP](https://modelcontextprotocol.io/) server for QA. The MCP has four tools.

* `rules`: a how to play guide for agent
* `new_run`: start a run for a given seed
* `observe`: read the board state
* `act`: one command (`guess crane`, `buy 2`, `collect`, `next`)

There is no shell, file access, web search or subagents, so agents can't write a solver or look up the word list. Each run is indepenedent of one another.

Every model plays the same five seeds. The answers and shop options start the same but eventually drift out of necessity; that way purchased relics don't re-appear in the shop, answers stay unique from previous guesses, and broken letters don't soft-lock answers.

Nothing about the setup is specific to the harness, and anything that can connect to the MCP server can play. Each run is saved as its list of moves which are used to replay the run and exported as a video.

## The strategies they found

**Sonnet broke half the alphabet and played TAXIS 26 times in a row.** On seed 5 of its ascension 0 playthrough, it bought Pyromaniac, which breaks a random letter every round making it unplayable, and Scorched Earth, which scales with the number of broken letters. By round 20, twelve letters were gone, and it had upgraded letters T, A, I and S. TAXIS is those four upgraded letters around an X, a rare, high-valued letter. From there it stopped solving:

```
round 21   taxis taxis taxis taxis taxis taxis
round 22   taxis taxis taxis taxis taxis taxis
round 23   taxis taxis taxis taxis taxis taxis
round 24   taxis taxis taxis          (won)
```

It chose to lose rounds in order to win the game — exactly the kind of thing this benchmark intends to capture. Later ascensions prevent this strategy by requiring uniqueness between guesses.

**Luna opened with JAZZY.** Instead of playing common, information-rich Wordle openers like CRANE or STARE, Luna's most-played words were EERIE (15 times), CRANE (13), JAZZY (11) and FUZZY (6), with QUEER and EJECT also seeing high usage. Every one of those but CRANE [repeats a letter](#word-shapes), and in three of its five runs purchased the Twins relic, which multiplies guesses containing a given letter twice. JAZZY is also one of the most rewarding words in the game, scoring 33 base points against 5 for AROSE. It leaned fully into the game's mechanic of trading information to solve rounds in exchange for points.

**Gemini opened with CRANE 100 times.** At ascension 0, nothing stops a model from playing the same word every round, and Gemini 3.8 Flash opened with CRANE in 100 of its 120 rounds. CRANE is a typical Wordle opener that scores very few points, but it gave Gemini the same head start on every answer, and it went on to solve 114 of the 120 rounds and win all five runs. At ascension 10, where a word can only be played once per run, it played CRANE and STARE in the first two rounds of every run and then had to find a new opener for each round after that. It lost all five of those runs, three of them before round 10.

At ascension 0, the most-played words show how far a favorite can carry a run.

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

**Rhyme traps ended runs that had plenty of points.** Since every guess at ascension 10 must reuse the green letters found, a model that finds four of them early can be left guessing the fifth one word at a time. On seed 1, the answer in round 12 was WOUND. GPT-6.1 Sol played POUND, FOUND, BOUND and HOUND, DeepSeek V4 Pro played POUND, BOUND, HOUND and MOUND, and both lost with more than double the points they needed. Of the 39 losses at ascension 10, 13 ended with enough points but the word unsolved.

Opus other the other hand never saw WOUND. This is a very intended consequence of the game. It played the same Pyromaniac and Scorched Earth build as Sonnet's TAXIS run, and W and N were among the broken letters. On ascension 10, improving your likelihood .

This does probably indicate a more generic representative of model performance with this benchmark would use ascension 9 to avoid RNG from these traps (but that would also be no fun).

## Cost and speed

A full Opus run cost about $4 and took 11 minutes at an average of $0.21 and 34s per round. For a relatively complex task, I think it is fairly cost and time efficient. A large majority of tokens were cached between turns. Each run is able to run in parallel with one another.

<svg class="results" style="width:100%;height:auto;margin:1em 0" viewBox="0 0 720 466" role="img" aria-labelledby="value-title" font-family="inherit">
<title id="value-title">Median final score against cost per round played, for each model at ascension 10. Score is on a logarithmic scale.</title>
<text x="0.0" y="14.0" font-size="12" fill="#8c0000" text-anchor="start" font-weight="700">Ascension 10, the hardest</text>
<circle cx="204" cy="10" r="5.5" fill="#8a8878" stroke="#fbfaef" stroke-width="2" paint-order="stroke"/>
<text x="214.0" y="14.0" font-size="11" fill="#8a8878" text-anchor="start">won a run</text>
<circle cx="288.4" cy="10" r="5.5" fill="#fbfaef" stroke="#8a8878" stroke-width="2.5"/>
<text x="298.4" y="14.0" font-size="11" fill="#8a8878" text-anchor="start">won none</text>
<text x="0.0" y="34.0" font-size="11" fill="#8a8878" text-anchor="start">median final score of 5 runs, on a log scale</text>
<line x1="40" x2="720" y1="350.6" y2="350.6" stroke="#e3e1cf"/>
<text x="34.0" y="354.6" font-size="11" fill="#8a8878" text-anchor="end">100K</text>
<line x1="40" x2="720" y1="165.5" y2="165.5" stroke="#e3e1cf"/>
<text x="34.0" y="169.5" font-size="11" fill="#8a8878" text-anchor="end">1.0M</text>
<line x1="52.0" x2="52.0" y1="44" y2="424" stroke="#e3e1cf"/>
<text x="52.0" y="440.0" font-size="11" fill="#8a8878" text-anchor="middle">0¢</text>
<line x1="211.2" x2="211.2" y1="44" y2="424" stroke="#e3e1cf"/>
<text x="211.2" y="440.0" font-size="11" fill="#8a8878" text-anchor="middle">5¢</text>
<line x1="370.4" x2="370.4" y1="44" y2="424" stroke="#e3e1cf"/>
<text x="370.4" y="440.0" font-size="11" fill="#8a8878" text-anchor="middle">10¢</text>
<line x1="529.7" x2="529.7" y1="44" y2="424" stroke="#e3e1cf"/>
<text x="529.7" y="440.0" font-size="11" fill="#8a8878" text-anchor="middle">15¢</text>
<line x1="688.9" x2="688.9" y1="44" y2="424" stroke="#e3e1cf"/>
<text x="688.9" y="440.0" font-size="11" fill="#8a8878" text-anchor="middle">20¢</text>
<line x1="40" x2="40" y1="44" y2="424" stroke="#8a8878"/>
<line x1="40" x2="720" y1="424" y2="424" stroke="#8a8878"/>
<text x="380.0" y="458.0" font-size="11" fill="#8a8878" text-anchor="middle">US cents per round played</text>
<circle cx="708.0" cy="65.1" r="5.5" fill="#8c0000" stroke="#fbfaef" stroke-width="2" paint-order="stroke"><title>Claude Opus 5.5, ascension 10: median 3,489,091 points, at 20.6¢ and 34 s a round, won 3 of 5</title></circle>
<circle cx="481.9" cy="203.9" r="5.5" fill="#8c0000" stroke="#fbfaef" stroke-width="2" paint-order="stroke"><title>Claude Sonnet 5.5, ascension 10: median 620,789 points, at 13.5¢ and 17 s a round, won 1 of 5</title></circle>
<circle cx="341.8" cy="249.6" r="5.5" fill="#8c0000" stroke="#fbfaef" stroke-width="2" paint-order="stroke"><title>GPT-6 Sol, ascension 10: median 351,490 points, at 9.1¢ and 20 s a round, won 1 of 5</title></circle>
<circle cx="246.3" cy="269.2" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>MiMo-V2.6-Pro, ascension 10: median 275,474 points, at 6.1¢ and 381 s a round, won 0 of 5</title></circle>
<circle cx="150.7" cy="273.2" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>DeepSeek V4.1 Flash, ascension 10: median 262,140 points, at 3.1¢ and 105 s a round, won 0 of 5</title></circle>
<circle cx="188.9" cy="311.0" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>GPT-6.1 Sol, ascension 10: median 163,688 points, at 4.3¢ and 19 s a round, won 0 of 5</title></circle>
<circle cx="239.9" cy="347.1" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>DeepSeek V4 Pro, ascension 10: median 104,477 points, at 5.9¢ and 203 s a round, won 0 of 5</title></circle>
<circle cx="198.5" cy="355.1" r="5.5" fill="#8c0000" stroke="#fbfaef" stroke-width="2" paint-order="stroke"><title>GLM 5.3 Flash, ascension 10: median 94,582 points, at 4.6¢ and 121 s a round, won 1 of 5</title></circle>
<circle cx="284.5" cy="402.9" r="5.5" fill="#fbfaef" stroke="#8c0000" stroke-width="2.5"><title>Gemini 3.8 Flash, ascension 10: median 52,176 points, at 7.3¢ and 55 s a round, won 0 of 5</title></circle>
<g stroke="#fbfaef" stroke-width="3" paint-order="stroke">
<text x="698.0" y="69.1" font-size="12" fill="#2c2c2c" text-anchor="end">Claude Opus 5.5</text>
<text x="491.9" y="207.9" font-size="12" fill="#2c2c2c" text-anchor="start">Claude Sonnet 5.5</text>
<text x="351.8" y="253.6" font-size="12" fill="#2c2c2c" text-anchor="start">GPT-6 Sol</text>
<text x="256.3" y="273.2" font-size="12" fill="#2c2c2c" text-anchor="start">MiMo-V2.6-Pro</text>
<text x="150.7" y="262.2" font-size="12" fill="#2c2c2c" text-anchor="middle">DeepSeek V4.1 Flash</text>
<text x="198.9" y="315.0" font-size="12" fill="#2c2c2c" text-anchor="start">GPT-6.1 Sol</text>
<text x="249.9" y="351.1" font-size="12" fill="#2c2c2c" text-anchor="start">DeepSeek V4 Pro</text>
<text x="188.5" y="359.1" font-size="12" fill="#2c2c2c" text-anchor="end">GLM 5.3 Flash</text>
<text x="294.5" y="406.9" font-size="12" fill="#2c2c2c" text-anchor="start">Gemini 3.8 Flash</text>
</g>
</svg>

*Note, the Claude figures are Claude Code's API prices while the rest are OpenRouter's prices.*

<svg class="results" style="width:100%;height:auto;margin:1em 0" viewBox="0 0 720 456" role="img" aria-labelledby="speed-title" font-family="inherit">
<title id="speed-title">Seconds of play per round for each model, coloured by the provider that served it.</title>
<text x="656.2" y="14.0" font-size="11" fill="#8a8878" text-anchor="start">OpenCode Go</text>
<rect x="640.2" y="4" width="11" height="11" rx="2" fill="#b59f3b"/>
<text x="566.2" y="14.0" font-size="11" fill="#8a8878" text-anchor="start">OpenRouter</text>
<rect x="550.2" y="4" width="11" height="11" rx="2" fill="#2f6f8f"/>
<text x="470.4" y="14.0" font-size="11" fill="#8a8878" text-anchor="start">Claude Code</text>
<rect x="454.4" y="4" width="11" height="11" rx="2" fill="#8c0000"/>
<text x="0.0" y="14.0" font-size="12" fill="#8c0000" text-anchor="start" font-weight="700">Ascension 10, the hardest</text>
<text x="0.0" y="35.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Opus 5.5</text>
<rect x="215" y="24" width="26.5" height="14" rx="2" fill="#8c0000"><title>Claude Opus 5.5, through Claude Code: 34 s per round</title></rect>
<text x="249.5" y="35.0" font-size="12" fill="#2c2c2c" text-anchor="start">34 s</text>
<text x="0.0" y="59.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Sonnet 5.5</text>
<rect x="215" y="48" width="13.3" height="14" rx="2" fill="#8c0000"><title>Claude Sonnet 5.5, through Claude Code: 17 s per round</title></rect>
<text x="236.3" y="59.0" font-size="12" fill="#2c2c2c" text-anchor="start">17 s</text>
<text x="0.0" y="83.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6 Sol</text>
<rect x="215" y="72" width="15.6" height="14" rx="2" fill="#2f6f8f"><title>GPT-6 Sol, through OpenRouter: 20 s per round</title></rect>
<text x="238.6" y="83.0" font-size="12" fill="#2c2c2c" text-anchor="start">20 s</text>
<text x="0.0" y="107.0" font-size="13" fill="#2c2c2c" text-anchor="start">MiMo-V2.6-Pro</text>
<rect x="215" y="96" width="297.3" height="14" rx="2" fill="#2f6f8f"><title>MiMo-V2.6-Pro, through OpenRouter: 381 s per round</title></rect>
<text x="520.3" y="107.0" font-size="12" fill="#2c2c2c" text-anchor="start">381 s</text>
<text x="0.0" y="131.0" font-size="13" fill="#2c2c2c" text-anchor="start">DeepSeek V4.1 Flash</text>
<rect x="215" y="120" width="81.9" height="14" rx="2" fill="#2f6f8f"><title>DeepSeek V4.1 Flash, through OpenRouter: 105 s per round</title></rect>
<text x="304.9" y="131.0" font-size="12" fill="#2c2c2c" text-anchor="start">105 s</text>
<text x="0.0" y="155.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6.1 Sol</text>
<rect x="215" y="144" width="14.8" height="14" rx="2" fill="#2f6f8f"><title>GPT-6.1 Sol, through OpenRouter: 19 s per round</title></rect>
<text x="237.8" y="155.0" font-size="12" fill="#2c2c2c" text-anchor="start">19 s</text>
<text x="0.0" y="179.0" font-size="13" fill="#2c2c2c" text-anchor="start">DeepSeek V4 Pro</text>
<rect x="215" y="168" width="158.4" height="14" rx="2" fill="#b59f3b"><title>DeepSeek V4 Pro, through OpenCode Go: 203 s per round</title></rect>
<text x="381.4" y="179.0" font-size="12" fill="#2c2c2c" text-anchor="start">203 s</text>
<text x="0.0" y="203.0" font-size="13" fill="#2c2c2c" text-anchor="start">GLM 5.3 Flash</text>
<rect x="215" y="192" width="94.4" height="14" rx="2" fill="#2f6f8f"><title>GLM 5.3 Flash, through OpenRouter: 121 s per round</title></rect>
<text x="317.4" y="203.0" font-size="12" fill="#2c2c2c" text-anchor="start">121 s</text>
<text x="0.0" y="227.0" font-size="13" fill="#2c2c2c" text-anchor="start">Gemini 3.8 Flash</text>
<rect x="215" y="216" width="42.9" height="14" rx="2" fill="#2f6f8f"><title>Gemini 3.8 Flash, through OpenRouter: 55 s per round</title></rect>
<text x="265.9" y="227.0" font-size="12" fill="#2c2c2c" text-anchor="start">55 s</text>
<text x="0.0" y="258.0" font-size="12" fill="#8c0000" text-anchor="start" font-weight="700">Ascension 0, the easiest</text>
<text x="0.0" y="279.0" font-size="13" fill="#2c2c2c" text-anchor="start">Gemini 3.8 Flash</text>
<rect x="215" y="268" width="25.0" height="14" rx="2" fill="#2f6f8f"><title>Gemini 3.8 Flash, through OpenRouter: 32 s per round</title></rect>
<text x="248.0" y="279.0" font-size="12" fill="#2c2c2c" text-anchor="start">32 s</text>
<text x="0.0" y="303.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Sonnet 5.5</text>
<rect x="215" y="292" width="21.1" height="14" rx="2" fill="#8c0000"><title>Claude Sonnet 5.5, through Claude Code: 27 s per round</title></rect>
<text x="244.1" y="303.0" font-size="12" fill="#2c2c2c" text-anchor="start">27 s</text>
<text x="0.0" y="327.0" font-size="13" fill="#2c2c2c" text-anchor="start">Step 5 Preview</text>
<rect x="215" y="316" width="355.0" height="14" rx="2" fill="#b59f3b"><title>Step 5 Preview, through OpenCode Go: 455 s per round</title></rect>
<text x="578.0" y="327.0" font-size="12" fill="#2c2c2c" text-anchor="start">455 s</text>
<text x="0.0" y="351.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6 Luna</text>
<rect x="215" y="340" width="32.0" height="14" rx="2" fill="#b59f3b"><title>GPT-6 Luna, through OpenCode Go: 41 s per round</title></rect>
<text x="255.0" y="351.0" font-size="12" fill="#2c2c2c" text-anchor="start">41 s</text>
<text x="0.0" y="375.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Haiku 5.5</text>
<rect x="215" y="364" width="15.6" height="14" rx="2" fill="#b59f3b"><title>Claude Haiku 5.5, through OpenCode Go: 20 s per round</title></rect>
<text x="238.6" y="375.0" font-size="12" fill="#2c2c2c" text-anchor="start">20 s</text>
<text x="0.0" y="399.0" font-size="13" fill="#2c2c2c" text-anchor="start">Nemotron 3 Ultra</text>
<rect x="215" y="388" width="217.7" height="14" rx="2" fill="#2f6f8f"><title>Nemotron 3 Ultra, through OpenRouter: 279 s per round</title></rect>
<text x="440.7" y="399.0" font-size="12" fill="#2c2c2c" text-anchor="start">279 s</text>
<text x="0.0" y="423.0" font-size="13" fill="#2c2c2c" text-anchor="start">MiniMax M3</text>
<rect x="215" y="412" width="96.0" height="14" rx="2" fill="#b59f3b"><title>MiniMax M3, through OpenCode Go: 123 s per round</title></rect>
<text x="319.0" y="423.0" font-size="12" fill="#2c2c2c" text-anchor="start">123 s</text>
<text x="215.0" y="450.0" font-size="11" fill="#8a8878" text-anchor="start">seconds per round played, shop included, waits excluded</text>
</svg>

## What it says about the game

I built this benchmark in part to balance the game. Balancing has been the hardest (and time-consuming) part of development by far. Solver bots with golden runs only assess difficulty at a cursory level. I'm also not too sure what range of ability to expect for players. I vaguely recall Balatro being incredibly difficult at first and the skill (and difficulty) distribution of solving the word puzzles is probably fairly variable.

It is fortunate the models were characteristically unique as that provides more variation in play styles to review.

**Opus plays like a strong human, and wins about as often as one should.** Ascension 10 is meant for a skilled player to win about 30% of the time and Opus was in the ballpark winning 3 of 5 runs. Though Opus's 3.59 guesses per solve isn't abnormally high by Wordle standards, ascension 10's requirement that each word be unique throughout the entire run adds about +1 expected average guess compared to the player's Wordle average, making Opus a considerably strong guesser.

**It did not win the same way twice.** Its four runs that reached the last round were with different builds:

| Seed | Result | Built around |
| ---- | ------ | ------------ |
| 1 | won | Pyromaniac and Scorched Earth: destroy letters and constrains guesses |
| 2 | won | Snowball, Indelible and The Vault: scales with correct guesses |
| 5 | won | Keystone, Snowball and Thesaurus: scales with word shape |
| 3 | 23 of 24 | Force of Habit: scales with word shape |

Across the five runs it held 23 different relics. Its words were as varied, 305 different ones in 358 guesses. No one build carries the game, and most seeds can win with optimal strategy from what is offered and solid guesses.

**A rare-letter build can win, but only just.** GLM 5.3 Flash's one win came on seed 5, where it twice bought the etching that upgrades J, Q, X and Z and then used one of those letters in 25 of its 100 guesses, with words like KUDZU, KAZOO, FRIZZ and XENIA. I designed that upgrade for farming points with words you don't expect to be the answer, which is hard to afford when every round has to be solved. It still worked, though its 1.7 million points was the lowest score of the six wins at ascension 10.

**Letter upgrades don't replace relics.** DeepSeek V4 Pro spent about half of its gold on letter upgrades, against a third for Opus. On seed 4 it went further and sold three of its four relics after round 14 to pay for a letter modifier, a shape level and an alphabet pack. It solved the next round's word in four guesses and still lost, about 3,600 points short of the target.

**The difficulty is spread across the run, not saved for the end.** Opus, the strongest player, failed 1 run in the early game (seed 4, the [36-second loss](https://www.youtube.com/watch?v=a2fVOav6bKg), where it solved LOLLY in round 3 but came up 146 points short of the target, and which also ended Sol's run after 2 rounds). Sonnet, a slightly weaker player, lost at ascension 10 in rounds 16, 18, 21 and 23 across stages 6, 7 and 8, on bosses and non-bosses alike. Most runs fail uniformly in stages 1-2 and 5-8; stages 3-4 are acceptably tame as relics are strong early and target points haven't yet scaled.

**Ascension 0 can be very forgiving.** Gemini showed that common strategies, e.g. guessing CRANE over 100 times, is viable for new players that may be learning the game's mechanics while ascension 10 requires strong, consistent guesses and optimized builds around them which is to be expected by weathered players. Moreover, Step 5 Preview consistently reached round 16 of 24 while only solving half of their rounds.

**Ascension 10 punishes poor solving, but solving alone doesn't win.** Ascension 9 is ascension 10 without its final rule: every round must be solved correctly. GPT-6.1 Sol lost 3 runs on ascension 10 because it failed to solve the word. I had it play the same five seeds one level down.

| Seed | Ascension 10 | Ascension 9 |
| ---- | ------------ | ----------- |
| 1 | 11 rounds, 164 thousand | won, 4.5 million |
| 2 | 8 rounds, 55 thousand | won, 2.1 million |
| 3 | 21 rounds, 637 thousand | 20 rounds, 1.6 million |
| 4 | 2 rounds, 2 thousand | 2 rounds, 2 thousand |
| 5 | 18 rounds, 1.1 million | won, 4.7 million |

It won three of five, and its median score went from 164 thousand to 2.1 million.

DeepSeek V4.1 Flash and GLM 5.3 Flash lost most of their runs at ascension 10 with the word unsolved. On the same seeds at ascension 9, they had nearly the identical performance indicating they would have likely failed on ascension 10 for points even with strong guesses. This suggests fewer guesses per round and early game gold generation matter most, since more gold allows builds to improve enough to beat later round point targets.

To strictly evaluate model performance, ascension 9 may be a better assessment to prevent rounds with unlucky guesses from tanking scores (though it is interesting solving rounds isn't necessarily a given for these SOTA models, too).

### Word shapes

Every guess scores as one of five shapes, and certain relics and shape levels reward specific ones. About a fifth of the game's answers repeat a letter. Luna (with its JAZZY and EERIE openers) repeated one in 40% of its guesses, about twice that rate, while Opus and Sonnet did in 16% and 11% of theirs at ascension 10. Most of every model's guesses were plain words without a repeated letter.

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

From a balancing perspective, I think this can be improved to reward riskier and more creative guessing.

### Relics

The game has 47 relics, and all but 2 were purchased across all runs. Only 1 model bought the same relic in all 5 of their runs.

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

### The Shop and Economy

Run performance is strongly correlated with income and shop usage. On one hand, better runs imply more money to spend, but it's clear: no run lasts without utilizing the shop efficiently.

Conversely, overselling, e.g. GPT-6 Luna, and poor purchasing strategies, e.g. GLM 5.3 Flash with 2.8 consumables while only reaching stage 5 or DeepSeek v4 Pro with the second most purchases per stage but focusing on economy instead of points, are inversely correlated with performance.

<svg class="results" style="width:100%;height:auto;margin:1em 0" viewBox="0 0 720 502" role="img" aria-labelledby="shop-title" font-family="inherit">
<title id="shop-title">Shop actions per stage played for each model: purchases, relics sold, rerolls and consumables used.</title>
<text x="278.1" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">Purchases</text>
<text x="404.4" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">Relics sold</text>
<text x="530.6" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">Rerolls</text>
<text x="656.9" y="12.0" font-size="11" fill="#8a8878" text-anchor="middle">Consumables used</text>
<text x="0.0" y="28.0" font-size="12" fill="#8c0000" text-anchor="start" font-weight="700">Ascension 10, the hardest</text>
<text x="0.0" y="51.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Opus 5.5</text>
<rect x="216.0" y="36" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.75"><title>Claude Opus 5.5: 3.88 purchases per stage, 128 over 99 rounds</title></rect>
<text x="278.1" y="51.0" font-size="11" fill="#ffffff" text-anchor="middle">3.88</text>
<rect x="342.3" y="36" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.26"><title>Claude Opus 5.5: 0.42 relics sold per stage, 14 over 99 rounds</title></rect>
<text x="404.4" y="51.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.42</text>
<rect x="468.5" y="36" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.60"><title>Claude Opus 5.5: 0.55 rerolls per stage, 18 over 99 rounds</title></rect>
<text x="530.6" y="51.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.55</text>
<rect x="594.8" y="36" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.72"><title>Claude Opus 5.5: 0.61 consumables used per stage, 20 over 99 rounds</title></rect>
<text x="656.9" y="51.0" font-size="11" fill="#ffffff" text-anchor="middle">0.61</text>
<text x="0.0" y="77.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Sonnet 5.5</text>
<rect x="216.0" y="62" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.60"><title>Claude Sonnet 5.5: 3.15 purchases per stage, 107 over 102 rounds</title></rect>
<text x="278.1" y="77.0" font-size="11" fill="#ffffff" text-anchor="middle">3.15</text>
<rect x="342.3" y="62" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.25"><title>Claude Sonnet 5.5: 0.41 relics sold per stage, 14 over 102 rounds</title></rect>
<text x="404.4" y="77.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.41</text>
<rect x="468.5" y="62" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Claude Sonnet 5.5: 0.03 rerolls per stage, 1 over 102 rounds</title></rect>
<text x="530.6" y="77.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.03</text>
<rect x="594.8" y="62" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.52"><title>Claude Sonnet 5.5: 0.44 consumables used per stage, 15 over 102 rounds</title></rect>
<text x="656.9" y="77.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.44</text>
<text x="0.0" y="103.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6 Sol</text>
<rect x="216.0" y="88" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.58"><title>GPT-6 Sol: 3.04 purchases per stage, 73 over 72 rounds</title></rect>
<text x="278.1" y="103.0" font-size="11" fill="#2c2c2c" text-anchor="middle">3.04</text>
<rect x="342.3" y="88" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.08"><title>GPT-6 Sol: 0.13 relics sold per stage, 3 over 72 rounds</title></rect>
<text x="404.4" y="103.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.13</text>
<rect x="468.5" y="88" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.64"><title>GPT-6 Sol: 0.58 rerolls per stage, 14 over 72 rounds</title></rect>
<text x="530.6" y="103.0" font-size="11" fill="#ffffff" text-anchor="middle">0.58</text>
<rect x="594.8" y="88" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.84"><title>GPT-6 Sol: 0.71 consumables used per stage, 17 over 72 rounds</title></rect>
<text x="656.9" y="103.0" font-size="11" fill="#ffffff" text-anchor="middle">0.71</text>
<text x="0.0" y="129.0" font-size="13" fill="#2c2c2c" text-anchor="start">MiMo-V2.6-Pro</text>
<rect x="216.0" y="114" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.51"><title>MiMo-V2.6-Pro: 2.66 purchases per stage, 62 over 70 rounds</title></rect>
<text x="278.1" y="129.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2.66</text>
<rect x="342.3" y="114" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.23"><title>MiMo-V2.6-Pro: 0.39 relics sold per stage, 9 over 70 rounds</title></rect>
<text x="404.4" y="129.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.39</text>
<rect x="468.5" y="114" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.19"><title>MiMo-V2.6-Pro: 0.17 rerolls per stage, 4 over 70 rounds</title></rect>
<text x="530.6" y="129.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.17</text>
<rect x="594.8" y="114" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.56"><title>MiMo-V2.6-Pro: 0.47 consumables used per stage, 11 over 70 rounds</title></rect>
<text x="656.9" y="129.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.47</text>
<text x="0.0" y="155.0" font-size="13" fill="#2c2c2c" text-anchor="start">DeepSeek V4.1 Flash</text>
<rect x="216.0" y="140" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.49"><title>DeepSeek V4.1 Flash: 2.55 purchases per stage, 57 over 67 rounds</title></rect>
<text x="278.1" y="155.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2.55</text>
<rect x="342.3" y="140" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.16"><title>DeepSeek V4.1 Flash: 0.27 relics sold per stage, 6 over 67 rounds</title></rect>
<text x="404.4" y="155.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.27</text>
<rect x="468.5" y="140" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.10"><title>DeepSeek V4.1 Flash: 0.09 rerolls per stage, 2 over 67 rounds</title></rect>
<text x="530.6" y="155.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.09</text>
<rect x="594.8" y="140" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.58"><title>DeepSeek V4.1 Flash: 0.49 consumables used per stage, 11 over 67 rounds</title></rect>
<text x="656.9" y="155.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.49</text>
<text x="0.0" y="181.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6.1 Sol</text>
<rect x="216.0" y="166" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.51"><title>GPT-6.1 Sol: 2.68 purchases per stage, 58 over 65 rounds</title></rect>
<text x="278.1" y="181.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2.68</text>
<rect x="342.3" y="166" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.31"><title>GPT-6.1 Sol: 0.51 relics sold per stage, 11 over 65 rounds</title></rect>
<text x="404.4" y="181.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.51</text>
<rect x="468.5" y="166" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.25"><title>GPT-6.1 Sol: 0.23 rerolls per stage, 5 over 65 rounds</title></rect>
<text x="530.6" y="181.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.23</text>
<rect x="594.8" y="166" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.44"><title>GPT-6.1 Sol: 0.37 consumables used per stage, 8 over 65 rounds</title></rect>
<text x="656.9" y="181.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.37</text>
<text x="0.0" y="207.0" font-size="13" fill="#2c2c2c" text-anchor="start">DeepSeek V4 Pro</text>
<rect x="216.0" y="192" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.73"><title>DeepSeek V4 Pro: 3.79 purchases per stage, 77 over 61 rounds</title></rect>
<text x="278.1" y="207.0" font-size="11" fill="#ffffff" text-anchor="middle">3.79</text>
<rect x="342.3" y="192" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.39"><title>DeepSeek V4 Pro: 0.64 relics sold per stage, 13 over 61 rounds</title></rect>
<text x="404.4" y="207.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.64</text>
<rect x="468.5" y="192" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>DeepSeek V4 Pro: 0.00 rerolls per stage, 0 over 61 rounds</title></rect>
<text x="530.6" y="207.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.00</text>
<rect x="594.8" y="192" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.46"><title>DeepSeek V4 Pro: 0.39 consumables used per stage, 8 over 61 rounds</title></rect>
<text x="656.9" y="207.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.39</text>
<text x="0.0" y="233.0" font-size="13" fill="#2c2c2c" text-anchor="start">GLM 5.3 Flash</text>
<rect x="216.0" y="218" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.56"><title>GLM 5.3 Flash: 2.90 purchases per stage, 57 over 59 rounds</title></rect>
<text x="278.1" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2.90</text>
<rect x="342.3" y="218" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.34"><title>GLM 5.3 Flash: 0.56 relics sold per stage, 11 over 59 rounds</title></rect>
<text x="404.4" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.56</text>
<rect x="468.5" y="218" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>GLM 5.3 Flash: 0.05 rerolls per stage, 1 over 59 rounds</title></rect>
<text x="530.6" y="233.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.05</text>
<rect x="594.8" y="218" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.84"><title>GLM 5.3 Flash: 0.71 consumables used per stage, 14 over 59 rounds</title></rect>
<text x="656.9" y="233.0" font-size="11" fill="#ffffff" text-anchor="middle">0.71</text>
<text x="0.0" y="259.0" font-size="13" fill="#2c2c2c" text-anchor="start">Gemini 3.8 Flash</text>
<rect x="216.0" y="244" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.54"><title>Gemini 3.8 Flash: 2.81 purchases per stage, 59 over 63 rounds</title></rect>
<text x="278.1" y="259.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2.81</text>
<rect x="342.3" y="244" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.09"><title>Gemini 3.8 Flash: 0.14 relics sold per stage, 3 over 63 rounds</title></rect>
<text x="404.4" y="259.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.14</text>
<rect x="468.5" y="244" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.16"><title>Gemini 3.8 Flash: 0.14 rerolls per stage, 3 over 63 rounds</title></rect>
<text x="530.6" y="259.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.14</text>
<rect x="594.8" y="244" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.39"><title>Gemini 3.8 Flash: 0.33 consumables used per stage, 7 over 63 rounds</title></rect>
<text x="656.9" y="259.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.33</text>
<text x="0.0" y="290.0" font-size="12" fill="#8c0000" text-anchor="start" font-weight="700">Ascension 0, the easiest</text>
<text x="0.0" y="313.0" font-size="13" fill="#2c2c2c" text-anchor="start">Gemini 3.8 Flash</text>
<rect x="216.0" y="298" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.85"><title>Gemini 3.8 Flash: 4.42 purchases per stage, 177 over 120 rounds</title></rect>
<text x="278.1" y="313.0" font-size="11" fill="#ffffff" text-anchor="middle">4.42</text>
<rect x="342.3" y="298" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Gemini 3.8 Flash: 0.05 relics sold per stage, 2 over 120 rounds</title></rect>
<text x="404.4" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.05</text>
<rect x="468.5" y="298" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.85"><title>Gemini 3.8 Flash: 0.78 rerolls per stage, 31 over 120 rounds</title></rect>
<text x="530.6" y="313.0" font-size="11" fill="#ffffff" text-anchor="middle">0.78</text>
<rect x="594.8" y="298" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.47"><title>Gemini 3.8 Flash: 0.40 consumables used per stage, 16 over 120 rounds</title></rect>
<text x="656.9" y="313.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.40</text>
<text x="0.0" y="339.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Sonnet 5.5</text>
<rect x="216.0" y="324" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.63"><title>Claude Sonnet 5.5: 3.25 purchases per stage, 115 over 106 rounds</title></rect>
<text x="278.1" y="339.0" font-size="11" fill="#ffffff" text-anchor="middle">3.25</text>
<rect x="342.3" y="324" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.15"><title>Claude Sonnet 5.5: 0.25 relics sold per stage, 9 over 106 rounds</title></rect>
<text x="404.4" y="339.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.25</text>
<rect x="468.5" y="324" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Claude Sonnet 5.5: 0.00 rerolls per stage, 0 over 106 rounds</title></rect>
<text x="530.6" y="339.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.00</text>
<rect x="594.8" y="324" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.30"><title>Claude Sonnet 5.5: 0.25 consumables used per stage, 9 over 106 rounds</title></rect>
<text x="656.9" y="339.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.25</text>
<text x="0.0" y="365.0" font-size="13" fill="#2c2c2c" text-anchor="start">Step 5 Preview</text>
<rect x="216.0" y="350" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.56"><title>Step 5 Preview: 2.91 purchases per stage, 65 over 67 rounds</title></rect>
<text x="278.1" y="365.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2.91</text>
<rect x="342.3" y="350" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.33"><title>Step 5 Preview: 0.54 relics sold per stage, 12 over 67 rounds</title></rect>
<text x="404.4" y="365.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.54</text>
<rect x="468.5" y="350" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.25"><title>Step 5 Preview: 0.22 rerolls per stage, 5 over 67 rounds</title></rect>
<text x="530.6" y="365.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.22</text>
<rect x="594.8" y="350" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.85"><title>Step 5 Preview: 0.72 consumables used per stage, 16 over 67 rounds</title></rect>
<text x="656.9" y="365.0" font-size="11" fill="#ffffff" text-anchor="middle">0.72</text>
<text x="0.0" y="391.0" font-size="13" fill="#2c2c2c" text-anchor="start">GPT-6 Luna</text>
<rect x="216.0" y="376" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.68"><title>GPT-6 Luna: 3.56 purchases per stage, 89 over 75 rounds</title></rect>
<text x="278.1" y="391.0" font-size="11" fill="#ffffff" text-anchor="middle">3.56</text>
<rect x="342.3" y="376" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.85"><title>GPT-6 Luna: 1.40 relics sold per stage, 35 over 75 rounds</title></rect>
<text x="404.4" y="391.0" font-size="11" fill="#ffffff" text-anchor="middle">1.40</text>
<rect x="468.5" y="376" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.09"><title>GPT-6 Luna: 0.08 rerolls per stage, 2 over 75 rounds</title></rect>
<text x="530.6" y="391.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.08</text>
<rect x="594.8" y="376" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.85"><title>GPT-6 Luna: 0.72 consumables used per stage, 18 over 75 rounds</title></rect>
<text x="656.9" y="391.0" font-size="11" fill="#ffffff" text-anchor="middle">0.72</text>
<text x="0.0" y="417.0" font-size="13" fill="#2c2c2c" text-anchor="start">Claude Haiku 5.5</text>
<rect x="216.0" y="402" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.54"><title>Claude Haiku 5.5: 2.81 purchases per stage, 45 over 48 rounds</title></rect>
<text x="278.1" y="417.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2.81</text>
<rect x="342.3" y="402" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.19"><title>Claude Haiku 5.5: 0.31 relics sold per stage, 5 over 48 rounds</title></rect>
<text x="404.4" y="417.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.31</text>
<rect x="468.5" y="402" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Claude Haiku 5.5: 0.00 rerolls per stage, 0 over 48 rounds</title></rect>
<text x="530.6" y="417.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.00</text>
<rect x="594.8" y="402" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.44"><title>Claude Haiku 5.5: 0.38 consumables used per stage, 6 over 48 rounds</title></rect>
<text x="656.9" y="417.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.38</text>
<text x="0.0" y="443.0" font-size="13" fill="#2c2c2c" text-anchor="start">Nemotron 3 Ultra</text>
<rect x="216.0" y="428" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.69"><title>Nemotron 3 Ultra: 3.57 purchases per stage, 56 over 47 rounds</title></rect>
<text x="278.1" y="443.0" font-size="11" fill="#ffffff" text-anchor="middle">3.57</text>
<rect x="342.3" y="428" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.35"><title>Nemotron 3 Ultra: 0.57 relics sold per stage, 9 over 47 rounds</title></rect>
<text x="404.4" y="443.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.57</text>
<rect x="468.5" y="428" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.06"><title>Nemotron 3 Ultra: 0.00 rerolls per stage, 0 over 47 rounds</title></rect>
<text x="530.6" y="443.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.00</text>
<rect x="594.8" y="428" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.75"><title>Nemotron 3 Ultra: 0.64 consumables used per stage, 10 over 47 rounds</title></rect>
<text x="656.9" y="443.0" font-size="11" fill="#ffffff" text-anchor="middle">0.64</text>
<text x="0.0" y="469.0" font-size="13" fill="#2c2c2c" text-anchor="start">MiniMax M3</text>
<rect x="216.0" y="454" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.47"><title>MiniMax M3: 2.45 purchases per stage, 27 over 33 rounds</title></rect>
<text x="278.1" y="469.0" font-size="11" fill="#2c2c2c" text-anchor="middle">2.45</text>
<rect x="342.3" y="454" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.28"><title>MiniMax M3: 0.45 relics sold per stage, 5 over 33 rounds</title></rect>
<text x="404.4" y="469.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.45</text>
<rect x="468.5" y="454" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.20"><title>MiniMax M3: 0.18 rerolls per stage, 2 over 33 rounds</title></rect>
<text x="530.6" y="469.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.18</text>
<rect x="594.8" y="454" width="124.3" height="22" rx="2" fill="#8c0000" fill-opacity="0.54"><title>MiniMax M3: 0.45 consumables used per stage, 5 over 33 rounds</title></rect>
<text x="656.9" y="469.0" font-size="11" fill="#2c2c2c" text-anchor="middle">0.45</text>
<text x="215.0" y="496.0" font-size="11" fill="#8a8878" text-anchor="start">per stage of three rounds played, over 5 runs · each column shaded against its own highest</text>
</svg>

While earning gold does not ensure victory, earning less than $6 per round effectively disqualified a run.

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

## Caveats

The Claude models ran through Claude Code and the others through [OpenCode](https://opencode.ai/), so the harness differs as well as the model. Luna, Haiku, DeepSeek V4 Pro, Step 5 Preview and MiniMax M3 ran on OpenCode's Go plan, and the rest through [OpenRouter](https://openrouter.ai/).

Thinking effort was not held equal either. I did not set it, so each model ran at whatever its harness defaulted to: Opus at medium and Sonnet at high in Claude Code, and the OpenCode models at OpenCode's defaults. Increasing thinking for Opus and Sonnet seemed to have marginal effect either way.

The benchmark lives in the game's repository under [`tools/bench/`](https://github.com/jmelahman/5-wild/tree/master/tools/bench#readme), and every result here was played on v0.10.4.

## Watch them play

Every run is recorded as its list of moves, so any of them can be replayed through the real game. These are at the game's ×2 speed, with its own sound.

* [Opus, ascension 10, seed 5](https://www.youtube.com/watch?v=iayM2a_q9eg): the highest-scoring win, 6.5 million points
* [Opus, ascension 10, seed 4](https://www.youtube.com/watch?v=a2fVOav6bKg): the 36-second loss
* [Sonnet, ascension 0, seed 5](https://www.youtube.com/watch?v=Zp8Y4W7u8hE): the TAXIS run
* [Gemini, ascension 0, seed 2](https://www.youtube.com/watch?v=R2jsTuhsbXU): the highest-scoring of its five wins, 5.1 million points
* [Sol, ascension 10, seed 5](https://www.youtube.com/watch?v=ey_7Feq_gns): its one win, 4.4 million points
* [Luna, ascension 0, seed 3](https://www.youtube.com/watch?v=5zj1K5oiwkE): JAZZY, QUEER and EERIE to the final round

Playlists of all five runs per model, Ascension 10:

* [Opus 5.5](https://www.youtube.com/playlist?list=PLNpeJ3c8LSSI)
* [Sonnet 5.5](https://www.youtube.com/playlist?list=PLPbdCMsQ0cyw)
* [GPT-6 Sol](https://www.youtube.com/playlist?list=PLbGZ69vm1-0M)
* [GPT-6.1 Sol](https://www.youtube.com/playlist?list=PLZnD_aFf73II)
* [GLM 5.3 Flash](https://www.youtube.com/playlist?list=PLDzRfb6K_o6M)
* [DeepSeek V4.1 Flash](https://www.youtube.com/playlist?list=PLZmqrZmb2p4g)
* [MiMo-V2.6-Pro](https://www.youtube.com/playlist?list=PLeDG6SXiQnNo)
* [DeepSeek V4 Pro](https://www.youtube.com/playlist?list=PLKbSnPmJE0kU)
* [Gemini 3.8](https://www.youtube.com/playlist?list=PLKtGosf5vcbQ)

Ascension 0:

* [Gemini 3.8](https://www.youtube.com/playlist?list=PLQcOUbLko88Y)
* [Sonnet 5.5](https://www.youtube.com/playlist?list=PLZIqSWoqu3A8)
* [GPT-6 Luna](https://www.youtube.com/playlist?list=PLARlXXTMTM3Y)
* [Haiku 5.5](https://www.youtube.com/playlist?list=PLKEUYwl-L6-o)
* [Step 5 Preview](https://www.youtube.com/playlist?list=PLM9YnyAIwvoc)
* [Nemotron 3 Ultra](https://www.youtube.com/playlist?list=PLPU68aAT2rX0)
