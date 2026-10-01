import type { Color, Growth } from "../../engine"
import { formatNumber as num, pluralizer } from "../format"
import type { Strings } from "./types"

/** A level's factor on the mult, to two places at most: ×1.3, ×1.69. */
const factor = (times: number): string => `×${Number(times.toFixed(2))}`.replace(".", ",")

const plural = pluralizer("de")

/** Read by one refusal, which has to name the card it is turning down. */
const MODIFIER: Strings["modifier"] = {
  chip: { name: "Chip", text: "bringt +20 Punkte" },
  mult: { name: "Mult", text: "bringt +8 Mult" },
  gold: { name: "Gold", text: "bringt $1, jedes Mal wenn du ihn spielst" },
  wild: { name: "Joker", text: "bringt +24 Mult auf Grau, +14 auf Gelb, +4 auf Grün" },
  lucky: { name: "Glückspilz", text: "bringt mit 1-zu-4-Chance +20 Mult" },
  echo: { name: "Echo", text: "bringt +60 Punkte, wenn das Wort ihn wiederholt" },
  anchor: { name: "Anker", text: "bringt +125 Punkte, wenn er grün wird" },
  steel: { name: "Stahl", text: "bringt ×2 Mult" },
  glass: { name: "Glas", text: "bringt ×3 Mult und kann zerbrechen, wenn er grau wird" },
}

const UNIT: Record<Exclude<Growth["unit"], "gold">, string> = { chips: "Punkte", mult: "Mult" }

const COLOR: Record<Color, string> = { green: "grün", yellow: "gelb", gray: "grau" }

/**
 * German. `en.ts` is the reference; this is one filling of the same shape.
 *
 * The vocabulary, which is mostly a matter of picking one word and never
 * wavering: what the engine calls a *chip* is a `Punkt` (it
 * was a `Chip` until the English went to "points"), a *guess* is a `Versuch`, a *run* is a
 * `Durchlauf` — `Lauf` alone is a footrace — a *stage* is an `Etappe` and a
 * *round* a `Runde`. A *pack* is a `Booster`, which is what German card players
 * call one.
 *
 * Two decisions worth naming because they cost width, and width is the scarce
 * thing on a 360px board.
 *
 * *Level* is `Stufe`, spelled out everywhere including the badges. German has no
 * abbreviation for it that is not worse than the word: `St.` reads as `Sankt`
 * and `Lv` is not German at all. `Stufe 3` is seven characters against English's
 * `Lv 3` at five, and the board's shape badge and the shop's level rows are the
 * two places to check that against, since they are the narrowest.
 *
 * The address is `du`, not `Sie`. Every sentence the game says is either an
 * instruction to the player or a description of what just happened to them, and
 * `Sie` would put a counter between the two that the English does not have.
 */
