class_name Rules
extends RefCounted
## The stylesheet itself: [code]src/ui/style.css[/code] as [Style] reads it,
## one [code][selector, props][/code] pair per rule, in the web sheet's order,
## since a later rule wins as it does there. See [Style] for the selector
## subset and what each prop means.
##
## Kept apart from the matcher because it is data and will grow with the web's
## sheet, and because the formatter lays each rule out a prop to a line.

const TABLE := [
	# ---------------------------------------------------------------- base
	["button", {"d": "row", "a": "center", "j": "center", "ta": "c"}],
	["svg", {"W": 18}],
	["strong", {"w": 700}],
	["b", {"w": 700}],
	# ------------------------------------------------------------- screens
	[".screen", {"d": "col", "gap": 6, "p": 8, "grow": 1, "maxw": 480}],
	[".screen.center", {"a": "center", "j": "center", "gap": 20}],
	# ------------------------------------------------------------------ hud
	[
		".hud",
		{
			"d": "grid",
			"cols": 3,
			"colw": [1, "a", 1],
			"gap": 8,
			"rgap": 6,
			"p": [6, 8],
			"bg": "panel",
			"r": 8
		}
	],
	[".round-screen .hud", {"mt": 0}],
	[".hud-round", {"d": "col"}],
	[".hud-title", {"d": "col"}],
	[".round-screen .hud-round", {"d": "row", "a": "center", "gap": 8}],
	[".round-name", {"w": 700, "f": 15}],
	[".stage", {"ls": 0.04, "c": "muted", "f": 11}],
	[".stage-asc", {"c": "mult", "w": 800}],
	[".hud-score", {"d": "col", "a": "center", "ta": "c", "minw": 80}],
	[".hud-score .score", {"f": 22, "w": 800}],
	[".hud-score .target", {"c": "muted", "f": 11}],
	[".hud-score.met .score", {"c": "green"}],
	[".score-line", {"d": "row", "a": "end", "j": "center", "gap": 5, "nowrap": true}],
	[".meter", {"d": "meter", "H": 3, "mt": 3, "span": true}],
	[".round-screen .meter", {"H": 5, "mt": 0}],
	[".hud-end", {"d": "row", "a": "center", "j": "end", "gap": 18}],
	[".hud-gold", {"c": "gold", "w": 800, "f": 18}],
	[
		".hud .boss",
		{
			"span": true,
			"d": "col",
			"j": "center",
			"H": Style.BOSS_BAND,
			"bg": ["mix", "mult", 14, "panel"],
			"b": [1, 0, 0, 0],
			"bc": ["mix", "mult", 40, "transparent"],
			"f": 10,
			"p": [0, 8],
			"r": 4
		}
	],
	[".boss strong", {"c": "mult"}],
	[".hud .dock-line", {"span": true}],
	[".menu-button", {"W": 32, "H": 32, "r": 6, "bg": "panel-2", "c": "muted", "f": 14}],
	# --------------------------------------------------------------- relics
	[".rarity-common", {"vars": {"rare": "rare-common"}}],
	[".rarity-uncommon", {"vars": {"rare": "rare-uncommon"}}],
	[".rarity-rare", {"vars": {"rare": "rare-rare"}}],
	[".rarity-legendary", {"vars": {"rare": "rare-legendary"}}],
	[".relics", {"d": "row", "gap": 4}],
	[
		".relic",
		{
			"grow": 1,
			"minw": 56,
			"minh": 40,
			"d": "col",
			"a": "center",
			"j": "center",
			"gap": 2,
			"p": 4,
			"r": 6,
			"bg": ["mix", "$rare|rare-common", "wash-tray", "panel-2"],
			"b": [2, 0, 0, 0],
			"bc": "$rare|rare-common",
			"ta": "c"
		}
	],
	[".relic.rarity-uncommon", {"dot": "$rare"}],
	[".relic.rarity-rare", {"dot": "$rare"}],
	[".relic.rarity-legendary", {"dot": "$rare"}],
	[".relic.empty", {"bg": "transparent", "b": 1, "bc": "line"}],
	[".relic-name", {"f": 10, "w": 700}],
	[".relic .sell", {"f": 10, "c": "gold"}],
	[".relic-detail", {"f": 10, "w": 700, "c": "mult"}],
	[".round-screen .relic", {"H": 40, "p": [2, 4], "gap": 1}],
	[".relic.fired", {"ol": ["gold", 2, 0], "sc": 1.06}],
	[".relic-tip", {"hide": true}],
	[".toast", {"hide": true}],
	# ---------------------------------------------------------- consumables
	[".consumables", {"d": "row", "gap": 4}],
	[
		".consumable",
		{
			"grow": 1,
			"d": "col",
			"a": "start",
			"gap": 1,
			"p": [4, 8],
			"r": 6,
			"bg": ["mix", "rare-legendary", "wash", "panel"],
			"b": 1,
			"bc": ["mix", "rare-legendary", 45, "transparent"],
			"ta": "l"
		}
	],
	[".consumable-name", {"f": 11, "w": 700}],
	[".consumable-text", {"f": 11, "c": "muted"}],
	[".round-screen .consumables", {"H": 40}],
	[".round-screen .consumable", {"p": [3, 8]}],
	[".round-screen .consumable-text", {"clip": true}],
	[".consumable.empty", {"bg": "transparent", "b": 1, "bc": "line"}],
	[".hand-line", {"d": "row", "gap": 4}],
	[".hand-line .consumables", {"grow": 1}],
	# ---------------------------------------------------------------- board
	[".grid-wrap", {"d": "board", "grow": 1}],
	# Set by Render.build where the web says :not(:has(.hud .boss)).
	# The band here is the header's last row, so a boss adds the header's row
	# gap as well as the band, and the room matches both.
	[".grid-wrap.band-room", {"pt": Style.BOSS_BAND + 6}],
	[".tile", {"d": "col", "a": "center", "j": "center", "r": 6, "b": 2, "bc": "line", "w": 700}],
	[".tile.filled", {"bc": "muted"}],
	[".tile.ghost", {"bc": "green", "c": ["mix", "green", 70, "fg"]}],
	[".tile.green", {"bg": "green", "bc": "green"}],
	[".tile.yellow", {"bg": "yellow", "bc": "yellow"}],
	[".tile.gray", {"bg": "gray", "bc": "gray"}],
	[".light .tile.green", {"c": "on-tile"}],
	[".light .tile.yellow", {"c": "on-tile"}],
	[".light .tile.gray", {"c": "on-tile"}],
	[".tile.pending", {"bg": "transparent", "bc": "muted", "c": "fg"}],
	[".tile[data-mod]", {"dot": "$mod"}],
	[".plain .tile[data-mod]", {"dot": null}],
	[
		".row-note",
		{
			"ls": 0.02,
			"p": [1, 5],
			"r": 999,
			"bg": "row-note-bg",
			"b": 1,
			"bc": ["mix", "yellow", 55, "transparent"],
			"c": "row-note-fg",
			"f": 11,
			"w": 800,
			"nowrap": true
		}
	],
	[".row-note", {"abs": ["br", -2, 6]}],
	# What the scoring animation marks: a tile, relic or category as it fires.
	[".tile.fired", {"ol": ["$mod|gold", 2, 2]}],
	[".quiet .tile.fired", {"ol": null}],
	[".row-note.pending", {"op": 0}],
	# ------------------------------------------------------------ coaching
	[".coach-slot", {"abs": ["below", 0, 6], "span": true}],
	[
		".coach",
		{
			"p": [5, 10],
			"r": 8,
			"bg": ["mix", "panel-2", 94, "bg"],
			"b": 1,
			"bc": ["mix", "chips", 40, "line"]
		}
	],
	[".coach-text", {"f": 12, "c": "fg"}],
	[".coached", {"ol": [["mix", "chips", 70, "transparent"], 2, 3]}],
	# ------------------------------------------------------ dock and readout
	[".dock-line", {"d": "row", "a": "center", "gap": 5}],
	[".category-slot", {"d": "row", "minh": 36, "grow": 36}],
	[
		".category",
		{
			"grow": 1,
			"d": "row",
			"a": "center",
			"j": "center",
			"gap": 4,
			"H": 36,
			"p": [0, 6, 0, 8],
			"r": 8,
			"b": 1,
			"bc": "line",
			"bg": "panel",
			"f": 12,
			"w": 700,
			"c": "fg",
			"nowrap": true
		}
	],
	[".category-name", {"clip": true}],
	[".category-level", {"p": [0, 4], "r": 3, "bg": ["mix", "muted", 18, "transparent"]}],
	[".category .icon", {"W": 12, "c": "muted"}],
	[".category.fired", {"c": "gold", "sc": 1.08}],
	[".category.fired .category-level", {"bg": ["mix", "gold", 26, "panel"]}],
	[".readout", {"d": "row", "a": "center", "gap": 4, "f": 18, "w": 800, "grow": 64}],
	[
		".readout .chips",
		{
			"grow": 1,
			"H": 36,
			"d": "row",
			"a": "center",
			"j": "center",
			"p": [0, 7],
			"r": 8,
			"bg": ["mix", "chips", 22, "panel"],
			"c": "chips"
		}
	],
	[
		".readout .mult",
		{
			"grow": 1,
			"H": 36,
			"d": "row",
			"a": "center",
			"j": "center",
			"p": [0, 7],
			"r": 8,
			"bg": ["mix", "mult", 22, "panel"],
			"c": "mult"
		}
	],
	[
		".readout.drafting .mult.pending",
		{
			"bg": "transparent",
			"b": 2,
			"bc": ["mix", "mult", 50, "transparent"],
			"c": ["mix", "mult", 75, "transparent"]
		}
	],
	[".readout.solved .chips", {"bg": ["mix", "gold", 26, "panel"], "c": "gold"}],
	[".readout.solved .mult", {"bg": ["mix", "gold", 26, "panel"], "c": "gold"}],
	[".readout .times", {"c": "muted", "f": 16}],
	[".decor-toggle", {"W": 40, "H": 40, "r": 6, "bg": "panel-2", "c": "fg", "f": 14, "w": 800}],
	[".decor-toggle-value", {"f": 10, "w": 400, "c": "value-base"}],
	# ------------------------------------------------------------- keyboard
	[".keyboard", {"d": "col", "gap": 6}],
	[".key-row", {"d": "keys", "gap": 4, "j": "center"}],
	[".key", {"H": Style.KEY_H, "r": 6, "bg": "key", "f": 16, "w": 700, "keyw": 1}],
	[".key .icon", {"W": 22}],
	[".key.wide", {"f": 11, "keyw": 1.5}],
	[".key.green", {"bg": "green"}],
	[".key.yellow", {"bg": "yellow"}],
	[".key.gray", {"bg": "gray", "c": "key-gray-fg"}],
	[".light .key.green", {"c": "on-tile"}],
	[".light .key.yellow", {"c": "on-tile"}],
	[".key.broken", {"bg": "broken-bg", "c": "broken-fg", "strike": true}],
	[".value-pip", {"abs": ["tr", -3, 1], "f": 10, "w": 400, "c": "value-base", "strike": false}],
	[".key.green .value-pip", {"c": ["mix", "on-tile", 72, "transparent"]}],
	[".key.yellow .value-pip", {"c": ["mix", "on-tile", 72, "transparent"]}],
	[".key.gray .value-pip", {"c": ["mix", "on-tile", 72, "transparent"]}],
	[".key.etched .value-pip", {"c": "chips", "w": 600}],
	[".key.broken .value-pip", {"c": "broken-fg"}],
	["[data-mod=chip]", {"vars": {"mod": "chips"}}],
	["[data-mod=mult]", {"vars": {"mod": "mult"}}],
	["[data-mod=gold]", {"vars": {"mod": "gold"}}],
	["[data-mod=wild]", {"vars": {"mod": "rare-legendary"}}],
	["[data-mod=steel]", {"vars": {"mod": "mod-steel"}}],
	["[data-mod=glass]", {"vars": {"mod": "mod-glass"}}],
	["[data-mod=lucky]", {"vars": {"mod": "mod-lucky"}}],
	["[data-mod=echo]", {"vars": {"mod": "mod-echo"}}],
	["[data-mod=anchor]", {"vars": {"mod": "mod-anchor"}}],
	[".key[data-mod]", {"b": 2, "bc": ["mix", "$mod", 62, "transparent"]}],
	[".key.broken[data-mod]", {"b": 0}],
	[".plain .key[data-mod]", {"b": 0}],
	[".mod-pip", {"abs": ["bl", 3, -1], "f": 10, "w": 800, "c": "$mod", "strike": false}],
	[".key.broken .mod-pip", {"c": "broken-fg"}],
	[".mod-pip.silenced", {"c": "broken-fg"}],
	[".quiet .key!etched .value-pip", {"hide": true}],
	[".plain .value-pip", {"hide": true}],
	[".plain .mod-pip", {"hide": true}],
	# ----------------------------------------------------------------- shop
	[".shop-screen", {"scroll": true, "gap": 8, "p": [8, 8, 0, 8]}],
	[".shop-items", {"d": "grid", "cols": 2, "gap": 8}],
	[
		".shop-item",
		{
			"d": "col",
			"a": "stretch",
			"gap": 4,
			"minh": 136,
			"p": [12, 8],
			"r": 12,
			"b": 1,
			"bc": ["mix", "$rare|rare-common", "edge", "line"],
			"bg": ["mix", "$rare|rare-common", "wash", "panel-2"],
			"lip": 3,
			"ta": "l"
		}
	],
	[".shop-item.kind-pack", {"span": true, "minh": 0}],
	[".shop-item.broke", {"op": 0.5}],
	[
		".shop-item.sold",
		{
			"d": "row",
			"a": "center",
			"j": "center",
			"gap": 6,
			"bg": "transparent",
			"b": 1,
			"bc": "line",
			"lip": 0,
			"c": "muted",
			"f": 14
		}
	],
	[".shop-item.sold .icon", {"W": 16, "c": "green"}],
	[".shop-item-head", {"d": "row", "a": "center", "gap": 8}],
	[".shop-item-head .icon", {"W": 24, "c": ["mix", "$rare|rare-common", 70, "fg"]}],
	[".shop-item-name", {"grow": 1, "f": 16, "w": 800}],
	[".shop-item-text", {"f": 13, "c": ["mix", "fg", 62, "muted"]}],
	[".shop-item-swap", {"f": 10, "w": 700, "c": "$mod"}],
	[
		".shop-item-foot",
		{"d": "row", "a": "center", "j": "between", "gap": 8, "push": true, "pt": 6}
	],
	[".shop-item-cost", {"c": "gold", "f": 18, "w": 900}],
	[".shop-item.broke .shop-item-cost", {"c": "muted"}],
	[".shop-item-cost.free", {"c": "muted", "f": 14, "strike": true}],
	[
		".shop-item-kind",
		{
			"d": "row",
			"a": "center",
			"gap": 5,
			"f": 10,
			"w": 700,
			"c": ["mix", "$rare|rare-common", 70, "fg"]
		}
	],
	[".shop-item-kind .icon", {"W": 16}],
	[".shop-item-kind.blocked", {"c": "mult"}],
	[".shop-item-full", {"c": "mult", "f": 10, "w": 700}],
	[".owned-label", {"p": [4, 0], "f": 11, "c": "muted"}],
	[".shop-actions", {"d": "row", "gap": 8, "push": true, "p": [8, 0, 16, 0]}],
	[
		".shop-actions .primary",
		{"grow": 5, "minh": 58, "r": 10, "f": 19, "w": 800, "lip": 4, "gap": 8}
	],
	[
		".shop-actions .secondary",
		{"grow": 3, "minh": 58, "r": 10, "f": 16, "w": 800, "lip": 3, "gap": 8}
	],
	[".reroll-cost", {"c": "gold"}],
	[
		".shapes-line",
		{
			"d": "row",
			"a": "center",
			"gap": 10,
			"minh": 44,
			"push": true,
			"p": [0, 12],
			"r": 10,
			"bg": ["mix", "panel", 75, "transparent"],
			"f": 14,
			"c": "muted",
			"ta": "l"
		}
	],
	[".shapes-line-label", {"w": 700, "c": "fg"}],
	[".shapes-line-body", {"grow": 1, "clip": true, "c": "muted"}],
	[".shapes-line-more", {"op": 0.65}],
	# --------------------------------------------------------------- shapes
	[".shapes-note", {"f": 13, "c": "muted"}],
	[".shapes", {"d": "col", "gap": 4}],
	[
		".shape",
		{
			"d": "grid",
			"cols": 2,
			"colw": [1, "a"],
			"gap": 12,
			"rgap": 2,
			"p": [6, 8],
			"b": [0, 0, 0, 2],
			"bc": "line",
			"r": [0, 4, 4, 0]
		}
	],
	[".shape.scoring", {"bc": "gold", "bg": ["mix", "gold", 10, "transparent"]}],
	[".shape.matched", {"bc": ["mix", "mult", 55, "transparent"]}],
	[".shape-head", {"d": "row", "a": "center", "gap": 8, "w": 700}],
	[".shape-level", {"p": [0, 4], "r": 3, "bg": ["mix", "muted", 18, "transparent"], "f": 12}],
	[
		".shape-tag",
		{
			"ls": 0.06,
			"p": [0, 4],
			"r": 3,
			"bg": ["mix", "gold", 26, "panel"],
			"f": 11,
			"w": 700,
			"up": true,
			"c": "gold"
		}
	],
	[".shape-tag.also", {"ls": 0.04, "bg": "transparent", "c": "muted"}],
	[".shape-text", {"f": 13}],
	[".shape-pay", {"ta": "r", "f": 12, "w": 600, "c": "mult"}],
	# ---------------------------------------------------------- packs, place
	[".pack-hint", {"f": 13, "c": "muted"}],
	[".pack-options", {"d": "col", "gap": 8}],
	[".pack-options .shop-item", {"minh": 0}],
	[".place-keys .key", {"H": 44}],
	[".place-chips", {"abs": ["tr", -3, 1], "f": 10, "w": 400, "c": "value-base"}],
	[".place-key.etched .place-chips", {"c": "chips", "w": 600}],
	[".place-key.taken", {"op": 0.7}],
	[".place-key.arming", {"b": 2, "bc": "$mod"}],
	[".place-key[disabled]", {"bg": "broken-bg", "c": "broken-fg", "b": 0}],
	[".place-key[disabled] .place-chips", {"c": "broken-fg"}],
	[
		".place-swap",
		{
			"d": "col",
			"gap": 10,
			"p": 10,
			"r": 8,
			"bg": "panel-2",
			"b": 1,
			"bc": ["mix", "$mod", 45, "transparent"]
		}
	],
	[".place-swap-line", {"f": 13, "c": "fg"}],
	[".place-swap-line strong", {"c": "$mod", "w": 700}],
	[".place-note", {"f": 12, "c": "muted"}],
	# -------------------------------------------------------------- buttons
	[".primary", {"grow": 1, "p": [12, 16], "r": 8, "w": 700, "bg": "green"}],
	[".secondary", {"grow": 1, "p": [12, 16], "r": 8, "w": 700, "bg": "panel-2"}],
	[
		".danger",
		{
			"grow": 1,
			"p": [12, 16],
			"r": 8,
			"w": 700,
			"bg": "transparent",
			"b": 1,
			"bc": ["mix", "mult", 50, "transparent"],
			"c": "mult"
		}
	],
	[".light .primary", {"c": "on-tile"}],
	[".primary[disabled]", {"op": 0.4}],
	[".secondary[disabled]", {"op": 0.4}],
	# ---------------------------------------------------- reward, end, stats
	[".panel", {"d": "col", "gap": 8, "p": 16, "r": 12, "bg": "panel", "maxw": 352}],
	[".reward-line", {"d": "row", "j": "between", "f": 14, "c": "muted"}],
	[
		".reward-line.total",
		{"pt": 8, "b": [1, 0, 0, 0], "bc": "line", "c": "gold", "w": 800, "f": 16}
	],
	[".banner", {"ls": -0.01, "f": 28, "w": 800, "ta": "c"}],
	[".banner.win", {"c": "green"}],
	[".banner.lose", {"c": "mult"}],
	[".answer-note", {"f": 16, "w": 700}],
	[".score-note", {"f": 13, "c": "muted"}],
	[".won-note", {"c": "green", "w": 700}],
	[".asc-note", {"c": "gold", "w": 700}],
	[".endless-note", {"mt": 8, "f": 13, "c": "muted", "ta": "l"}],
	[".inline-link", {"c": "fg"}],
	[
		".share-thanks",
		{"d": "row", "a": "center", "j": "center", "gap": 7, "f": 11, "w": 600, "c": "fg"}
	],
	[".icon-heart", {"c": "mult", "fill": true}],
	[".screen.center .primary", {"grow": 0, "maxw": 352}],
	[".screen.center .secondary", {"grow": 0, "maxw": 352}],
	[".figures", {"d": "grid", "cols": 3, "gap": 4}],
	[".figure", {"d": "col", "a": "center", "gap": 1, "p": [8, 4], "r": 6, "bg": "panel-2"}],
	[".figure strong", {"f": 18}],
	[".figure span", {"ls": 0.06, "f": 10, "up": true, "c": "muted"}],
	[".stat-line", {"f": 13, "c": "muted"}],
	[".stat-line strong", {"c": "fg"}],
	[".stat-head", {"ls": 0.08, "f": 11, "up": true, "c": "muted", "w": 700}],
	[".breakdown", {"d": "col", "gap": 4}],
	[
		".breakdown-row",
		{"d": "grid", "cols": 4, "colw": ["p88", 1, "p36", "p32"], "gap": 6, "f": 12}
	],
	[".breakdown-label", {"c": "muted"}],
	[".breakdown-track", {"d": "bar", "H": 8, "r": 4, "bg": "panel-2", "c": "green"}],
	[".breakdown-row.lost .breakdown-track", {"c": "gray"}],
	[".breakdown-share", {"ta": "r", "w": 700}],
	[".breakdown-count", {"ta": "r", "c": "muted"}],
	[".stat-foot", {"f": 11, "c": "muted"}],
	# ---------------------------------------------------------------- intro
	[".intro", {"gap": 12, "p": [12, 16, 20, 16], "scroll": true}],
	[
		".intro-stage",
		{
			"ls": 0.14,
			"minh": 44,
			"d": "row",
			"a": "center",
			"f": 12,
			"w": 700,
			"up": true,
			"c": "muted"
		}
	],
	[".round-token", {"W": 18}],
	[".intro-card .round-token", {"W": 72}],
	[".track-name .round-token", {"W": 16}],
	[".hud .round-token", {"W": 30}],
	[".stage-track", {"d": "grid", "cols": 3, "gap": 6}],
	[
		".track-round",
		{
			"d": "col",
			"gap": 4,
			"p": 10,
			"r": 10,
			"b": [1, 1, 3, 1],
			"bc": "transparent",
			"bg": "panel",
			"vars": {"edge": "#5a7fa8"}
		}
	],
	[".track-round.elite", {"vars": {"edge": "#c08a3e"}}],
	[".track-round.boss", {"vars": {"edge": "mult"}}],
	[".track-round.current", {"bc": "$edge", "bg": "panel-2"}],
	[
		".track-name",
		{"d": "row", "a": "center", "gap": 6, "f": 12, "w": 700, "c": "muted", "nowrap": true}
	],
	[".track-round.current .track-name", {"c": "fg"}],
	[".track-target", {"f": 15, "w": 800}],
	[".track-round.cleared .track-target", {"c": "muted", "strike": true}],
	[".track-round.cleared .round-token", {"op": 0.55}],
	[
		".track-status",
		{"d": "row", "a": "center", "gap": 4, "minh": 14, "f": 11, "w": 700, "c": "muted"}
	],
	[".track-round.cleared .track-status", {"c": ["mix", "green", 70, "fg"]}],
	[".track-round.current .track-status", {"c": ["mix", "$edge", 60, "fg"]}],
	[".track-status .icon", {"W": 12}],
	[".intro-body", {"grow": 1, "d": "col", "j": "center"}],
	[
		".intro-card",
		{
			"d": "col",
			"a": "center",
			"p": [28, 20, 22, 20],
			"r": 16,
			"b": 1,
			"bc": "line",
			"bg": "panel",
			"lip": 4
		}
	],
	[
		".intro-card.boss-card",
		{"b": [2, 2, 4, 2], "bc": "mult", "bg": ["mix", "mult", 6, "panel"], "lip": 0}
	],
	[".intro-kind", {"ls": 0.14, "mt": 14, "f": 11, "w": 800, "up": true, "c": "mult"}],
	[".intro-name", {"ls": -0.01, "mt": 14, "f": 28, "w": 800, "ta": "c"}],
	[
		".intro-rule",
		{
			"fillw": true,
			"mt": 12,
			"p": [10, 14],
			"r": 10,
			"b": 1,
			"bc": ["mix", "mult", 28, "transparent"],
			"bg": ["mix", "mult", 8, "transparent"],
			"ta": "c",
			"f": 14,
			"c": ["mix", "mult", 20, "fg"]
		}
	],
	[".intro-label", {"ls": 0.14, "mt": 20, "f": 11, "w": 700, "up": true, "c": "muted"}],
	[".intro-target", {"ls": -0.02, "f": 64, "w": 800, "c": "chips"}],
	[
		".intro-stats",
		{"fillw": true, "mt": 20, "d": "grid", "cols": 2, "b": [1, 0, 0, 0], "bc": "line"}
	],
	[".boss-card .intro-stats", {"bc": ["mix", "mult", 22, "line"]}],
	[".intro-stat", {"d": "col", "a": "center", "gap": 2, "pt": 14}],
	[".intro-stat-value", {"f": 22, "w": 800}],
	[".intro-stat-value.gold", {"c": "gold"}],
	[".intro-stat-label", {"f": 12, "c": "muted"}],
	[
		".intro-asc",
		{
			"fillw": true,
			"mt": 14,
			"pt": 10,
			"b": [1, 0, 0, 0],
			"bc": "line",
			"ta": "c",
			"f": 11,
			"c": "muted"
		}
	],
	[".intro-asc strong", {"c": "mult"}],
	[".intro-actions", {"d": "col", "gap": 8}],
	[".intro-actions .primary", {"grow": 0, "H": 56, "r": 12, "f": 18, "w": 800, "lip": 4}],
	# ---------------------------------------------------------------- title
	[".screen.title", {"gap": 0, "p": [56, 20, 16, 20], "a": "center"}],
	[".title-mast", {"d": "col", "a": "center"}],
	[".title-ghost", {"d": "col", "gap": 6, "p": [0, 0, 6, 0]}],
	[".title-ghost-row", {"d": "row", "gap": 6}],
	[".title-ghost-row:first", {"op": 0.35}],
	[".title-ghost-row:last", {"op": 0.6}],
	[".title-ghost-tile", {"W": 62, "H": 62, "r": 6, "b": 2, "bc": "line"}],
	[".title-word", {"d": "col"}],
	[".title-name", {"d": "row", "gap": 6, "f": 36, "w": 900}],
	[
		".title-tile",
		{"W": 62, "H": 62, "a": "center", "j": "center", "r": 6, "bg": "gray", "lip": 5, "c": "fg"}
	],
	[".light .title-tile", {"c": "on-tile"}],
	[".title-tile.wild", {"bg": "green"}],
	[
		".title-score",
		{"abs": ["tr", 14, -16], "d": "row", "a": "center", "gap": 3, "f": 13, "w": 900, "rot": 6}
	],
	[".title-chips", {"p": [3, 8], "r": 6, "c": "on-bright", "bg": "chips"}],
	[".title-mult", {"p": [3, 8], "r": 6, "c": "on-bright", "bg": "mult"}],
	[".title-times", {"f": 12, "c": "muted"}],
	[".title-tag", {"mt": 18, "c": ["mix", "fg", 50, "muted"], "f": 16, "ta": "c"}],
	[".title-record", {"ls": 0.06, "mt": 8, "c": "muted", "f": 12}],
	[
		".ladder",
		{
			"fillw": true,
			"d": "col",
			"gap": 4,
			"minh": 102,
			"mt": 32,
			"p": 10,
			"r": 12,
			"b": 1,
			"bc": "line",
			"bg": ["mix", "panel", 85, "transparent"]
		}
	],
	[".ladder.lit", {"bc": ["mix", "mult", 45, "transparent"], "bg": ["mix", "mult", 12, "panel"]}],
	[".ladder.locked", {"bc": ["mix", "muted", 55, "transparent"]}],
	[".ladder-row", {"d": "row", "a": "center", "gap": 8}],
	[".ladder-level", {"d": "col", "grow": 1, "a": "center", "gap": 4, "ta": "c"}],
	[".ladder-name", {"d": "row", "a": "center", "gap": 6, "f": 17, "w": 800}],
	[".ladder-lock", {"c": "gold"}],
	[".ladder-lock .icon", {"W": 14}],
	[".ladder-rule", {"f": 12, "w": 700, "c": "mult"}],
	[".ladder!lit .ladder-rule", {"c": "muted"}],
	[".ladder-step", {"W": 44, "H": 44, "r": 10, "bg": "panel-2", "c": "fg", "f": 20}],
	[".ladder-step[disabled]", {"op": 0.3}],
	[".ladder-step.shut", {"c": "gold"}],
	[".ladder-step .icon", {"W": 16}],
	[".ladder-text", {"grow": 1, "d": "col", "j": "center", "ta": "c", "f": 12, "c": "muted"}],
	[".ladder-text.carrot", {"c": "gold"}],
	[
		".title-play",
		{"fillw": true, "grow": 0, "H": 56, "mt": 14, "r": 10, "f": 19, "w": 800, "lip": 4}
	],
	[".title-more", {"fillw": true, "d": "grid", "cols": 2, "gap": 10, "mt": 12}],
	[".title-more .secondary", {"gap": 7, "minh": 48, "p": [6, 4], "r": 10, "f": 15, "lip": 3}],
	[".title-more .icon", {"c": "muted"}],
	[
		".title-foot",
		{
			"fillw": true,
			"d": "grid",
			"cols": 3,
			"colw": ["a", 1, "a"],
			"a": "center",
			"gap": 8,
			"push": true,
			"pt": 16
		}
	],
	[
		".title-pill",
		{
			"H": 44,
			"p": [0, 14],
			"gap": 8,
			"r": 999,
			"bg": "panel",
			"f": 14,
			"c": ["mix", "fg", 50, "muted"]
		}
	],
	[".title-dials", {"d": "row", "gap": 8, "j": "end"}],
	[".title-dial", {"W": 44, "H": 44, "r": 999, "bg": "panel", "c": ["mix", "fg", 50, "muted"]}],
	[".title-build", {"ls": 0.04, "ta": "c", "f": 12, "c": "muted", "op": 0.65, "clip": true}],
	[".title-build-commit", {"hide": true}],
	# ---------------------------------------------------------------- sheets
	[".overlay", {"d": "col", "j": "end", "a": "center", "p": 16, "bg": "scrim"}],
	[
		".sheet",
		{
			"d": "col",
			"gap": 12,
			"p": 20,
			"r": 16,
			"bg": "panel",
			"b": 1,
			"bc": "line",
			# width: min(100%, 26rem), not a max-width: a sheet that shrank to
			# its content changed size when a codex section opened.
			"fillw": true,
			"maxw": 416,
			"maxh": 0.8
		}
	],
	[".sheet-title", {"f": 20, "w": 800}],
	[".sheet-body", {"scroll": true, "d": "col", "gap": 10, "f": 14, "c": "muted", "grow": 1}],
	[".sheet-lead", {"c": "fg"}],
	[
		".help-tutorial",
		{
			"d": "col",
			"gap": 8,
			"p": [10, 12],
			"r": 8,
			"b": 1,
			"bc": ["mix", "chips", 40, "line"],
			"bg": "panel-2",
			"c": "fg"
		}
	],
	[".sheet-heading", {"ls": 0.08, "mt": 6, "f": 12, "up": true, "c": "muted"}],
	[".rule strong", {"c": "fg"}],
	[".codex-section", {"d": "col", "b": [1, 0, 0, 0], "bc": "line"}],
	[
		".codex-summary",
		{"d": "row", "a": "center", "j": "between", "gap": 8, "p": [10, 0], "w": 700, "c": "fg"}
	],
	[".codex-count", {"f": 12, "w": 600, "c": "muted"}],
	[".codex-blurb", {"f": 13, "c": "muted", "p": [0, 0, 4, 0]}],
	[
		".codex-group",
		{"ls": 0.08, "mt": 4, "p": [4, 0], "f": 11, "w": 700, "up": true, "c": "$rare|muted"}
	],
	[
		".codex-entry",
		{"d": "col", "gap": 2, "p": [6, 0, 6, 10], "b": [0, 0, 0, 2], "bc": "$rare|line"}
	],
	[
		".codex-entry-head",
		{"d": "row", "a": "center", "j": "between", "gap": 8, "c": "fg", "w": 700}
	],
	[".codex-note", {"f": 12, "w": 600, "c": "muted"}],
	[".codex-text", {"f": 13}],
	[".sheet-actions", {"d": "col", "gap": 8}],
	[".sheet-actions .primary", {"grow": 0}],
	[".sheet-actions .secondary", {"grow": 0}],
	[".sheet-actions .danger", {"grow": 0}],
	[".sheet .primary", {"grow": 0}],
	[".sheet-link", {"d": "row", "a": "center", "j": "center"}],
	[".settings", {"d": "col", "r": 8, "bg": "panel-2"}],
	[
		".setting",
		{
			"d": "row",
			"a": "center",
			"j": "between",
			"gap": 12,
			"minh": 44,
			"p": [8, 16],
			"ta": "l",
			"f": 14
		}
	],
	[".setting[disabled]", {"c": "muted"}],
	[".setting-value", {"d": "row", "a": "center", "gap": 8, "c": "muted", "nowrap": true}],
	[".lang-value", {"d": "row", "a": "center", "gap": 8}],
	[".setting-more", {"f": 17}],
	[".switch", {"d": "switch", "W": 36, "H": 20, "r": 999, "bg": "gray", "c": "muted"}],
	["[aria-checked=true] .switch", {"bg": "green", "c": "fg"}],
	[".lang-button", {"d": "row", "a": "center", "j": "center", "gap": 8}],
	[".lang-note", {"f": 11, "c": "muted"}],
	[".sheet-note", {"f": 11, "c": "muted"}],
]
