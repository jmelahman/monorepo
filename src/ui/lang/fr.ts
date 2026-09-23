import type { Color, Growth } from "../../engine"
import { formatNumber as num, pluralizer } from "../format"
import type { Strings } from "./types"

const plural = pluralizer("fr")

/** Read by one refusal, which has to name the card it is turning down. */
const MODIFIER: Strings["modifier"] = {
  chip: { name: "Jeton", text: "rapporte +20 jetons" },
  mult: { name: "Mult", text: "rapporte +8 mult" },
  gold: { name: "Or", text: "rapporte $2 à chaque fois que vous la jouez" },
  wild: {
    name: "Joker",
    text: "rapporte +24 mult sur une grise, +14 sur une jaune, +4 sur une verte",
  },
  lucky: { name: "Chanceuse", text: "a 1 chance sur 4 de rapporter +20 mult" },
  echo: { name: "Écho", text: "rapporte +60 jetons quand le mot la répète" },
  anchor: { name: "Ancre", text: "rapporte +125 jetons quand elle tombe verte" },
  steel: { name: "Acier", text: "rapporte ×2 mult" },
  glass: { name: "Verre", text: "rapporte ×3 mult, et peut se briser quand elle tombe grise" },
}

const UNIT: Record<Growth["unit"], string> = { chips: "jetons", mult: "mult" }

const COLOR: Record<Color, string> = { green: "vert", yellow: "jaune", gray: "gris" }

/**
 * French. `en.ts` is the reference; this is one filling of the same shape.
 *
 * Two conventions run through the whole file.
 *
 * `\u00A0` is written as an escape, never as a literal space, wherever French
 * typography puts a space before a double punctuation mark — `:`, `;`, `?` — or
 * between a number and its `%`. The character itself is invisible in a diff and
 * indistinguishable from an ordinary space in an editor, so the one thing nobody
 * could review is whether the file has the right one. Nor is it a typographic
 * nicety: the HUD is narrow enough that a score wrapping away from its `%` is a
 * real outcome. Written as an escape, it is a thing a reviewer can see.
 *
 * The vocabulary: a *chip* is a `jeton`, a *guess* is an `essai` because it is
 * one of six things you spend, a *run* is a `partie`, a *stage* is an `étape`
 * and a *round* is a `manche` — the two nested units of time, which have to stay
 * distinguishable in every sentence that names both. A *pack* is a `booster`,
 * which is what French card players call one; `pochette` would be a stationery
 * item. `Le Palier` is the boss English calls The Plateau, renamed because
 * `plateau` is already the word for the board it is played on.
 */