export const de: Strings = {
  tag: "de",
  endonym: "Deutsch",

  relic: {
    green_thumb: { name: "Grüner Daumen", text: "+4 Punkte pro grünem Feld" },
    scavenger: { name: "Aasfresser", text: "+$1 pro gelbem Feld" },
    vowel_hoarder: { name: "Vokalhorter", text: "+4 Mult pro Vokal" },
    slow_burn: {
      name: "Schwelbrand",
      text: "+3 Mult für jeden Versuch schon in der Runde. Gewinnt dauerhaft +3 mehr pro Runde, die in 4 Versuchen oder mehr geschafft ist",
    },
    consonant_cluster: {
      name: "Konsonantenhäufung",
      text: "×1.5 Mult, wenn das Wort 3+ Konsonanten hintereinander hat",
    },
    cold_open: { name: "Kaltstart", text: "+30 Punkte beim ersten Versuch einer Runde" },
    bloodhound: {
      name: "Bluthund",
      text: "Gewinnt dauerhaft +1 Punkt für jedes gelbe Feld, das du spielst",
    },
    head_start: { name: "Vorsprung", text: "+15 Mult, wenn das Wort mit einem Vokal beginnt" },
    loaded_dice: { name: "Gezinkte Würfel", text: "+0 bis +20 Mult, bei jedem Versuch neu" },
    anagrammer: { name: "Anagrammist", text: "×1,5 Mult, wenn sich kein Buchstabe wiederholt" },
    keystone: { name: "Schlussstein", text: "×2 Mult, wenn das mittlere Feld grün ist" },
    lexicographer: {
      name: "Lexikograph",
      text: "+3 Punkte pro Buchstabe, den deine früheren Versuche der Runde nicht hatten",
    },
    sunk_cost: { name: "Versunkene Kosten", text: "+6 Mult pro Versuch, der dir bliebe" },
    speedrunner: { name: "Sprinter", text: "×3 Mult, wenn du in 3 Versuchen oder weniger löst" },
    qs_bargain: { name: "Das Q-Schnäppchen", text: "J, Q, X und Z bringen dreifache Punkte" },
    greedy_grammarian: { name: "Gieriger Grammatiker", text: "+15 Punkte pro grauem Feld" },
    doppelganger: {
      name: "Doppelgänger",
      text: "Wiederholte Buchstaben zählen doppelt, und je +5 Mult",
    },
    hot_streak: {
      name: "Glückssträhne",
      text: "Gewinnt dauerhaft +12 Punkte pro Runde, die in 3 Versuchen oder weniger geschafft ist",
    },
    hoarder: {
      name: "Der Hamsterer",
      text: "Gewinnt dauerhaft +10 Punkte, wenn du mit beiden Kartenplätzen voll in den Laden kommst",
    },
    masochist: {
      name: "Masochist",
      text: "Einmal pro Runde dauerhaft +10 Mult, wenn ein Versuch nach dem ersten 4 in dieser Runde noch nicht gespielte Buchstaben ausschließt",
    },
    chorus: {
      name: "Der Chor",
      text: "×3 Mult, wenn das Wort drei oder mehr Vokale hat und einer davon grün ist",
    },
    alphabetist: {
      name: "Alphabetist",
      text: "×2 Mult, wenn deine Buchstaben in alphabetischer Reihenfolge stehen",
    },
    vault: { name: "Der Tresor", text: "+25 Punkte für jeden Versuch schon in der Runde" },
    mint: {
      name: "Die Münzstätte",
      text: "Gewinnt dauerhaft +1 Mult pro $2, die du am Rundenende hältst, bis $25. Du bekommst keine Zinsen.",
    },
    scorched_earth: {
      name: "Verbrannte Erde",
      text: "+12 Mult pro Buchstabe, der aus dem Alphabet zerbrochen ist",
    },
    snowball: {
      name: "Schneeball",
      text: "Gewinnt dauerhaft +1 Mult für jedes grüne Feld, das du spielst",
    },
    long_game: { name: "Das lange Spiel", text: "+1 auf deinen Lösungs-Multiplikator" },
    pyromaniac: {
      name: "Pyromane",
      text: "+40 Mult. Zerbricht in jeder Runde einen zufälligen Buchstaben aus dem Alphabet",
    },
    fresh_ink: {
      name: "Frische Tinte",
      text: "+15 Mult. 1 zu 6, dass sie am Ende jeder Runde eintrocknet",
    },
    first_draft: { name: "Erster Entwurf", text: "+20 Mult, 4 weniger am Ende jeder Runde" },
    candle: { name: "Kerze", text: "+30 Punkte, 6 weniger am Ende jeder Runde" },
    reserve: { name: "Reserve", text: "+4 Punkte pro Versuch, der dir bliebe" },
    collector: { name: "Sammler", text: "+4 Mult pro Relikt, das du hältst" },
    second_look: {
      name: "Zweiter Blick",
      text: "Dein erstes Neuauslegen in jedem Laden ist kostenlos",
    },
    stipend: { name: "Stipendium", text: "Erhalte $2 am Ende jeder geschafften Runde" },
    first_impression: { name: "Erster Eindruck", text: "×2 Mult, wenn das erste Feld grün ist" },
    twins: { name: "Zwillinge", text: "×2,5 Mult, wenn ein Buchstabe zweimal vorkommt" },
    blank_page: {
      name: "Leeres Blatt",
      text: "×1 Mult pro leerem Reliktplatz, dieser eingeschlossen",
    },
    habit: {
      name: "Macht der Gewohnheit",
      text: "+3 Mult für jedes Wort derselben Form, das seit dem Kauf gespielt wurde",
    },
    no_maybes: {
      name: "Kein Vielleicht",
      text: "×1,75 Mult, wenn der Versuch kein gelbes Feld hat",
    },
    compound: { name: "Zinseszins", text: "Verdoppelt deine Zinsen" },
    royalties: {
      name: "Tantiemen",
      text: "Erhalte $1 am Ende jeder geschafften Runde, und $1 mehr für jeden besiegten Boss",
    },
    thesaurus: { name: "Thesaurus", text: "×1 Mult, plus ×0,1 für jede gekaufte Formstufe" },
    patron: { name: "Mäzen", text: "×1,25 Mult pro ungewöhnlichem Relikt, das du hältst" },
    indelible: {
      name: "Unauslöschlich",
      text: "×1,75 Mult. 1 zu 40, dass es nach jedem Versuch verblasst",
    },
    second_wind: {
      name: "Zweiter Atem",
      text: "Überlebe eine verlorene Runde, wenn du ein Viertel des Ziels erreicht hast. Dann ist er weg",
    },
    carbon_copy: { name: "Durchschlag", text: "Zählt wie das Relikt rechts daneben" },
  },

  boss: {
    silence: {
      name: "Die Stille",
      text: "Falsch platzierte Buchstaben zählen und erscheinen als fehlend. Du erfährst nur, wie viele es sind.",
    },
    fog: {
      name: "Der Nebel",
      text: "Gelb und Grau sehen gleich aus. Sie zählen weiterhin verschieden.",
    },
    tyrant: {
      name: "Der Tyrann",
      text: "Jeder Versuch muss die grünen Buchstaben wiederverwenden, die du gefunden hast.",
    },
    miser: { name: "Der Geizhals", text: "Schon benutzte Buchstaben bringen keine Punkte." },
    clock: { name: "Die Uhr", text: "Nur vier Versuche. Das Ziel ist um ein Viertel niedriger." },
    glutton: { name: "Der Vielfraß", text: "Jeder Versuch braucht mindestens zwei Vokale." },
    auditor: { name: "Der Prüfer", text: "Dein Lösungs-Multiplikator ist bei ×2 gedeckelt." },
    purist: {
      name: "Der Purist",
      text: "Kein Buchstabe darf in einem Versuch zweimal vorkommen.",
    },
    drought: { name: "Die Dürre", text: "Vokale bringen keine Punkte." },
    mirror: {
      name: "Der Spiegel",
      text: "Deine Hinweise erscheinen rückwärts. Sie zählen weiterhin so, wie sie gefallen sind.",
    },
    famine: {
      name: "Die Hungersnot",
      text: "Nur drei Versuche. Das Ziel ist ein Drittel des üblichen.",
    },
    rust: {
      name: "Der Rost",
      text: "Buchstaben-Aufwertungen bringen nichts. Jeder Buchstabe zählt nur seinen Grundwert.",
    },
    margin: { name: "Der Rand", text: "Der erste und der letzte Buchstabe bringen keine Punkte." },
    vandal: { name: "Der Vandale", text: "Buchstaben-Modifikatoren tun nichts." },
    plateau: {
      name: "Das Plateau",
      text: "Effekte, die multiplizieren, tun nichts. Mult kann nur addiert werden.",
    },
  },

  modifier: MODIFIER,

  consumable: {
    oracle: { name: "Das Orakel", text: "Deckt einen Buchstaben der Antwort an seinem Platz auf" },
    hermit: {
      name: "Der Einsiedler",
      text: "Schließt 5 Buchstaben aus, ohne einen Versuch zu kosten",
    },
    magician: {
      name: "Der Magier",
      text: "Für den Rest der Runde zählen graue Felder als gelb.",
    },
    fool: { name: "Der Narr", text: "Zählt deinen vorigen Versuch ein zweites Mal" },
  },

  pack: {
    alphabet: { name: "Alphabet-Booster", text: "Wähle 1 von 3 Buchstaben-Modifikatoren" },
    relic: { name: "Relikt-Booster", text: "Wähle 1 von 3 Relikten" },
    category: {
      name: "Kategorie-Booster",
      text: "Wähle 1 von 3 Wortkategorien zum Aufstufen",
    },
  },

  category: {
    alphabetical: { name: "Alphabetisch", text: "Buchstaben in A–Z-Folge" },
    vowel_heavy: { name: "Vokalreich", text: "Drei oder mehr Vokale" },
    cluster: { name: "Häufung", text: "Drei Konsonanten hintereinander" },
    twinned: { name: "Gedoppelt", text: "Ein Buchstabe kommt zweimal vor" },
    distinct: { name: "Verschieden", text: "Kein Buchstabe wiederholt sich" },
  },

  etching: {
    etch_vowels: {
      name: "Vokale gravieren",
      text: (chips) => `A E I O U bringen +${chips} Punkte`,
    },
    etch_staples: {
      name: "Häufige gravieren",
      text: (chips) => `L N S T R bringen +${chips} Punkte`,
    },
    etch_heavy: {
      name: "Schwere gravieren",
      text: (chips) => `J Q X Z bringen +${chips} Punkte`,
    },
    etch_consonants: {
      name: "Konsonanten gravieren",
      text: (chips) =>
        plural(chips, {
          one: `Jeder Konsonant bringt +${chips} Punkt`,
          other: `Jeder Konsonant bringt +${chips} Punkte`,
        }),
    },
  },

  ascension: {
    1: { name: "Gejagt", text: "Jeder Versuch muss die Buchstaben nutzen, die du gefunden hast." },
    2: { name: "Nur einmal", text: "Kein Wort zweimal in derselben Runde." },
    3: { name: "Steiler", text: "Jedes Ziel liegt 15\u00A0% höher." },
    4: {
      name: "Verankert",
      text: "Jeder Versuch muss die Buchstaben nutzen, die du platziert hast.",
    },
    5: {
      name: "Tyrannei",
      text: "Platzierte Buchstaben müssen dort bleiben, wo du sie platziert hast.",
    },
    6: { name: "Beengt", text: "Vier Relikt-Plätze, nicht fünf." },
    7: { name: "Magere Jahre", text: "Jede Runde zahlt $1 weniger." },
    8: { name: "Totes Gewicht", text: "Eine Runde, die du nicht gelöst hast, zahlt nichts." },
    9: { name: "Kein Echo", text: "Kein Wort zweimal im ganzen Durchlauf." },
    10: {
      name: "Mach es fertig",
      text: "Das Ziel zu erreichen genügt nicht. Du musst das Wort lösen.",
    },
    steeper: {
      name: "Steiler",
      text: (percent, total) => `Ziele steigen um weitere ${percent}\u00A0% (×${total} insgesamt).`,
    },
    endsFirstGuess: "Die Erstversuch-Immunität ist deaktiviert.",
  },

  round: ["Normale Runde", "Elite-Runde", "Boss-Runde"],

  refusal: {
    not_your_turn: "du bist nicht am Zug",
    not_a_letter: "das ist kein Buchstabe",
    letter_broken: ({ letter }) => `${letter.toUpperCase()} ist zerbrochen`,
    no_room: "kein Platz mehr",
    wrong_length: ({ length }) =>
      plural(length, { one: `${length} Buchstabe`, other: `${length} Buchstaben` }),
    not_in_word_list: "nicht in der Wortliste",

    must_use: ({ letter }) => `${letter.toUpperCase()} muss vorkommen`,
    must_keep: ({ letter, position }) =>
      `${letter.toUpperCase()} muss auf Position ${position} bleiben`,
    needs_two_vowels: "mindestens zwei Vokale nötig",
    no_repeated_letters: "kein Buchstabe doppelt",
    already_guessed_round: "in dieser Runde schon versucht",
    already_used_run: "in diesem Durchlauf schon benutzt",

    only_during_round: "nur während einer Runde",
    no_such_card: "diese Karte gibt es nicht",
    word_already_revealed: "das Wort ist schon ganz aufgedeckt",
    nothing_to_reveal: "nichts aufzudecken",
    nothing_to_rule_out: "nichts mehr auszuschließen",
    already_prepared: "schon vorbereitet",
    no_guess_to_repeat: "kein Versuch zum Wiederholen",

    nothing_to_collect: "nichts zu kassieren",
    run_not_won: "der Durchlauf ist nicht gewonnen",

    not_in_shop: "nicht im Laden",
    sell_only_in_shop: "verkauft wird nur im Laden",
    drop_only_in_shop: "Karten wirft man nur im Laden ab",
    finish_pack_first: "beende erst den offenen Booster",
    place_mod_first: "platziere erst den Modifikator",
    already_bought: "schon gekauft",
    not_enough_gold: "nicht genug Gold",
    no_such_relic: "dieses Relikt gibt es nicht",
    no_relic_slots: "kein freier Relikt-Platz",
    no_card_slots: "kein freier Kartenplatz",
    pack_empty: "nichts mehr für diesen Booster",
    no_pack_open: "kein Booster offen",
    already_taken: "schon genommen",
    nothing_to_place: "nichts zu platzieren",
    no_letter_for_mod: "kein Buchstabe mehr dafür",
    mod_not_allowed: ({ id, letter }) =>
      `${MODIFIER[id].name} passt nicht auf ${letter.toUpperCase()}`,

    unknown_card: "unbekannte Karte",
    mod_needs_letter: "der braucht erst einen Buchstaben",
    nested_pack: "aus einem Booster kommt kein Booster",
    unknown_letter: "unbekannter Buchstabe",
    unknown_etching: "unbekannte Gravur",
    unknown_category: "unbekannte Kategorie",
    unknown_range: "unbekannter Bereich",
    unknown_modifier: "unbekannter Modifikator",
    unknown_pack: "unbekannter Booster",
  },

  event: {
    // Gold wears the currency sign rather than a word, the way every other
    // price in the game does.
    growth: ({ amount, unit }) => (unit === "gold" ? `+$${amount}` : `+${amount} ${UNIT[unit]}`),

    payout: (paid) => {
      switch (paid.kind) {
        case "chips":
          return `+${paid.amount}`
        case "mult":
          return `+${paid.amount} Mult`
        case "times":
          return `×${paid.factor} Mult`
        case "blocked":
          return "×1 blockiert"
        case "gold":
          return `+$${paid.amount}`
      }
    },

    categoryLevel: (name, level) => `${name} Stufe ${level}`,
    modPlaced: (name, letter) => `${name} ${letter.toUpperCase()}`,
    note: (note) => {
      switch (note.card) {
        case "oracle":
          return `${note.letter.toUpperCase()} ist Nr. ${note.position}`
        case "hermit":
          return `kein ${note.letters.map((letter) => letter.toUpperCase()).join(" ")}`
        case "magician":
          return "Graue zählen diese Runde als gelb"
        case "fool":
          return `+${note.score}`
      }
    },

    // `falsch platziert` is a predicative participle and does not inflect, so
    // there is nothing for `Intl` to choose between and asking it would only make
    // the badge look like it had a choice. Zero still gets its own word, because
    // `0 falsch platziert` reads as a count that failed to print.
    guessNote: ({ count }) =>
      count === 0 ? "keine falsch platziert" : `${count} falsch platziert`,
  },

  ui: {
    /**
     * The long scale, and the widest ladder of the four: `Mrd` where English
     * writes `B`, because `10^9` is a `Milliarde` and a `Billion` is `10^12`.
     * Three characters a rung against English's one is the reason the HUD's
     * score line is worth looking at in German before any other language.
     */
    units: ["Tsd", "Mio", "Mrd", "Bio", "Brd", "Trl", "Trd", "Qua"],

    error: {
      words: (cause) => `Wortlisten konnten nicht geladen werden: ${cause}`,
    },

    watch: {
      waiting: "Warte auf einen Lauf…",
      lost: "Verbindung zum Host verloren. Neuer Versuch…",
      seed: (seed, ascension) => `Seed ${seed} · Aufstieg ${ascension}`,
      move: (n) => `Zug ${n}`,
      refused: (command, reason) => `Abgelehnt: „${command}“: ${reason}`,
      over: "Lauf beendet",
      version: (host, here) =>
        `Der Host nutzt Inhalt ${host}, dieses Spiel ${here}. Die Züge passen womöglich nicht.`,
      diverged: (n) => `Zug ${n} passt nicht zu diesem Spiel. Hier angehalten.`,
    },

    common: {
      close: "Schließen",
      back: "Zurück",
      play: "Spielen",
      howToPlay: "Spielanleitung",
      codex: "Kodex",
      sound: {
        off: "Ton: aus",
        sound: "Ton: Effekte",
        music: "Ton: Effekte und Musik",
        musicOnly: "Ton: nur Musik",
      },
      ascension: (level) => `Aufstieg ${level}`,
      // DIN 5008 puts a space between the number and the sign, and it is written
      // as an escape here for the same reason French writes its own: a literal
      // non-breaking space is invisible in a diff.
      percent: (share) => `${share}\u00A0%`,
      none: "—",
      loading: "Wörter werden geladen…",
    },

    board: {
      menu: "Menü",
      decor: {
        all: {
          label: "Nur verbesserte Buchstaben zeigen",
          tip: "Der Wert jedes Buchstabens wird gezeigt.\nTippen, um nur verbesserte Buchstaben zu zeigen.",
        },
        minimal: {
          label: "Alle Werte ausblenden",
          tip: "Nur verbesserte Buchstaben werden gezeigt.\nTippen, um alle Werte auszublenden.",
        },
        none: {
          label: "Wert jedes Buchstabens zeigen",
          tip: "Alle Werte sind ausgeblendet.\nTippen, um den Wert jedes Buchstabens zu zeigen.",
        },
      },
      stage: (stage, total) => `Etappe ${stage}/${total}`,
      stageEndless: (stage) => `Etappe ${stage} ∞`,
      ascensionTag: (level) => `A${level}`,
      target: (target) => `von ${target}`,
      relicTip: (text, detail) => `${text} (${detail})`,
      relicLabel: (name, text) => `${name}: ${text}`,
      relicLabelGrown: (name, text, detail) => `${name}: ${text} (${detail})`,
      // A German keyboard's own two keys. `Entf` is the forward-delete key and
      // this one deletes backwards, so it is spelled out instead.
      enter: "ENTER",
      del: "LÖSCH",
      solveFactor: (factor) => `lösen ×${factor}`,
      solveFloor: (score) => `→ ${score}`,
      solveFloorClears: (score) => `→ ${score}, geschafft`,
      shapeLevel: (level) => `Stufe ${level}`,
      multUnknown: "Die Farbe ist der Mult, und nur Raten deckt sie auf.",
      letterBroken: (letter) => `${letter.toUpperCase()} zerbrochen`,
      relicGone: (name) => `${name} ist weg`,
    },

    tip: {
      tileChips: (letter, chips) =>
        chips === 0
          ? `${letter.toUpperCase()} · keine Punkte`
          : plural(chips, {
              one: `${letter.toUpperCase()} · +${chips} Punkt`,
              other: `${letter.toUpperCase()} · +${chips} Punkte`,
            }),
      keyChips: (letter, chips) =>
        chips === 0
          ? `${letter.toUpperCase()} · keine Punkte`
          : plural(chips, {
              one: `${letter.toUpperCase()} · ${chips} Punkt`,
              other: `${letter.toUpperCase()} · ${chips} Punkte`,
            }),
      broken: (letter) => `${letter.toUpperCase()} · zerbrochen, nicht mehr tippbar`,
      base: (chips) => `${chips} Grundwert`,
      etched: (chips) => `+${chips} graviert`,
      fromRange: (chips, range, level) => `+${chips} aus ${range} Stufe ${level}`,
      boss: (name, text) => `${name}: ${text}`,
      color: (color, mult) =>
        mult === 0 ? `${COLOR[color]} · kein Mult` : `${COLOR[color]} · +${mult} Mult`,
      mod: (name, badge) => `${name} · ${badge}`,
      modIdle: (name, text) => `${name} · ${text}`,
      modSilenced: (name, text) => `${name} · ${text} · diese Runde stumm`,
      modQuiet: (name, text) => `${name} · ${text} · diesmal nichts`,
      relic: (name, badge) => `${name} · ${badge}`,
      share: (chips, total) => `${chips} von ${total} Punkten · kein Mult`,
      shareWithMult: (chips, total, mult, multTotal) =>
        `${chips} von ${total} Punkten · ${mult} von ${multTotal} Mult`,
    },

    intro: {
      stage: (stage, total) => `Etappe ${stage} von ${total}`,
      stageEndless: (stage) => `Etappe ${stage} · endlos`,
      scoreAtLeast: "Erreiche mindestens",
      guesses: (guesses) => plural(guesses, { one: "Versuch", other: "Versuche" }),
      reward: "Belohnung",
      track: "Diese Etappe",
      trackRound: ["Normal", "Elite", "Boss"],
      cleared: "Geschafft",
      current: "Diese Runde",
      upNext: "Als Nächstes",
      targets: (factor) => `Ziele ×${factor}`,
      coachYes: "Tutorial spielen",
      coachNo: "Tutorial überspringen",
      play: "Spielen",
    },

    reward: {
      cleared: "Runde geschafft",
      answerWas: (word) => `Das Wort war ${word.toUpperCase()}`,
      score: (score, target) => `${score} von ${target}`,
      unusedGuesses: "Ungenutzte Versuche",
      interest: "Zinsen",
      relics: "Relikte",
      savedBy: (name) => `Gerettet von ${name}`,
      firstGuess: "Im ersten Versuch gelöst",
      total: "Gesamt",
      collect: "Kassieren",
    },

    shop: {
      title: "Laden",
      sold: "verkauft",
      sell: (amount) => `für ${amount} verkaufen`,
      drop: "abwerfen",
      reroll: "Neu auslegen",
      nextRound: "Nächste Runde",
      short: (amount) => `Dir fehlen ${amount}.`,
      shapesLabel: "Wortformen",
      shapesLevel: (name, level) => `${name} Stufe ${level}`,
      shapesNone: "alle auf Stufe 1",

      tagPack: "Booster",
      tipPack: (picks) =>
        `Er legt seine Karten offen aus und du behältst ${picks > 1 ? picks : "eine"} davon, kostenlos.`,
      tagRelic: "Relikt",
      tipRelic: "Du behältst es den ganzen Durchlauf, und es wirkt in jeder Runde von allein.",
      tagConsumable: "Verbrauchskarte",
      slotsFull: "Kein Platz frei",
      tipConsumable: "Du benutzt sie einmal, wann du willst, dann ist sie weg.",
      tagLetter: "Buchstabe",
      tipMod: "Er klebt für den Rest des Durchlaufs an einem Buchstaben.",
      tagRange: "Alphabet",
      tipRange:
        "Es stuft einen Abschnitt des Alphabets auf, sodass jeder Buchstabe darin mehr wert ist.",
      tagShape: "Wortform",
      tipShape: "Es stuft eine Wortform auf, sodass jeder Versuch dieser Form mehr zahlt.",
      tagEtching: "Gravur",
      tipEtching:
        "Sie gibt einer Gruppe von Buchstaben dauerhaft Punkte, und Nachkaufen summiert sich.",

      modAnyTitle: (name) => `${name} · beliebiger Buchstabe`,
      modAnyText: (text) => `Wähle einen beliebigen Buchstaben. Er ${text}`,
      modAnyTextOnly: (text, letters) =>
        `Wähle einen beliebigen Buchstaben. Er ${text}, aus ${letters}`,
      modTitle: (name, letter) => `${name} ${letter}`,
      modText: (letter, text) => `${letter} ${text}`,
      swap: (name, pip) => `Ersetzt ${name} ${pip}`,

      level: (level) => `Stufe ${level}`,
      rangeText: (letters, chips) => `${letters} bringen +${chips} Punkte pro Stufe`,
      levelText: (name, chips, mult, times) =>
        `${name}-Wörter bringen +${chips} Punkte, +${mult} Mult und ${factor(times)} Mult pro Stufe`,
      fallbackRange: "Bereich",
      fallbackLevel: "Stufe",
      fallbackEtching: "Gravur",
    },

    pack: {
      // A numeral, as on the pack's own card, and it also settles the gender:
      // "eine" was feminine under packs dealing Modifikatoren and Relikte.
      choose: (left) => `Wähle 1 von ${left}`,
      choosePicks: (picks, left) => `Wähle ${picks} von ${left}`,
      taken: "genommen",
      skip: "Überspringen",
      skipSome: "Nichts weiter nehmen",
    },

    place: {
      choose: (text) => `Wähle einen Buchstaben. Er ${text}.`,
      onlyOn: (name, letters) => `${name} passt nur auf ${letters}.`,
      oneEach:
        "Ein Buchstabe trägt nur einen Modifikator. Wer einen schon markierten antippt, wird vorher gefragt.",
      carrying: (letter) => `${letter.toUpperCase()} trägt`,
      loses: (name) => `${name} hier zu setzen, verliert ihn für den Rest des Durchlaufs.`,
      replace: (name) => `${name} ersetzen`,
      keep: (name) => `${name} behalten`,
    },

    end: {
      won: "Durchlauf gewonnen",
      lost: "Durchlauf verloren",
      short: (score, target, by) => `${score} von ${target}, ${by} zu wenig`,
      wonAndWent: (stages) => `Etappe ${stages} geschafft, und du bist weitergegangen`,
      reached: (stage, round) => `Etappe ${stage} erreicht, ${round}`,
      endlessNote: (stages) =>
        `Bei Etappe ${stages} hört das Spiel auf, nicht der Durchlauf. Der Sieg gehört dir so ` +
        "oder so. Weiterzumachen fragt nur, wie weit dieser Aufbau wirklich trägt, und die " +
        "Ziele wachsen dabei im selben Takt weiter.",
      endless: "Endlos weiterspielen",
      mainMenu: "Hauptmenü",
      newRun: "Neuer Durchlauf",
      firstEarned: "Aufstieg 1 ist freigespielt, und der Durchlauf lässt sich härter stellen",
      earned: (level, next) => `Aufstieg ${level} geschafft, und ${next} ist freigespielt`,
      topOfLadder: (level) => `Aufstieg ${level} geschafft. Darüber gibt es nichts mehr.`,
    },

    title: {
      name: "5 WILD",
      tagline: "Ein Wortrate-Roguelike",
      runs: (count) => plural(count, { one: `${count} Durchlauf`, other: `${count} Durchläufe` }),
      wins: (count) => plural(count, { one: `${count} Sieg`, other: `${count} Siege` }),
      bestStage: (stage) => `beste Etappe ${stage}`,
    },

    stats: {
      title: "Rekorde",
      runs: "Durchläufe",
      wins: (count) => plural(count, { one: "Sieg", other: "Siege" }),
      bestStage: "beste Etappe",
      guesses: "Versuche",
      solved: "gelöst",
      meanSolve: "Schnitt zum Lösen",
      cracked: (count) =>
        plural(count, { one: `${num(count)} Wort`, other: `${num(count)} Wörter` }),
      // `geknackt` is a predicative participle and invariable, so German takes
      // the count and spends it the way English does: the agreement it needs is
      // already spent on `Wort` / `Wörter` in the bold half above.
      crackedBare: (_count) => "geknackt",
      crackedOf: (_count, pool) => `geknackt, von ${pool}`,
      played: (count) =>
        plural(count, { one: `${num(count)} Wort`, other: `${num(count)} Wörter` }),
      // `gespielt` is invariable for the same reason `geknackt` is, so the count
      // goes unspent here and is spent on `Wort` / `Wörter` above.
      playedBare: (_count) => "gespielt",
      playedOf: (_count, pool) => `gespielt, von ${pool}`,
      mostPlayed: "Am meisten gespielt:",
      // `-mal` is a suffix, not a countable noun, so it does not inflect and
      // there is nothing for `Intl` to select between.
      times: (count) => `${num(count)}-mal`,
      breakdown: "Wie die Antworten fallen",
      solvedIn: (guesses) => `Gelöst in ${guesses}`,
      neverFound: "Nie gefunden",
      favoriteRelics: "Lieblingsrelikte",
      taken: (count) => `${num(count)}× genommen`,
      noStreak: "Noch keine Antwort geknackt.",
      streakBest: (now) => `${now} in Folge, die längste Serie bisher, und sie läuft noch.`,
      streakWithNow: (best, now) => `Beste Serie: ${best} in Folge, ${now} laufen gerade.`,
      streak: (best) => `Beste Serie: ${best} in Folge.`,
    },

    ladder: {
      carrot: "Schlag das, um Aufstieg 1 freizuspielen",
      base: "Standard",
      baseText: "Keine Zusatzregeln. Jede Stufe darüber fügt eine hinzu.",
      lower: "Aufstieg senken",
      raise: "Aufstieg erhöhen",
      locked: "Gesperrt",
      skipTo: (level) => `Zu Aufstieg ${level} springen`,
      andBelow: "Und alle Regeln darunter, ebenfalls.",
      askTitle: (level) => `Zu Aufstieg ${level} springen?`,
      ruleLabel: (name) => `${name}:`,
      askAndBelow: "Dazu alle Regeln darunter.",
      intended: "Auf der Stufe darunter zu gewinnen, ist der vorgesehene Weg nach oben.",
      skipAnyway: (level) => `Trotzdem zu ${level} springen`,
    },

    help: {
      title: "Spielanleitung",
      lead:
        "Rate das Wort, so wie du es schon kennst: Grün ist der richtige Buchstabe am richtigen " +
        "Platz, Gelb der richtige Buchstabe woanders.",
      scored: "Der Unterschied ist, dass jeder Versuch gewertet wird.",
      chipsMult: {
        term: "Punkte × Mult",
        text: "Jeder Versuch ist seine Punkte mal seinen Mult wert.",
      },
      letters: {
        term: "Buchstaben zahlen Punkte",
        text:
          "Seltene Buchstaben zahlen mehr. Der Laden verkauft zwei Wege, sie aufzuwerten: " +
          "Gravuren, die einer Sorte Buchstaben etwas geben, und Stufen auf einem Abschnitt des " +
          "Alphabets. Jeder Buchstabe liegt in genau einem Abschnitt, und beide summieren sich.",
      },
      colors: {
        term: "Farben zahlen Mult",
        text:
          "Grün bringt +3 Mult, Gelb +1, Grau nichts. Ein Versuch voller Grau ist fast nichts " +
          "wert, sodass ein hingeworfener Sondierversuch dich wirklich Punkte kostet.",
      },
      solving: {
        term: "Lösen multipliziert die Runde",
        text:
          "Finde das Wort, und der ganze in der Runde angesammelte Haufen (nicht nur der " +
          "Versuch, der gelöst hat) wird mit 1 plus deinen übrigen Versuchen multipliziert. " +
          "Dann endet die Runde sofort, ob das Ziel erreicht ist oder nicht.",
      },
      farming:
        "Das ist das ganze Spiel: Jeder Versuch, den du ins Sammeln steckst, macht den Haufen " +
        "größer und den Multiplikator kleiner, der auf ihn wartet.",
      solveLine: {
        term: "Achte also auf den Balken",
        text:
          "Hinter dem Balken der Punktzahl läuft ein blasserer voraus: wo der Haufen stünde, " +
          "wenn du beim nächsten Versuch löst. Reicht er bis ans Ende, gewinnt Lösen die " +
          "Runde. Halte die Punktzahl gedrückt, um den Multiplikator zu sehen.",
      },
      runHeading: "Der Durchlauf",
      target: {
        term: "Schlag das Ziel",
        text: (stages, rounds, authored) =>
          `${stages} Etappen zu je ${rounds} Runden. Verfehle das Ziel einer Runde, und der ` +
          "Durchlauf ist vorbei. Das ist die einzige Art zu verlieren, mit einer Ausnahme, " +
          "der Erstversuch-Immunität: Findest du das Wort mit dem ersten Versuch, ist die Runde geschafft, egal wie viele " +
          `Punkte sie gebracht hat, bis Aufstieg ${authored} und Etappe ${stages}.`,
      },
      endless: {
        term: "Dann mach weiter, wenn du dich traust",
        text: (stages) =>
          `Etappe ${stages} zu schaffen gewinnt den Durchlauf, und du kannst dort aufhören oder ` +
          "in Etappen weiterziehen, die niemand ausbalanciert hat. Die Ziele wachsen im selben " +
          "Takt, und dort zu sterben nimmt dir den Sieg nicht wieder weg.",
      },
      bosses: {
        term: "Bosse",
        text: "Jede dritte Runde verbiegt eine Regel. Lies sie, bevor du spielst.",
      },
      ascensions: {
        term: "Aufstiege",
        text: (authored) =>
          "Der Schwierigkeitsregler auf dem Titelbildschirm, und die Zahl, die sich zu " +
          `vergleichen lohnt. Die ersten ${authored} Stufen fügen je eine dauerhafte Regel ` +
          "hinzu: darauf, was du raten darfst, was eine Runde zahlt, wie hoch die Ziele liegen, " +
          "wie viele Relikte du behältst, wie viele Versuche du hast. Darüber hört die Leiter " +
          "nicht auf: Jede weitere Stufe hebt einfach alle Ziele noch einmal an. Auf einer zu " +
          "gewinnen schaltet die nächste frei, und Sprosse für Sprosse zu steigen ist der " +
          "vorgesehene Weg, kein Schloss.",
      },
      money: {
        term: "Geld",
        text: (payouts, perGuess, per, cap) =>
          `Runden zahlen ${payouts}, dazu ${perGuess} pro ungenutztem Versuch, dazu $1 Zinsen ` +
          `je ${per}, die du hältst, bis ${cap}. Auf seinem Geld sitzen zu bleiben ist eine ` +
          "Strategie.",
      },
      relics: {
        term: "Relikte",
        text: (slots) =>
          `Bis zu ${slots}, und sie wirken von links nach rechts, sodass die Kaufreihenfolge ` +
          "zählt. Tippe eines an, um es zu lesen.",
      },
      packs: {
        term: "Booster",
        text:
          "Ein Platz verkauft eine Auswahl statt einer Karte. Ein Booster legt drei aus und du " +
          "behältst eine, kostenlos. Der Rest des Ladens wartet, bis du gewählt hast oder " +
          "gegangen bist, und Gehen behält nichts.",
      },
      mods: {
        term: "Buchstaben-Modifikatoren",
        text:
          "Der Laden verkauft Modifikatoren und du wählst, an welchem Buchstaben jeder für den " +
          "Rest des Durchlaufs klebt. Jedes Mal, wenn du diesen Buchstaben spielst, tut er das. " +
          "Ein ×Mult-Buchstabe multipliziert, was das Wort bis zu seiner Stelle gebracht hat, " +
          "sodass er spät im Wort mehr wert ist als früh. Booster geben sie billiger aus, mit " +
          "schon gewähltem Buchstaben.",
      },
      tutorial:
        "Neu hier? Wir empfehlen das Tutorial: Es spielt deine erste Runde mit dir und " +
        "erklärt die Wertung, während sie passiert.",
      tutorialStart: "Tutorial starten",
      codexNote:
        "Der Kodex enthält jedes Relikt, jeden Boss, jede Wortform und jeden Modifikator im Spiel, vollständig.",
      openCodex: "Kodex öffnen",
      gotIt: "Verstanden",
    },

    shapes: {
      title: "Wortformen",
      scoresAs: (word, shape) => `${word.toUpperCase()} zählt als ${shape}`,
      anyWord: "Jeder Versuch hat eine Form.",
      note:
        "Ein Versuch zählt als die seltenste Form, die er erfüllt, also als die erste in dieser " +
        "Liste, die er erfüllt. Eine Form aufzustufen hebt alle künftigen Versuche dieser Form. " +
        "Stufe 1 zahlt nichts, sodass erst eine Stufe eine Form lohnend macht. Der Laden " +
        "bietet eine Form umso öfter an, je höher sie schon gestuft ist.",
      scoring: "zählt als",
      alsoMatches: "erfüllt außerdem",
      payNow: (chips, mult, times) =>
        `jetzt +${chips} Punkte, +${mult} Mult, ${factor(times)} Mult`,
      payPerLevel: (chips, mult, times) =>
        `+${chips} Punkte, +${mult} Mult, ${factor(times)} Mult pro Stufe`,
    },

    codex: {
      title: "Kodex",
      lead: "Alles, was im Spiel ist, ob du ihm schon begegnet bist oder nicht.",
      relics: {
        title: "Relikte",
        blurb: (slots) =>
          `Bis zu ${slots} auf einmal, wirkend von links nach rechts, sodass die Kaufreihenfolge ` +
          "Teil des Aufbaus ist.",
      },
      bosses: {
        title: "Bosse",
        blurb:
          "Jede dritte Runde. Jede Gruppe wird ohne Zurücklegen gezogen, sodass ein Durchlauf " +
          "denselben Boss nie zweimal trifft.",
      },
      ascensions: {
        title: "Aufstiege",
        blurb:
          "Die Schwierigkeit des Durchlaufs selbst, vor seinem Beginn gewählt und für seine " +
          "ganze Länge fest. Ein Durchlauf auf einer Stufe spielt alle Regeln bis zu ihr, und " +
          "auf einer zu gewinnen schaltet die nächste frei. Hier stehen die geschriebenen; " +
          "hinter der letzten hebt jede Stufe einfach alle Ziele um weitere 8\u00A0%, und eine letzte " +
          "Stufe gibt es nicht.",
      },
      shapes: {
        title: "Wortformen",
        blurb:
          "Jeder Versuch zählt als genau eine Form: als die seltenste, die er erfüllt, also als " +
          "die erste in dieser Liste. Eine Form aufzustufen hebt alle künftigen Versuche dieser " +
          "Form.",
      },
      mods: {
        title: "Buchstaben-Modifikatoren",
        blurb:
          "Für den Rest des Durchlaufs an einen einzigen Buchstaben geklebt. Nur einer je " +
          "Buchstabe, und die Tastatur trägt das Zeichen. Ein ×Mult-Buchstabe multipliziert, " +
          "was das Wort bis zu seiner Stelle gebracht hat, sodass er spät im Wort mehr wert ist " +
          "als früh. Der Ladenpreis kauft die Karte und lässt dich den Buchstaben wählen; ein " +
          "Booster gibt sie zum genannten Preis aus, mit schon gewähltem Buchstaben.",
      },
      upgrades: {
        title: "Buchstaben-Aufwertungen",
        blurb:
          "Zwei Wege, die beide Buchstaben Punkte geben und sich summieren: Gravuren heben eine " +
          "Sorte Buchstaben, Bereiche heben einen Abschnitt des Alphabets. Jeder Buchstabe " +
          "liegt in genau einem Abschnitt.",
      },
      consumables: {
        title: "Verbrauchskarten",
        blurb: (slots) => `Einmal benutzt, wann du willst. Du kannst ${slots} halten.`,
      },
      packs: {
        title: "Booster",
        blurb:
          "Auf einem eigenen Platz verkauft. Ein Booster legt seine Karten aus und du behältst " +
          "eine, kostenlos. Der Laden wartet, bis du gewählt hast oder gegangen bist.",
      },
      rarity: {
        common: "gewöhnlich",
        uncommon: "ungewöhnlich",
        rare: "selten",
        legendary: "legendär",
      },
      tier: { early: "Früh", mid: "Mitte", late: "Spät" },
      tierBand: (tier, first, last) => `${tier} · Etappen ${first}–${last}`,
      shapePer: (chips, mult, times) => `+${chips} / +${mult} / ${factor(times)} pro Stufe`,
      modName: (name, pip) => `${name} ${pip}`,
      modText: (text) => `Der Buchstabe ${text}.`,
      modTextOnly: (text, letters) => `Der Buchstabe ${text}. Nur auf ${letters}.`,
      packText: (text) => `${text}.`,
      packTextPicks: (text, picks) => `${text}, und behalte ${picks}.`,
    },

    pause: {
      title: "Pause",
      sound: "Ton",
      music: "Musik",
      track: "Stück",
      speed: "Animationstempo",
      skin: "Design",
      skins: {
        smoke: "Rauchsalon",
        tabletop: "Spielbrett",
        "classic-dark": "Klassisch dunkel",
        "classic-light": "Klassisch hell",
      },
      language: "Sprache",
      wordsNextRun: "Die Wörter wechseln beim nächsten Durchlauf.",
      quit: "Durchlauf aufgeben",
      resume: "Weiterspielen",
    },

    about: {
      title: "Über das Spiel",
      sharing: "Anonyme Spieldaten teilen",
      sharingNote:
        "Hilf, das Spiel zu verbessern, indem du deine Ergebnisse teilst. " +
        "Persönliche Daten sind nie dabei.",
      privacy: {
        before: "Was genau erhoben wird, steht in der ",
        link: "Datenschutzerklärung",
        after: ".",
      },
      thanks: "Danke, dass du hilfst, das Spiel zu verbessern!",
      credits: "Mitwirkende",
      source: "Quellcode",
      report: "Problem melden",
    },

    credits: {
      title: "Mitwirkende",
      madeBy: "Von",
      music: "Musik",
      musicText: (piece) => ({ before: `„${piece}“ von `, after: "." }),
      sounds: "Klänge",
      soundsText: (people) => `Kenney, und ${people} auf Freesound. Alles CC0.`,
      licence: "Lizenz",
      licenceText: "Freie Software unter der GNU GPL, Version 3.",
      font: "Schrift",
      fontText: "IBM Plex und Jost, unter der SIL Open Font License 1.1.",
      back: "Zurück",
    },

    quit: {
      title: "Diesen Durchlauf aufgeben?",
      body: (stage, total, round) =>
        `Du bist auf Etappe ${stage} von ${total}, ${round}. Aufgeben löscht ihn. ` +
        "Es gibt keinen Weg zu diesem Durchlauf zurück.",
      bodyEndless: (stage, round) =>
        `Du bist auf Etappe ${stage}, ${round}. Aufgeben löscht ihn. ` +
        "Es gibt keinen Weg zu diesem Durchlauf zurück.",
      confirm: "Durchlauf aufgeben",
      cancel: "Weiterspielen",
    },

    seed: {
      title: "Durchlauf mit Seed",
      open: "Version und Durchläufe mit Seed",
      blurb:
        "Jeder Durchlauf wird aus einem Seed gegeben. Ein Code spielt auf deiner Stufe " +
        "in deiner Sprache; füge einen geteilten Link ein, um dieselben Wörter, Bosse " +
        "und Läden wie sein Durchlauf zu bekommen.",
      field: "Seed-Code",
      play: "Diesen Seed spielen",
      abandons: "Das beendet den laufenden Durchlauf.",
      badCode: "Das ist kein Seed-Code: sieben Buchstaben und Ziffern, beginnend mit 0 oder 1.",
      words: (language) => `Gegeben aus der Wortliste: ${language}.`,
      line: (code) => `Seed ${code}`,
      copy: "Einen Link zu diesem Durchlauf kopieren",
      copied: "Link kopiert",
    },

    coach: {
      chips:
        "Jeder Versuch wird gewertet. Tippe ein Wort, und die Zeile über dem Brett addiert, was seine Buchstaben wert sind.",
      rare: (chips) =>
        `${chips} Punkte bisher. Seltene Buchstaben zahlen mehr: A bringt 1, K bringt 5, Z bringt 10.`,
      mult: "Das ? ist der Mult. Jedes gelbe Feld erhöht ihn um 1, jedes grüne um 3. Wortformen und Relikte erhöhen ihn weiter. Drück ENTER, um den Mult dieses Versuchs zu sehen.",
      banked: (chips, mult, score, target) =>
        `${chips} × ${mult} = ${score}, angesammelt Richtung ${target}. Jeder Versuch kommt auf denselben Haufen.`,
      solve: (score, target, now, floor) =>
        `Der volle Balken ist deine Punktzahl: ${score} von ${target}. Lösen multipliziert deine ` +
        `Endpunktzahl mit deinen übrigen Versuchen. Jetzt zu lösen würde sie mit ${now} multiplizieren: ${score} × ${now} = ${floor}. Ist der blasse Balken voll, gewinnt Lösen die Runde.`,
      shape: (word, shape) =>
        `${word.toUpperCase()} zählt als ${shape}. Jeder Versuch hat eine Form, und eine Form im ` +
        "Laden aufzuwerten lässt solche Versuche mehr punkten. Tippe, um alle zu sehen.",
      menu: "Im Menü stellst du Ton, Musik und Design ein. Probier ein paar Designs aus: Manche haben mehr Effekte, andere kaum Bewegung. Dort findest du auch die Spielanleitung und den Kodex.",
      decor:
        "Dieser Knopf hat drei Modi für Tastatur und Kacheln: der Wert jedes Buchstabens, nur verbesserte Buchstaben oder gar nichts. Wenn spätere Runden ablenken, hilft er dir, dich aufs Wort zu konzentrieren.",
      shelf:
        "Gib hier zwischen den Runden dein Gold aus. Relikte bleiben bei dir und punkten bei jedem Versuch. " +
        "Verbrauchskarten wirken einmal. Der Rest lässt Buchstaben oder Wortformen mehr punkten. " +
        "Booster kosten weniger, aber was drin ist, siehst du erst nach dem Kauf.",
      relics: (slots) =>
        `Deine Relikte liegen hier oben. Du hast ${slots} Plätze, und Antippen verkauft eins zum halben Preis.`,
    },
  },
}