export const fr: Strings = {
  tag: "fr",
  endonym: "Français",

  relic: {
    green_thumb: { name: "Main Verte", text: "+8 jetons par case verte" },
    scavenger: { name: "Charognard", text: "+$1 par case jaune" },
    vowel_hoarder: { name: "Amasseur de Voyelles", text: "+4 mult par voyelle" },
    slow_burn: {
      name: "Combustion Lente",
      text: "+5 mult pour chaque essai déjà fait dans la manche",
    },
    consonant_cluster: {
      name: "Groupe Consonantique",
      text: "×1.5 mult si le mot a 3+ consonnes de suite",
    },
    cold_open: { name: "Départ à Froid", text: "+30 jetons au premier essai d'une manche" },
    bloodhound: { name: "Limier", text: "+6 jetons par case jaune" },
    head_start: { name: "Avance", text: "+15 mult si le mot commence par une voyelle" },
    loaded_dice: { name: "Dés Pipés", text: "+0 à +20 mult, relancés à chaque essai" },
    anagrammer: { name: "Anagrammiste", text: "×2 mult si aucune lettre ne se répète" },
    keystone: { name: "Clé de Voûte", text: "×3 mult si la case centrale est verte" },
    lexicographer: {
      name: "Lexicographe",
      text: "+3 jetons par lettre différente de vos essais précédents dans la manche",
    },
    sunk_cost: { name: "Coût Irrécupérable", text: "+10 mult par essai qu'il vous resterait" },
    speedrunner: { name: "Sprinteur", text: "×3 mult si vous résolvez en 3 essais ou moins" },
    qs_bargain: { name: "L'Affaire du Q", text: "J, Q, X et Z rapportent le triple de jetons" },
    greedy_grammarian: { name: "Grammairien Cupide", text: "+15 jetons par case grise" },
    doppelganger: {
      name: "Doppelgänger",
      text: "Les lettres répétées rapportent leurs jetons deux fois",
    },
    hot_streak: {
      name: "Bonne Série",
      text: "Gagne +30 jetons définitifs par manche réussie en 3 essais ou moins",
    },
    hoarder: {
      name: "L'Amasseur",
      text: "Gagne +40 jetons définitifs si vous arrivez en boutique avec les deux emplacements de carte pleins",
    },
    masochist: { name: "Masochiste", text: "+8 mult par case grise" },
    chorus: { name: "Le Chœur", text: "×3 mult si le mot contient trois voyelles ou plus" },
    alphabetist: {
      name: "Alphabétiste",
      text: "×2 mult si vos lettres sont dans l'ordre alphabétique",
    },
    vault: { name: "Le Coffre", text: "+25 jetons pour chaque essai déjà fait dans la manche" },
    mint: {
      name: "La Monnaie",
      text: "+3 mult par $5 que vous détenez. Vous ne gagnez aucun intérêt.",
    },
    scorched_earth: {
      name: "Terre Brûlée",
      text: "+12 mult par lettre brisée hors de l'alphabet",
    },
    snowball: {
      name: "Boule de Neige",
      text: "Gagne +1 mult définitif pour chaque case verte que vous jouez",
    },
    long_game: { name: "Le Long Terme", text: "+1 à votre multiplicateur de résolution" },
    pyromaniac: {
      name: "Pyromane",
      text: "+40 mult. Brise une lettre au hasard hors de l'alphabet à chaque manche",
    },
  },

  boss: {
    silence: {
      name: "Le Silence",
      text: "Les lettres mal placées comptent et s'affichent comme absentes. On vous dit seulement combien il y en a.",
    },
    fog: {
      name: "Le Brouillard",
      text: "Le jaune et le gris sont identiques à l'œil. Ils comptent toujours différemment.",
    },
    tyrant: {
      name: "Le Tyran",
      text: "Chaque essai doit réutiliser les lettres vertes que vous avez trouvées.",
    },
    miser: { name: "L'Avare", text: "Les lettres déjà utilisées ne rapportent aucun jeton." },
    clock: { name: "L'Horloge", text: "Quatre essais seulement." },
    glutton: { name: "Le Glouton", text: "Chaque essai doit contenir au moins deux voyelles." },
    auditor: { name: "L'Auditeur", text: "Votre multiplicateur de résolution est plafonné à ×2." },
    purist: {
      name: "Le Puriste",
      text: "Aucune lettre ne peut apparaître deux fois dans un essai.",
    },
    drought: { name: "La Sécheresse", text: "Les voyelles ne rapportent aucun jeton." },
    mirror: {
      name: "Le Miroir",
      text: "Vos indices sont affichés à l'envers. Ils comptent toujours comme ils sont tombés.",
    },
    famine: { name: "La Famine", text: "Trois essais seulement." },
    rust: {
      name: "La Rouille",
      text: "Les améliorations de lettre ne rapportent rien. Chaque lettre ne vaut que sa valeur de départ.",
    },
    margin: {
      name: "La Marge",
      text: "La première et la dernière lettre ne rapportent aucun jeton.",
    },
    vandal: { name: "Le Vandale", text: "Les modificateurs de lettre ne font rien." },
    plateau: {
      name: "Le Palier",
      text: "Les effets qui multiplient ne font rien. Le mult ne peut être qu'ajouté.",
    },
  },

  modifier: MODIFIER,

  consumable: {
    oracle: { name: "L'Oracle", text: "Révèle une lettre de la réponse, à sa place" },
    hermit: {
      name: "L'Ermite",
      text: "Écarte une lettre de la réponse sans dépenser d'essai",
    },
    magician: {
      name: "Le Magicien",
      text: "Votre prochain essai compte sa première case grise comme jaune. Cela vaut du mult, pas un indice.",
    },
    fool: { name: "Le Fou", text: "Compte votre essai précédent une seconde fois" },
  },

  pack: {
    alphabet: {
      name: "Booster Alphabet",
      text: "Choisissez un modificateur de lettre parmi trois",
    },
    relic: { name: "Booster Reliques", text: "Choisissez une relique parmi trois" },
    category: {
      name: "Booster Catégories",
      text: "Choisissez une catégorie de mot à monter parmi trois",
    },
  },

  category: {
    alphabetical: { name: "Alphabétique", text: "Ses lettres ne reculent jamais" },
    vowel_heavy: { name: "Riche en Voyelles", text: "Trois voyelles ou plus" },
    cluster: { name: "Groupe", text: "Trois consonnes de suite" },
    twinned: { name: "Jumelée", text: "Une lettre apparaît deux fois" },
    distinct: { name: "Distincte", text: "Aucune lettre ne se répète" },
  },

  etching: {
    etch_vowels: {
      name: "Graver les Voyelles",
      text: (chips) => `A E I O U valent +${chips} jetons`,
    },
    etch_staples: {
      name: "Graver les Courantes",
      text: (chips) => `L N S T R valent +${chips} jetons`,
    },
    etch_heavy: {
      name: "Graver les Lourdes",
      text: (chips) => `J Q X Z valent +${chips} jetons`,
    },
    etch_consonants: {
      name: "Graver les Consonnes",
      text: (chips) =>
        plural(chips, {
          one: `Chaque consonne vaut +${chips} jeton`,
          other: `Chaque consonne vaut +${chips} jetons`,
        }),
    },
  },

  ascension: {
    1: { name: "Traqué", text: "Chaque essai doit utiliser les lettres que vous avez trouvées." },
    2: { name: "Une Seule Fois", text: "Aucun mot deux fois dans la même manche." },
    3: { name: "Plus Raide", text: "Chaque objectif est 15\u00A0% plus haut." },
    4: { name: "Ancré", text: "Chaque essai doit utiliser les lettres que vous avez placées." },
    5: {
      name: "Tyrannie",
      text: "Les lettres placées doivent rester là où vous les avez placées.",
    },
    6: { name: "À l'Étroit", text: "Quatre emplacements de relique, pas cinq." },
    7: { name: "Années Maigres", text: "Chaque manche paie $1 de moins." },
    8: { name: "Poids Mort", text: "Une manche que vous n'avez pas résolue ne paie rien." },
    9: { name: "Sans Écho", text: "Aucun mot deux fois dans toute la partie." },
    10: {
      name: "Achevez-le",
      text: "Atteindre l'objectif ne suffit pas. Il faut résoudre le mot.",
    },
    steeper: {
      name: "Plus Raide",
      text: (percent, total) =>
        `Les objectifs montent encore de ${percent}\u00A0% (×${total} au total).`,
    },
  },

  round: ["Manche Normale", "Manche d'Élite", "Manche de Boss"],

  refusal: {
    not_your_turn: "ce n'est pas votre tour",
    not_a_letter: "ce n'est pas une lettre",
    letter_broken: ({ letter }) => `${letter.toUpperCase()} est brisée`,
    no_room: "plus de place",
    // French puts 0 in the singular, which is why this asks `Intl` rather than a
    // ternary. No refusal ever carries 0 here, and the rule is the rule.
    wrong_length: ({ length }) =>
      plural(length, { one: `${length} lettre`, other: `${length} lettres` }),
    not_in_word_list: "absent de la liste",

    must_use: ({ letter }) => `il faut utiliser ${letter.toUpperCase()}`,
    must_keep: ({ letter, position }) =>
      `${letter.toUpperCase()} doit rester en position ${position}`,
    needs_two_vowels: "il faut au moins deux voyelles",
    no_repeated_letters: "pas de lettre répétée",
    already_guessed_round: "déjà essayé dans cette manche",
    already_used_run: "déjà utilisé dans cette partie",

    only_during_round: "seulement pendant une manche",
    no_such_card: "cette carte n'existe pas",
    word_already_revealed: "le mot est déjà révélé en entier",
    nothing_to_reveal: "rien à révéler",
    nothing_to_rule_out: "plus rien à écarter",
    already_prepared: "déjà préparé",
    no_guess_to_repeat: "aucun essai à répéter",

    nothing_to_collect: "rien à encaisser",
    run_not_won: "la partie n'est pas gagnée",

    not_in_shop: "pas dans la boutique",
    sell_only_in_shop: "on ne vend qu'en boutique",
    finish_pack_first: "finissez d'abord le booster ouvert",
    place_mod_first: "placez d'abord le modificateur",
    already_bought: "déjà acheté",
    not_enough_gold: "pas assez d'or",
    no_such_relic: "cette relique n'existe pas",
    no_relic_slots: "aucun emplacement de relique libre",
    no_card_slots: "aucun emplacement de carte libre",
    pack_empty: "plus rien à mettre dans ce booster",
    no_pack_open: "aucun booster ouvert",
    already_taken: "déjà pris",
    nothing_to_place: "rien à placer",
    no_letter_for_mod: "plus de lettre pour ça",
    mod_not_allowed: ({ id, letter }) =>
      `${MODIFIER[id].name} ne peut pas aller sur ${letter.toUpperCase()}`,

    unknown_card: "carte inconnue",
    mod_needs_letter: "celui-là a d'abord besoin d'une lettre",
    nested_pack: "un booster ne peut pas sortir d'un booster",
    unknown_letter: "lettre inconnue",
    unknown_etching: "gravure inconnue",
    unknown_category: "catégorie inconnue",
    unknown_range: "plage inconnue",
    unknown_modifier: "modificateur inconnu",
    unknown_pack: "booster inconnu",
  },

  event: {
    growth: ({ amount, unit }) => `+${amount} ${UNIT[unit]}`,

    payout: (paid) => {
      switch (paid.kind) {
        case "chips":
          return `+${paid.amount}`
        case "mult":
          return `+${paid.amount} mult`
        case "times":
          return `×${paid.factor} mult`
        case "blocked":
          return "×1 bloqué"
        case "gold":
          return `+$${paid.amount}`
      }
    },

    categoryLevel: (name, level) => `${name} Niv ${level}`,
    modPlaced: (name, letter) => `${name} ${letter.toUpperCase()}`,
    note: (note) => {
      switch (note.card) {
        case "oracle":
          return `${note.letter.toUpperCase()} est en #${note.position}`
        case "hermit":
          return `pas de ${note.letter.toUpperCase()}`
        case "magician":
          return "la prochaine grise passe en jaune"
        case "fool":
          return `+${note.score}`
      }
    },

    guessNote: ({ count }) =>
      count === 0
        ? "aucune mal placée"
        : plural(count, {
            one: `${count} mal placée`,
            other: `${count} mal placées`,
          }),
  },

  ui: {
    /**
     * The long scale, which is why this table exists at all: `10^9` is a
     * `milliard` and not a `billion`, and a `billion` is `10^12`. So the third
     * rung is `Md` where English writes `B`, and every rung above it means one
     * order of magnitude more than the English letter it resembles.
     */
    units: ["k", "M", "Md", "B", "Bd", "T", "Td", "Qa"],

    error: {
      words: (cause) => `Impossible de charger les listes de mots\u00A0: ${cause}`,
    },

    common: {
      close: "Fermer",
      back: "Retour",
      play: "Jouer",
      howToPlay: "Comment jouer",
      codex: "Codex",
      soundOn: "Son activé",
      soundOff: "Son coupé",
      ascension: (level) => `Ascension ${level}`,
      percent: (share) => `${share}\u00A0%`,
      none: "—",
      loading: "Chargement des mots…",
    },

    board: {
      menu: "Menu",
      decor: {
        all: {
          label: "Ne marquer que ce que vous avez changé",
          tip: "Chaque lettre affiche ce qu'elle rapporte.\nAppuyez pour ne marquer que ce que vous avez changé.",
        },
        minimal: {
          label: "Nettoyer le plateau",
          tip: "Seuls les modificateurs et les lettres améliorées sont marqués.\nAppuyez pour nettoyer le plateau.",
        },
        none: {
          label: "Afficher la valeur de chaque lettre",
          tip: "Rien sur le plateau ne dit ce que vaut une lettre.\nAppuyez pour tout réafficher.",
        },
      },
      stage: (stage, total) => `Étape ${stage}/${total}`,
      stageEndless: (stage) => `Étape ${stage} ∞`,
      ascensionTag: (level) => `A${level}`,
      target: (target) => `sur ${target}`,
      relicTip: (text, detail) => `${text} (${detail})`,
      relicLabel: (name, text) => `${name}\u00A0: ${text}`,
      relicLabelGrown: (name, text, detail) => `${name}\u00A0: ${text} (${detail})`,
      // The two keys a French keyboard actually wears, under the AZERTY rows.
      enter: "ENTRÉE",
      del: "SUPPR",
      solveFactor: (factor) => `résoudre ×${factor}`,
      solveFloor: (score) => `→ ${score}`,
      solveFloorClears: (score) => `→ ${score}, ça passe`,
      shapeLevel: (level) => `Niv ${level}`,
      shapeBonus: (chips, mult) => `+${chips} +${mult} mult`,
      shapesMore: "formes ›",
      multUnknown: "La couleur est le multiplicateur. Deviner est le seul moyen de le savoir.",
      letterBroken: (letter) => `${letter.toUpperCase()} brisée`,
    },

    tip: {
      tileChips: (letter, chips) =>
        chips === 0
          ? `${letter.toUpperCase()} · aucun jeton`
          : plural(chips, {
              one: `${letter.toUpperCase()} · +${chips} jeton`,
              other: `${letter.toUpperCase()} · +${chips} jetons`,
            }),
      keyChips: (letter, chips) =>
        chips === 0
          ? `${letter.toUpperCase()} · aucun jeton`
          : plural(chips, {
              one: `${letter.toUpperCase()} · ${chips} jeton`,
              other: `${letter.toUpperCase()} · ${chips} jetons`,
            }),
      broken: (letter) => `${letter.toUpperCase()} · brisée, ne peut plus être tapée`,
      base: (chips) => `${chips} de base`,
      etched: (chips) => `+${chips} gravés`,
      fromRange: (chips, range, level) => `+${chips} de ${range} Niv ${level}`,
      boss: (name, text) => `${name}\u00A0: ${text}`,
      color: (color, mult) =>
        mult === 0 ? `${COLOR[color]} · aucun mult` : `${COLOR[color]} · +${mult} mult`,
      mod: (name, badge) => `${name} · ${badge}`,
      modIdle: (name, text) => `${name} · ${text}`,
      modSilenced: (name, text) => `${name} · ${text} · réduit au silence cette manche`,
      modQuiet: (name, text) => `${name} · ${text} · rien cette fois`,
      relic: (name, badge) => `${name} · ${badge}`,
      share: (chips, total) => `${chips} sur ${total} jetons · aucun mult`,
      shareWithMult: (chips, total, mult, multTotal) =>
        `${chips} sur ${total} jetons · ${mult} sur ${multTotal} mult`,
    },

    intro: {
      stage: (stage, total) => `Étape ${stage} sur ${total}`,
      stageEndless: (stage) => `Étape ${stage} · sans fin`,
      scoreAtLeast: "Marquez au moins",
      meta: (guesses, reward) =>
        plural(guesses, {
          one: `${guesses} essai · récompense ${reward}`,
          other: `${guesses} essais · récompense ${reward}`,
        }),
      targets: (factor) => `objectifs ×${factor}`,
      coachAsk: "Première partie. Le plateau peut vous expliquer le score au fur et à mesure.",
      coachYes: "Jouer avec les conseils",
      coachNo: "Passer les conseils",
      play: "Jouer",
    },

    reward: {
      cleared: "Manche réussie",
      answerWas: (word) => `Le mot était ${word.toUpperCase()}`,
      score: (score, target) => `${score} sur ${target}`,
      unusedGuesses: "Essais inutilisés",
      interest: "Intérêts",
      total: "Total",
      collect: "Encaisser",
    },

    shop: {
      title: "Boutique",
      sold: "vendu",
      sell: (amount) => `vendre ${amount}`,
      reroll: (cost) => `Renouveler ${cost}`,
      nextRound: "Manche suivante",
      owned: (relics, relicSlots, cards, cardSlots) =>
        `Reliques ${relics}/${relicSlots} · Consommables ${cards}/${cardSlots}`,
      ownedSellable: (relics, relicSlots, cards, cardSlots) =>
        `Reliques ${relics}/${relicSlots} · Consommables ${cards}/${cardSlots} · appuyez sur une relique pour la vendre`,
      shapesLabel: "Formes de mot",
      shapesLevel: (name, level) => `${name} Niv ${level}`,
      shapesNone: "toutes au niveau 1",

      tagPack: "Booster",
      tipPack: (picks) =>
        `Il étale ses cartes face visible et vous en gardez ${picks > 1 ? picks : "une"} gratuitement.`,
      tagRelic: "Relique",
      tipRelic: "Vous la gardez toute la partie, et elle agit seule à chaque manche.",
      tagConsumable: "Consommable",
      tagConsumableFull: "Consommable · emplacements pleins",
      tipConsumable: "Vous l'utilisez une fois, quand vous voulez, et il disparaît.",
      tagLetter: "Lettre",
      tipMod: "Il colle à une lettre pour le reste de la partie.",
      tagRange: "Alphabet",
      tipRange: "Il monte une tranche de l'alphabet, si bien que chaque lettre dedans vaut plus.",
      tagShape: "Forme de mot",
      tipShape: "Il monte une forme de mot, si bien que chaque essai de cette forme paie plus.",
      tagEtching: "Gravure",
      tipEtching:
        "Elle ajoute des jetons à un groupe de lettres pour de bon, et la racheter se cumule.",

      modAnyTitle: (name) => `${name} · n'importe quelle lettre`,
      modAnyText: (text) => `Choisissez n'importe quelle lettre. Elle ${text}`,
      modAnyTextOnly: (text, letters) =>
        `Choisissez n'importe quelle lettre. Elle ${text}, parmi ${letters}`,
      modTitle: (name, letter) => `${name} ${letter}`,
      modText: (letter, text) => `${letter} ${text}`,
      swap: (name, pip) => `Remplace ${name} ${pip}`,

      rangeTitle: (name, level) => `${name} → Niv ${level}`,
      rangeText: (letters, chips) => `${letters} valent +${chips} jetons par niveau`,
      levelTitle: (name, level) => `${name} → Niv ${level}`,
      levelText: (name, chips, mult) =>
        `Les mots ${name} rapportent +${chips} jetons et +${mult} mult par niveau`,
      fallbackRange: "Plage",
      fallbackLevel: "Niveau",
      fallbackEtching: "Gravure",
    },

    pack: {
      choose: (left) => `Choisissez-en une parmi ${left}`,
      choosePicks: (picks, left) => `Choisissez-en ${picks} parmi ${left}`,
      taken: "prise",
      skip: "Passer",
      skipSome: "Ne rien prendre de plus",
    },

    place: {
      choose: (text) => `Choisissez une lettre. Elle ${text}.`,
      onlyOn: (name, letters) => `${name} ne va que sur ${letters}.`,
      oneEach:
        "Une lettre ne porte qu'un modificateur. Appuyer sur une lettre déjà marquée demande d'abord.",
      carrying: (letter) => `${letter.toUpperCase()} porte`,
      loses: (name) => `Mettre ${name} ici le perd pour le reste de la partie.`,
      replace: (name) => `Remplacer ${name}`,
      keep: (name) => `Garder ${name}`,
    },

    end: {
      won: "Partie gagnée",
      lost: "Partie perdue",
      short: (score, target, by) => `${score} sur ${target}, il manquait ${by}`,
      wonAndWent: (stages) => `Étape ${stages} battue, et vous avez continué`,
      reached: (stage, round) => `Étape ${stage} atteinte, ${round}`,
      endlessNote: (stages) =>
        `L'étape ${stages} est là où le jeu s'arrête, pas là où la partie doit s'arrêter. ` +
        "La victoire est à vous dans tous les cas. Continuer ne fait que demander jusqu'où va " +
        "vraiment cette construction, et les objectifs croissent au même rythme tout du long.",
      endless: "Mode sans fin",
      mainMenu: "Menu principal",
      newRun: "Nouvelle partie",
      firstEarned: "L'ascension 1 est acquise, et la partie peut être rendue plus dure",
      earned: (level, next) => `Ascension ${level} réussie, et la ${next} est acquise`,
      topOfLadder: (level) => `Ascension ${level} réussie. Il n'y a rien au-dessus.`,
    },

    title: {
      name: "5 WILD",
      tagline: "Un roguelike de mots à deviner",
      runs: (count) => plural(count, { one: `${count} partie`, other: `${count} parties` }),
      wins: (count) => plural(count, { one: `${count} victoire`, other: `${count} victoires` }),
      bestStage: (stage) => `meilleure étape ${stage}`,
    },

    stats: {
      title: "Palmarès",
      runs: "parties",
      wins: (count) => plural(count, { one: "victoire", other: "victoires" }),
      bestStage: "meilleure étape",
      guesses: "essais",
      solved: "résolus",
      meanSolve: "essai moyen",
      cracked: (count) => plural(count, { one: `${num(count)} mot`, other: `${num(count)} mots` }),
      // Agrees with `mot`, and French puts 0 in the singular, so `0 mot trouvé`
      // is the right sentence and the rule engine already knows it — which is
      // why this goes through `plural` rather than through `count > 1`.
      crackedBare: (count) => plural(count, { one: "trouvé", other: "trouvés" }),
      crackedOf: (count, pool) =>
        plural(count, { one: `trouvé, sur ${pool}`, other: `trouvés, sur ${pool}` }),
      played: (count) => plural(count, { one: `${num(count)} mot`, other: `${num(count)} mots` }),
      // Agrees with `mot`, and takes the same singular 0 as the line above it.
      playedBare: (count) => plural(count, { one: "joué", other: "joués" }),
      playedOf: (count, pool) =>
        plural(count, { one: `joué, sur ${pool}`, other: `joués, sur ${pool}` }),
      mostPlayed: "Les plus jouées\u00A0:",
      // `fois` is invariable, so there is nothing for `Intl` to select between
      // and asking it would only make the sentence look like it had a choice.
      times: (count) => `${num(count)} fois`,
      breakdown: "Comment tombent les réponses",
      solvedIn: (guesses) => `Résolus en ${guesses}`,
      neverFound: "Jamais trouvés",
      favoriteRelics: "Reliques préférées",
      taken: (count) => `prise ${num(count)}×`,
      noStreak: "Aucune réponse trouvée pour l'instant.",
      streakBest: (now) => `${now} d'affilée, la plus longue série jusqu'ici, et elle continue.`,
      streakWithNow: (best, now) => `Meilleure série\u00A0: ${best} d'affilée, ${now} en cours.`,
      streak: (best) => `Meilleure série\u00A0: ${best} d'affilée.`,
    },

    ladder: {
      carrot: "Battez ceci pour débloquer l'ascension 1",
      lower: "Baisser l'ascension",
      raise: "Monter l'ascension",
      locked: "Verrouillé",
      skipTo: (level) => `Passer à l'ascension ${level}`,
      andBelow: "Et toutes les règles en dessous, aussi.",
      noRule: "Le jeu tel qu'il est écrit, sans rien de plus demandé.",
      askTitle: (level) => `Passer à l'ascension ${level}\u00A0?`,
      ruleLabel: (name) => `${name}\u00A0:`,
      askAndBelow: "Plus toutes les règles en dessous.",
      intended: "Gagner au niveau inférieur est la façon prévue de monter.",
      skipAnyway: (level) => `Passer quand même à ${level}`,
    },

    help: {
      title: "Comment jouer",
      lead:
        "Devinez le mot, comme vous savez déjà le faire\u00A0: vert, c'est la bonne lettre à la " +
        "bonne place, jaune, c'est la bonne lettre ailleurs.",
      scored: "La différence, c'est que chaque essai est compté.",
      chipsMult: {
        term: "Jetons × Mult",
        text: "Chaque essai vaut ses jetons multipliés par son mult.",
      },
      letters: {
        term: "Les lettres paient des jetons",
        text:
          "Les lettres rares paient plus. La boutique vend deux façons de les monter\u00A0: les " +
          "gravures, qui ajoutent à un type de lettre, et les niveaux sur une tranche de " +
          "l'alphabet. Chaque lettre est dans exactement une tranche, et les deux se cumulent.",
      },
      colors: {
        term: "Les couleurs paient du mult",
        text:
          "Le vert vaut +3 mult, le jaune +1, le gris rien. Un essai plein de gris ne vaut " +
          "presque rien, si bien qu'un sondage jeté au hasard vous coûte vraiment des points.",
      },
      solving: {
        term: "Résoudre multiplie la manche",
        text:
          "Trouvez le mot et tout le tas mis de côté dans la manche, pas seulement l'essai qui " +
          "l'a résolu, est multiplié par 1 + les essais qui vous restaient. Puis la manche " +
          "s'arrête aussitôt, objectif atteint ou non.",
      },
      farming:
        "C'est tout le jeu\u00A0: chaque essai dépensé à cultiver fait grossir le tas, et " +
        "rétrécit le multiplicateur qui l'attend.",
      solveLine: {
        term: "Surveillez donc la ligne de résolution",
        text:
          "Sous le plateau, elle montre le multiplicateur qu'une résolution rapporterait tout " +
          "de suite, et ce que le tas vaut déjà avec lui. Quand elle passe au vert, résoudre " +
          "gagne la manche.",
      },
      runHeading: "La partie",
      target: {
        term: "Battez l'objectif",
        text: (stages, rounds) =>
          `${stages} étapes de ${rounds} manches. Ratez l'objectif d'une manche et la partie ` +
          "est finie. C'est la seule façon de perdre.",
      },
      endless: {
        term: "Puis continuez, si vous l'osez",
        text: (stages) =>
          `Réussir l'étape ${stages} gagne la partie, et vous pouvez vous arrêter là ou pousser ` +
          "vers des étapes que personne n'a équilibrées. Les objectifs croissent au même rythme, " +
          "et mourir là-bas ne vous reprend pas la victoire.",
      },
      bosses: {
        term: "Boss",
        text: "Une manche sur trois tord une règle. Lisez-la avant de jouer.",
      },
      ascensions: {
        term: "Ascensions",
        text: (authored) =>
          "Le curseur de difficulté de l'écran d'accueil, et le chiffre qui vaut la peine d'être " +
          `comparé. Les ${authored} premiers niveaux ajoutent chacun une règle permanente\u00A0: ` +
          "sur ce que vous pouvez deviner, sur ce que paie une manche, sur la hauteur des " +
          "objectifs, sur le nombre de reliques que vous gardez, sur le nombre d'essais. " +
          "Au-dessus, l'échelle ne s'arrête pas\u00A0: chaque niveau de plus relève encore tous " +
          "les objectifs. Gagner à l'un ouvre le suivant, et monter barreau par barreau est la " +
          "façon prévue, pas un verrou.",
      },
      money: {
        term: "Argent",
        text: (payouts, perGuess, per, cap) =>
          `Les manches paient ${payouts}, plus ${perGuess} par essai inutilisé, plus $1 ` +
          `d'intérêt par ${per} que vous détenez, jusqu'à ${cap}. Rester assis sur son argent ` +
          "est une stratégie.",
      },
      relics: {
        term: "Reliques",
        text: (slots) =>
          `Jusqu'à ${slots}, et elles agissent de gauche à droite, si bien que l'ordre d'achat ` +
          "compte. Appuyez sur l'une d'elles pour la lire.",
      },
      packs: {
        term: "Boosters",
        text:
          "Un emplacement vend un choix plutôt qu'une carte. Un booster en étale trois et vous " +
          "en gardez une, gratuitement. Le reste de la boutique attend que vous ayez choisi ou " +
          "que vous soyez parti, et partir ne garde rien.",
      },
      mods: {
        term: "Modificateurs de lettre",
        text:
          "La boutique vend des modificateurs et vous choisissez à quelle lettre chacun colle " +
          "pour le reste de la partie. Chaque fois que vous jouez cette lettre, il fait ceci. " +
          "Une lettre ×mult multiplie ce que le mot a rapporté jusqu'à sa position, si bien " +
          "qu'elle vaut plus tard dans un mot que tôt. Les boosters les distribuent moins cher, " +
          "la lettre déjà choisie pour vous.",
      },
      codexNote:
        "Le codex contient toutes les reliques, tous les boss, toutes les formes de mot et tous les modificateurs du jeu, en entier.",
      openCodex: "Ouvrir le codex",
      gotIt: "Compris",
    },

    shapes: {
      title: "Formes de mot",
      scoresAs: (word, shape) => `${word.toUpperCase()} compte comme ${shape}`,
      anyWord: "Chaque essai a une forme.",
      note:
        "Un essai compte comme la forme la plus rare qu'il satisfait, c'est-à-dire la première " +
        "de cette liste qu'il satisfait. Monter une forme relève tous les essais futurs de " +
        "cette forme. Le niveau 1 ne paie rien, si bien qu'un niveau est ce qui rend une forme " +
        "digne d'être visée.",
      scoring: "compte comme",
      alsoMatches: "satisfait aussi",
      payNow: (chips, mult) => `désormais +${chips} jetons, +${mult} mult`,
      payPerLevel: (chips, mult) => `+${chips} jetons, +${mult} mult par niveau`,
    },

    codex: {
      title: "Codex",
      lead: "Tout ce qu'il y a dans le jeu, que vous l'ayez rencontré ou non.",
      relics: {
        title: "Reliques",
        blurb: (slots) =>
          `Jusqu'à ${slots} à la fois, agissant de gauche à droite, si bien que l'ordre d'achat ` +
          "fait partie de la construction.",
      },
      bosses: {
        title: "Boss",
        blurb:
          "Une manche sur trois. Chaque groupe est tiré sans remise, si bien qu'une partie ne " +
          "croise jamais deux fois le même boss.",
      },
      ascensions: {
        title: "Ascensions",
        blurb:
          "La difficulté de la partie elle-même, choisie avant qu'elle commence et fixée pour " +
          "toute sa durée. Une partie à un niveau joue toutes les règles jusqu'à lui, et gagner " +
          "à l'un débloque le suivant. Voici celles qui sont écrites\u00A0; passé la dernière, " +
          "chaque niveau relève simplement tous les objectifs de 8\u00A0% de plus, et il n'y a pas de " +
          "dernier niveau.",
      },
      shapes: {
        title: "Formes de mot",
        blurb:
          "Chaque essai compte comme exactement une forme\u00A0: la plus rare qu'il satisfait, " +
          "c'est-à-dire la première de cette liste. Monter une forme relève tous les essais " +
          "futurs de cette forme.",
      },
      mods: {
        title: "Modificateurs de lettre",
        blurb:
          "Collés à une seule lettre pour le reste de la partie. Un seul à la fois par lettre, " +
          "et le clavier en porte la marque. Une lettre ×mult multiplie ce que le mot a rapporté " +
          "jusqu'à sa position, si bien qu'elle vaut plus tard dans un mot que tôt. Le prix en " +
          "boutique achète la carte et vous laisse choisir la lettre\u00A0; un booster la " +
          "distribue au prix indiqué, la lettre déjà choisie.",
      },
      upgrades: {
        title: "Améliorations de lettre",
        blurb:
          "Deux voies qui ajoutent toutes deux des jetons aux lettres, et se cumulent\u00A0: " +
          "les gravures relèvent un type de lettre, les plages relèvent une tranche de " +
          "l'alphabet. Chaque lettre est dans exactement une tranche.",
      },
      consumables: {
        title: "Consommables",
        blurb: (slots) => `Utilisés une fois, quand vous voulez. Vous pouvez en tenir ${slots}.`,
      },
      packs: {
        title: "Boosters",
        blurb:
          "Vendus dans un emplacement à eux. Un booster étale ses cartes et vous en gardez une, " +
          "gratuitement. La boutique attend que vous ayez choisi ou que vous soyez parti.",
      },
      rarity: {
        common: "commune",
        uncommon: "peu commune",
        rare: "rare",
        legendary: "légendaire",
      },
      tier: { early: "Début", mid: "Milieu", late: "Fin" },
      tierBand: (tier, first, last) => `${tier} · étapes ${first}–${last}`,
      shapePer: (chips, mult) => `+${chips} / +${mult} par niveau`,
      modName: (name, pip) => `${name} ${pip}`,
      modText: (text) => `La lettre ${text}.`,
      modTextOnly: (text, letters) => `La lettre ${text}. Seulement sur ${letters}.`,
      packText: (text) => `${text}.`,
      packTextPicks: (text, picks) => `${text}, et gardez-en ${picks}.`,
    },

    pause: {
      title: "En pause",
      musicOn: "Musique activée",
      musicOff: "Musique coupée",
      speed: (speed) => `Vitesse d'animation ×${speed}`,
      language: "Langue",
      wordsNextRun: "Les mots changeront à la prochaine partie.",
      quit: "Abandonner la partie",
      resume: "Reprendre",
    },

    quit: {
      title: "Abandonner cette partie\u00A0?",
      body: (stage, total, round) =>
        `Vous êtes à l'étape ${stage} sur ${total}, ${round}. L'abandonner la supprime. ` +
        "Il n'y a pas de retour vers cette partie.",
      bodyEndless: (stage, round) =>
        `Vous êtes à l'étape ${stage}, ${round}. L'abandonner la supprime. ` +
        "Il n'y a pas de retour vers cette partie.",
      confirm: "Abandonner la partie",
      cancel: "Continuer à jouer",
    },

    coach: {
      chips:
        "Chaque essai est compté. Tapez un mot, et la ligne sous le plateau additionne ce que valent ses lettres.",
      rare: (chips) =>
        `${chips} jetons pour l'instant. Les lettres rares paient plus\u00A0: A vaut 1, K vaut 5, Z vaut 10.`,
      mult: "Le ? est le mult, et seule la réponse le connaît\u00A0: vert +3, jaune +1, gris rien. Appuyez sur ENTRÉE pour le découvrir.",
      banked: (chips, mult, score, target) =>
        `${chips} × ${mult} = ${score}, mis de côté vers ${target}. Chaque essai s'ajoute au même tas.`,
      solve: (now, next) =>
        `Résoudre multiplie tout le tas par ×${now}, pas seulement l'essai qui y arrive, ` +
        `et termine la manche. Un essai de plus et ce serait ×${next}.`,
    },
  },
}
