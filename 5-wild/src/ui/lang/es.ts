import type { Color, Growth } from "../../engine"
import { formatNumber as num, pluralizer } from "../format"
import type { Strings } from "./types"

const plural = pluralizer("es")

/** Read by one refusal, which has to name the card it is turning down. */
const MODIFIER: Strings["modifier"] = {
  chip: { name: "Ficha", text: "puntúa +20 fichas" },
  mult: { name: "Mult", text: "puntúa +8 mult" },
  gold: { name: "Oro", text: "paga $2 cada vez que la juegas" },
  wild: { name: "Comodín", text: "puntúa +24 mult en gris, +14 en amarillo, +4 en verde" },
  lucky: { name: "Suerte", text: "tiene 1 posibilidad entre 4 de puntuar +20 mult" },
  echo: { name: "Eco", text: "puntúa +60 fichas cuando la palabra la repite" },
  anchor: { name: "Ancla", text: "puntúa +125 fichas cuando cae en verde" },
  steel: { name: "Acero", text: "puntúa ×2 mult" },
  glass: { name: "Cristal", text: "puntúa ×3 mult, y puede romperse cuando cae en gris" },
}

const UNIT: Record<Growth["unit"], string> = { chips: "fichas", mult: "mult" }

const COLOR: Record<Color, string> = { green: "verde", yellow: "amarillo", gray: "gris" }

/**
 * Spanish. `en.ts` is the reference; this is one filling of the same shape.
 *
 * Four words carry the whole game and are used the same way everywhere below,
 * so they are worth stating once. A *chip* is a `ficha`, the gambling token, not
 * a `patata frita` and not a `chip` — the game is built out of a casino's
 * vocabulary and Spanish has the same casino. A *guess* is an `intento` rather
 * than a `conjetura`, because the player is spending one of six, and what they
 * spend is an attempt. A *run* is a `partida`, which is also what `game` would
 * be, and that collision is fine here: this game has no other meaning for the
 * word, and `recorrida` reads like a bus route. A *stage* is a `fase` and a
 * *round* is a `ronda`, which keeps the two nested things distinct where
 * `etapa`/`ronda` would have blurred them.
 *
 * The numbers in these sentences are restatements, exactly as in English. When a
 * relic is rebalanced its prose moves in four files, not one.
 */
export const es: Strings = {
  tag: "es",
  endonym: "Español",

  relic: {
    green_thumb: { name: "Mano Verde", text: "+8 fichas por casilla verde" },
    scavenger: { name: "Carroñero", text: "+$1 por casilla amarilla" },
    vowel_hoarder: { name: "Acaparador de Vocales", text: "+4 mult por vocal" },
    slow_burn: {
      name: "Fuego Lento",
      text: "+5 mult por cada intento ya hecho en la ronda",
    },
    consonant_cluster: {
      name: "Grupo Consonántico",
      text: "×1.5 mult si la palabra tiene 3+ consonantes seguidas",
    },
    cold_open: { name: "Arranque en Frío", text: "+30 fichas en el primer intento de la ronda" },
    bloodhound: { name: "Sabueso", text: "+6 fichas por casilla amarilla" },
    head_start: { name: "Ventaja", text: "+15 mult si la palabra empieza por vocal" },
    loaded_dice: { name: "Dados Cargados", text: "+0 a +20 mult, tirados de nuevo cada intento" },
    anagrammer: { name: "Anagramista", text: "×2 mult si no se repite ninguna letra" },
    keystone: { name: "Clave de Bóveda", text: "×3 mult si la casilla central es verde" },
    lexicographer: {
      name: "Lexicógrafo",
      text: "+3 fichas por cada letra distinta de tus intentos anteriores en la ronda",
    },
    sunk_cost: { name: "Coste Hundido", text: "+10 mult por cada intento que te quedaría" },
    speedrunner: { name: "Velocista", text: "×3 mult si resuelves en 3 intentos o menos" },
    qs_bargain: { name: "El Trato de la Q", text: "J, Q, X y Z puntúan fichas triples" },
    greedy_grammarian: { name: "Gramático Avaro", text: "+15 fichas por casilla gris" },
    doppelganger: {
      name: "Doppelgänger",
      text: "Las letras repetidas puntúan sus fichas dos veces",
    },
    hot_streak: {
      name: "Racha",
      text: "Gana +30 fichas permanentes por cada ronda que superes en 3 intentos o menos",
    },
    hoarder: {
      name: "El Acaparador",
      text: "Gana +40 fichas permanentes al llegar a la tienda con las dos ranuras de carta llenas",
    },
    masochist: { name: "Masoquista", text: "+8 mult por casilla gris" },
    chorus: { name: "El Coro", text: "×3 mult si la palabra tiene tres vocales o más" },
    alphabetist: { name: "Alfabetista", text: "×2 mult si tus letras van en orden alfabético" },
    vault: { name: "La Cámara", text: "+25 fichas por cada intento ya hecho en la ronda" },
    mint: {
      name: "La Casa de la Moneda",
      text: "+3 mult por cada $5 que tengas. No ganas intereses.",
    },
    scorched_earth: {
      name: "Tierra Quemada",
      text: "+12 mult por cada letra rota del alfabeto",
    },
    snowball: {
      name: "Bola de Nieve",
      text: "Gana +1 mult permanente por cada casilla verde que juegues",
    },
    long_game: { name: "El Juego Largo", text: "+1 a tu multiplicador de resolución" },
    pyromaniac: {
      name: "Pirómano",
      text: "+40 mult. Rompe una letra al azar del alfabeto en cada ronda",
    },
  },

  boss: {
    silence: {
      name: "El Silencio",
      text: "Las letras mal colocadas puntúan y se ven como ausentes. Solo se te dice cuántas hay.",
    },
    fog: {
      name: "La Niebla",
      text: "El amarillo y el gris se ven iguales. Siguen puntuando distinto.",
    },
    tyrant: {
      name: "El Tirano",
      text: "Cada intento debe reutilizar las letras verdes que hayas encontrado.",
    },
    miser: { name: "El Avaro", text: "Las letras que ya hayas usado no puntúan fichas." },
    clock: { name: "El Reloj", text: "Solo cuatro intentos." },
    glutton: { name: "El Glotón", text: "Cada intento debe contener al menos dos vocales." },
    auditor: { name: "El Auditor", text: "Tu multiplicador de resolución se limita a ×2." },
    purist: { name: "El Purista", text: "Ninguna letra puede aparecer dos veces en un intento." },
    drought: { name: "La Sequía", text: "Las vocales no puntúan fichas." },
    mirror: {
      name: "El Espejo",
      text: "Tus pistas se muestran del revés. Siguen puntuando como cayeron.",
    },
    famine: { name: "La Hambruna", text: "Solo tres intentos." },
    rust: {
      name: "El Óxido",
      text: "Las mejoras de letra no puntúan. Cada letra vale solo lo que valía al principio.",
    },
    margin: { name: "El Margen", text: "La primera y la última letra no puntúan fichas." },
    vandal: { name: "El Vándalo", text: "Los modificadores de letra no hacen nada." },
    plateau: {
      name: "La Meseta",
      text: "Los efectos que multiplican no hacen nada. El mult solo puede sumarse.",
    },
  },

  modifier: MODIFIER,

  consumable: {
    oracle: { name: "El Oráculo", text: "Revela una letra de la respuesta, en su sitio" },
    hermit: {
      name: "El Ermitaño",
      text: "Descarta una letra de la respuesta sin gastar un intento",
    },
    magician: {
      name: "El Mago",
      text: "Tu próximo intento puntúa su primera casilla gris como amarilla. Da mult, no una pista.",
    },
    fool: { name: "El Loco", text: "Puntúa tu intento anterior una segunda vez" },
  },

  pack: {
    alphabet: { name: "Sobre de Alfabeto", text: "Elige uno de tres modificadores de letra" },
    relic: { name: "Sobre de Reliquias", text: "Elige una de tres reliquias" },
    category: {
      name: "Sobre de Categorías",
      text: "Elige una de tres categorías de palabra para subir de nivel",
    },
  },

  category: {
    alphabetical: { name: "Alfabética", text: "Sus letras nunca retroceden" },
    vowel_heavy: { name: "Rica en Vocales", text: "Tres vocales o más" },
    cluster: { name: "Grupo", text: "Tres consonantes seguidas" },
    twinned: { name: "Gemela", text: "Alguna letra aparece dos veces" },
    distinct: { name: "Distinta", text: "No se repite ninguna letra" },
  },

  etching: {
    etch_vowels: {
      name: "Grabar Vocales",
      text: (chips) => `A E I O U valen +${chips} fichas`,
    },
    etch_staples: {
      name: "Grabar Básicas",
      text: (chips) => `L N S T R valen +${chips} fichas`,
    },
    etch_heavy: {
      name: "Grabar Pesadas",
      text: (chips) => `J Q X Z valen +${chips} fichas`,
    },
    etch_consonants: {
      name: "Grabar Consonantes",
      text: (chips) =>
        plural(chips, {
          one: `Cada consonante vale +${chips} ficha`,
          other: `Cada consonante vale +${chips} fichas`,
        }),
    },
  },

  ascension: {
    1: { name: "Acosado", text: "Cada intento debe usar las letras que hayas encontrado." },
    2: { name: "Una Sola Vez", text: "Ninguna palabra dos veces en la misma ronda." },
    3: { name: "Más Empinado", text: "Cada objetivo es un 15\u00A0% mayor." },
    4: { name: "Anclado", text: "Cada intento debe usar las letras que hayas colocado." },
    5: {
      name: "Tiranía",
      text: "Las letras que hayas colocado deben quedarse donde las colocaste.",
    },
    6: { name: "Estrecho", text: "Cuatro ranuras de reliquia, no cinco." },
    7: { name: "Años de Vacas Flacas", text: "Cada ronda paga $1 menos." },
    8: { name: "Peso Muerto", text: "Una ronda que no resuelvas no paga nada." },
    9: { name: "Sin Ecos", text: "Ninguna palabra dos veces en toda la partida." },
    10: {
      name: "Remátalo",
      text: "Llegar al objetivo no basta. Tienes que resolver la palabra.",
    },
    steeper: {
      name: "Más Empinado",
      text: (percent, total) => `Los objetivos suben otro ${percent}\u00A0% (×${total} en total).`,
    },
  },

  round: ["Ronda Normal", "Ronda de Élite", "Ronda de Jefe"],

  refusal: {
    not_your_turn: "no es tu turno",
    not_a_letter: "no es una letra",
    letter_broken: ({ letter }) => `la ${letter.toUpperCase()} está rota`,
    no_room: "no hay sitio",
    // Spanish agrees here where English does not: one letter is `1 letra`.
    wrong_length: ({ length }) =>
      plural(length, { one: `${length} letra`, other: `${length} letras` }),
    not_in_word_list: "no está en la lista",

    must_use: ({ letter }) => `debes usar la ${letter.toUpperCase()}`,
    must_keep: ({ letter, position }) =>
      `la ${letter.toUpperCase()} debe quedarse en la posición ${position}`,
    needs_two_vowels: "necesita al menos dos vocales",
    no_repeated_letters: "sin letras repetidas",
    already_guessed_round: "ya la has usado en esta ronda",
    already_used_run: "ya la has usado en esta partida",

    only_during_round: "solo durante una ronda",
    no_such_card: "esa carta no existe",
    word_already_revealed: "la palabra ya está revelada entera",
    nothing_to_reveal: "nada que revelar",
    nothing_to_rule_out: "no queda nada que descartar",
    already_prepared: "ya está preparado",
    no_guess_to_repeat: "no hay intento que repetir",

    nothing_to_collect: "nada que cobrar",
    run_not_won: "la partida no está ganada",

    not_in_shop: "no estás en la tienda",
    sell_only_in_shop: "solo puedes vender en la tienda",
    finish_pack_first: "termina antes el sobre abierto",
    place_mod_first: "coloca antes el modificador",
    already_bought: "ya lo has comprado",
    not_enough_gold: "no tienes suficiente oro",
    no_such_relic: "esa reliquia no existe",
    no_relic_slots: "sin ranuras de reliquia libres",
    no_card_slots: "sin ranuras de carta libres",
    pack_empty: "no queda nada que poner en ese sobre",
    no_pack_open: "no hay ningún sobre abierto",
    already_taken: "ya la has cogido",
    nothing_to_place: "nada que colocar",
    no_letter_for_mod: "no queda letra para eso",
    mod_not_allowed: ({ id, letter }) =>
      `${MODIFIER[id].name} no puede ir en la ${letter.toUpperCase()}`,

    unknown_card: "carta desconocida",
    mod_needs_letter: "ese necesita una letra primero",
    nested_pack: "un sobre no puede salir de un sobre",
    unknown_letter: "letra desconocida",
    unknown_etching: "grabado desconocido",
    unknown_category: "categoría desconocida",
    unknown_range: "rango desconocido",
    unknown_modifier: "modificador desconocido",
    unknown_pack: "sobre desconocido",
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
          return "×1 bloqueado"
        case "gold":
          return `+$${paid.amount}`
      }
    },

    // `Nv` for `nivel`, which is what every level in this game is written as:
    // the board, the shop and the codex all use it and they have to match.
    categoryLevel: (name, level) => `${name} Nv ${level}`,
    modPlaced: (name, letter) => `${name} ${letter.toUpperCase()}`,
    note: (note) => {
      switch (note.card) {
        case "oracle":
          return `la ${note.letter.toUpperCase()} es la #${note.position}`
        case "hermit":
          return `sin ${note.letter.toUpperCase()}`
        case "magician":
          return "el próximo gris pasa a amarillo"
        case "fool":
          return `+${note.score}`
      }
    },

    guessNote: ({ count }) =>
      count === 0
        ? "ninguna mal colocada"
        : plural(count, {
            one: `${count} mal colocada`,
            other: `${count} mal colocadas`,
          }),
  },

  ui: {
    /**
     * The long scale, which is the whole reason this is a per-language table:
     * `10^9` is not a `billón` in Spanish, it is `mil millones`, and a `billón`
     * is `10^12`. So the rungs go K, M, and then an `M` prefix meaning `mil`
     * on each named order: MM is `mil millones`, MB is `mil billones`. Nothing
     * past B is reachable in a run that ends, and endless has no last rung.
     */
    units: ["K", "M", "MM", "B", "MB", "T", "MT", "C"],

    error: {
      words: (cause) => `No se pudieron cargar las listas de palabras: ${cause}`,
    },

    common: {
      close: "Cerrar",
      back: "Atrás",
      play: "Jugar",
      howToPlay: "Cómo se juega",
      codex: "Códice",
      soundOn: "Sonido activado",
      soundOff: "Sonido desactivado",
      ascension: (level) => `Ascensión ${level}`,
      // Spanish sets the sign off from the number, which is exactly why this is
      // a catalog entry and not a `${n}%` at the call site.
      percent: (share) => `${share}\u00A0%`,
      none: "—",
      loading: "Cargando palabras…",
    },

    board: {
      menu: "Menú",
      decor: {
        all: {
          label: "Marcar solo lo que has cambiado",
          tip: "Cada letra muestra lo que puntúa.\nToca para marcar solo lo que has cambiado.",
        },
        minimal: {
          label: "Despejar el tablero",
          tip: "Solo se marcan los modificadores y las letras mejoradas.\nToca para despejar el tablero.",
        },
        none: {
          label: "Mostrar el valor de cada letra",
          tip: "Nada en el tablero dice lo que vale una letra.\nToca para mostrarlo todo otra vez.",
        },
      },
      stage: (stage, total) => `Fase ${stage}/${total}`,
      stageEndless: (stage) => `Fase ${stage} ∞`,
      ascensionTag: (level) => `A${level}`,
      target: (target) => `de ${target}`,
      relicTip: (text, detail) => `${text} (${detail})`,
      relicLabel: (name, text) => `${name}: ${text}`,
      relicLabelGrown: (name, text, detail) => `${name}: ${text} (${detail})`,
      // The two keys a Spanish keyboard actually wears, which is the point of
      // them: they are read as keys, not as words.
      enter: "INTRO",
      del: "SUPR",
      solveFactor: (factor) => `resolver ×${factor}`,
      solveFloor: (score) => `→ ${score}`,
      solveFloorClears: (score) => `→ ${score}, supera`,
      shapeLevel: (level) => `Nv ${level}`,
      shapeBonus: (chips, mult) => `+${chips} +${mult} mult`,
      shapesMore: "formas ›",
      multUnknown: "El color es el multiplicador. Adivinar es como se descubre.",
      letterBroken: (letter) => `${letter.toUpperCase()} rota`,
    },

    tip: {
      tileChips: (letter, chips) =>
        chips === 0
          ? `${letter.toUpperCase()} · sin fichas`
          : plural(chips, {
              one: `${letter.toUpperCase()} · +${chips} ficha`,
              other: `${letter.toUpperCase()} · +${chips} fichas`,
            }),
      keyChips: (letter, chips) =>
        chips === 0
          ? `${letter.toUpperCase()} · sin fichas`
          : plural(chips, {
              one: `${letter.toUpperCase()} · ${chips} ficha`,
              other: `${letter.toUpperCase()} · ${chips} fichas`,
            }),
      broken: (letter) => `${letter.toUpperCase()} · rota, ya no se puede escribir`,
      base: (chips) => `${chips} de base`,
      etched: (chips) => `+${chips} grabadas`,
      fromRange: (chips, range, level) => `+${chips} de ${range} Nv ${level}`,
      boss: (name, text) => `${name}: ${text}`,
      color: (color, mult) =>
        mult === 0 ? `${COLOR[color]} · sin mult` : `${COLOR[color]} · +${mult} mult`,
      mod: (name, badge) => `${name} · ${badge}`,
      modIdle: (name, text) => `${name} · ${text}`,
      modSilenced: (name, text) => `${name} · ${text} · silenciado esta ronda`,
      modQuiet: (name, text) => `${name} · ${text} · nada esta vez`,
      relic: (name, badge) => `${name} · ${badge}`,
      share: (chips, total) => `${chips} de ${total} fichas · sin mult`,
      shareWithMult: (chips, total, mult, multTotal) =>
        `${chips} de ${total} fichas · ${mult} de ${multTotal} mult`,
    },

    intro: {
      stage: (stage, total) => `Fase ${stage} de ${total}`,
      stageEndless: (stage) => `Fase ${stage} · infinita`,
      scoreAtLeast: "Puntúa al menos",
      meta: (guesses, reward) =>
        plural(guesses, {
          one: `${guesses} intento · recompensa ${reward}`,
          other: `${guesses} intentos · recompensa ${reward}`,
        }),
      targets: (factor) => `objetivos ×${factor}`,
      coachAsk: "Primera partida. El tablero puede explicarte la puntuación mientras juegas.",
      coachYes: "Jugar con consejos",
      coachNo: "Saltar los consejos",
      play: "Jugar",
    },

    reward: {
      cleared: "Ronda superada",
      answerWas: (word) => `La palabra era ${word.toUpperCase()}`,
      score: (score, target) => `${score} de ${target}`,
      unusedGuesses: "Intentos sin usar",
      interest: "Intereses",
      total: "Total",
      collect: "Cobrar",
    },

    shop: {
      title: "Tienda",
      sold: "vendido",
      sell: (amount) => `vender ${amount}`,
      // `Renovar` rather than a calque of reroll: what the button does is deal a
      // new shelf, and no Spanish player is rolling anything.
      reroll: (cost) => `Renovar ${cost}`,
      nextRound: "Siguiente ronda",
      owned: (relics, relicSlots, cards, cardSlots) =>
        `Reliquias ${relics}/${relicSlots} · Consumibles ${cards}/${cardSlots}`,
      ownedSellable: (relics, relicSlots, cards, cardSlots) =>
        `Reliquias ${relics}/${relicSlots} · Consumibles ${cards}/${cardSlots} · toca una reliquia para venderla`,
      shapesLabel: "Formas de palabra",
      shapesLevel: (name, level) => `${name} Nv ${level}`,
      shapesNone: "todas a nivel 1",

      tagPack: "Sobre",
      tipPack: (picks) =>
        `Reparte sus cartas boca arriba y te quedas ${picks > 1 ? picks : "una"} gratis.`,
      tagRelic: "Reliquia",
      tipRelic: "La conservas toda la partida, y actúa sola en cada ronda.",
      tagConsumable: "Consumible",
      tagConsumableFull: "Consumible · ranuras llenas",
      tipConsumable: "Lo usas una vez, cuando quieras, y desaparece.",
      tagLetter: "Letra",
      tipMod: "Se pega a una letra durante el resto de la partida.",
      tagRange: "Alfabeto",
      tipRange: "Sube de nivel un tramo del alfabeto, así cada letra suya vale más.",
      tagShape: "Forma de palabra",
      tipShape: "Sube de nivel una forma de palabra, así cada intento de esa forma paga más.",
      tagEtching: "Grabado",
      tipEtching: "Añade fichas a un grupo de letras para siempre, y comprarlo otra vez acumula.",

      modAnyTitle: (name) => `${name} · cualquier letra`,
      modAnyText: (text) => `Elige cualquier letra. ${text}`,
      modAnyTextOnly: (text, letters) => `Elige cualquier letra. ${text}, de ${letters}`,
      modTitle: (name, letter) => `${name} ${letter}`,
      modText: (letter, text) => `${letter} ${text}`,
      swap: (name, pip) => `Sustituye a ${name} ${pip}`,

      rangeTitle: (name, level) => `${name} → Nv ${level}`,
      rangeText: (letters, chips) => `${letters} valen +${chips} fichas por nivel`,
      levelTitle: (name, level) => `${name} → Nv ${level}`,
      levelText: (name, chips, mult) =>
        `Las palabras ${name} puntúan +${chips} fichas y +${mult} mult por nivel`,
      fallbackRange: "Rango",
      fallbackLevel: "Nivel",
      fallbackEtching: "Grabado",
    },

    pack: {
      choose: (left) => `Elige una de ${left}`,
      choosePicks: (picks, left) => `Elige ${picks} de ${left}`,
      taken: "cogida",
      skip: "Saltar",
      skipSome: "No coger más",
    },

    place: {
      choose: (text) => `Elige una letra. ${text}.`,
      onlyOn: (name, letters) => `${name} solo va en ${letters}.`,
      oneEach: "Cada letra lleva un modificador. Tocar una que ya tiene marca pregunta antes.",
      carrying: (letter) => `La ${letter.toUpperCase()} lleva`,
      loses: (name) => `Poner ${name} aquí lo pierde durante el resto de la partida.`,
      replace: (name) => `Sustituir ${name}`,
      keep: (name) => `Conservar ${name}`,
    },

    end: {
      won: "Partida completada",
      lost: "Partida terminada",
      short: (score, target, by) => `${score} de ${target}, te faltan ${by}`,
      wonAndWent: (stages) => `Superaste la fase ${stages} y seguiste`,
      reached: (stage, round) => `Llegaste a la fase ${stage}, ${round}`,
      endlessNote: (stages) =>
        `La fase ${stages} es donde para el juego, no donde tiene que parar la partida. ` +
        "La victoria es tuya de todos modos. Seguir solo pregunta hasta dónde llega de " +
        "verdad esta construcción, y los objetivos crecen al mismo ritmo todo el camino.",
      endless: "Modo infinito",
      mainMenu: "Menú principal",
      newRun: "Nueva partida",
      firstEarned: "Has ganado la ascensión 1, y la partida ya puede ponerse más difícil",
      earned: (level, next) => `Ascensión ${level} superada, y has ganado la ${next}`,
      topOfLadder: (level) => `Ascensión ${level} superada. No hay nada por encima.`,
    },

    title: {
      name: "5 WILD",
      tagline: "Un roguelike de adivinar palabras",
      runs: (count) => plural(count, { one: `${count} partida`, other: `${count} partidas` }),
      wins: (count) => plural(count, { one: `${count} victoria`, other: `${count} victorias` }),
      bestStage: (stage) => `mejor fase ${stage}`,
    },

    stats: {
      title: "Registro",
      runs: "partidas",
      wins: (count) => plural(count, { one: "victoria", other: "victorias" }),
      bestStage: "mejor fase",
      guesses: "intentos",
      solved: "resueltas",
      meanSolve: "intento medio",
      cracked: (count) =>
        plural(count, { one: `${num(count)} palabra`, other: `${num(count)} palabras` }),
      // Agrees with `palabra`, which is where the count in the signature earns
      // its keep: the tail carries no number and still has to know it.
      crackedBare: (count) => plural(count, { one: "descifrada", other: "descifradas" }),
      crackedOf: (count, pool) =>
        plural(count, { one: `descifrada, de ${pool}`, other: `descifradas, de ${pool}` }),
      played: (count) =>
        plural(count, { one: `${num(count)} palabra`, other: `${num(count)} palabras` }),
      // `jugada` agrees with `palabra` exactly as `descifrada` does, which is why
      // the pair is two keys rather than one shared tail.
      playedBare: (count) => plural(count, { one: "jugada", other: "jugadas" }),
      playedOf: (count, pool) =>
        plural(count, { one: `jugada, de ${pool}`, other: `jugadas, de ${pool}` }),
      mostPlayed: "Más jugadas:",
      times: (count) => plural(count, { one: `${num(count)} vez`, other: `${num(count)} veces` }),
      breakdown: "Cómo caen las respuestas",
      solvedIn: (guesses) => `Resueltas en ${guesses}`,
      neverFound: "Nunca encontradas",
      favoriteRelics: "Reliquias favoritas",
      taken: (count) => `cogida ${num(count)}×`,
      noStreak: "Aún no has encontrado ninguna respuesta.",
      streakBest: (now) => `${now} seguidas, la racha más larga hasta ahora, y sigue viva.`,
      streakWithNow: (best, now) => `Racha más larga: ${best} seguidas, ${now} ahora.`,
      streak: (best) => `Racha más larga: ${best} seguidas.`,
    },

    ladder: {
      carrot: "Supera esto para desbloquear la ascensión 1",
      lower: "Bajar la ascensión",
      raise: "Subir la ascensión",
      locked: "Bloqueada",
      skipTo: (level) => `Saltar a la ascensión ${level}`,
      andBelow: "Y todas las reglas por debajo, también.",
      noRule: "El juego tal como está escrito, sin nada más que se te pida.",
      askTitle: (level) => `¿Saltar a la ascensión ${level}?`,
      ruleLabel: (name) => `${name}:`,
      askAndBelow: "Más todas las reglas por debajo.",
      intended: "Ganar en el nivel de abajo es la forma prevista de subir.",
      skipAnyway: (level) => `Saltar a ${level} igualmente`,
    },

    help: {
      title: "Cómo se juega",
      lead:
        "Adivina la palabra, como ya sabes hacerlo: verde es la letra correcta en el sitio " +
        "correcto, amarillo es la letra correcta en otro sitio.",
      scored: "La diferencia es que cada intento se puntúa.",
      chipsMult: {
        term: "Fichas × Mult",
        text: "Cada intento vale sus fichas multiplicadas por su mult.",
      },
      letters: {
        term: "Las letras pagan fichas",
        text:
          "Las letras raras pagan más. La tienda vende dos formas de subirlas: los grabados, " +
          "que suman a un tipo de letra, y los niveles de un tramo del alfabeto. Cada letra " +
          "está en exactamente un tramo, y las dos se acumulan.",
      },
      colors: {
        term: "Los colores pagan mult",
        text:
          "El verde vale +3 mult, el amarillo +1, el gris nada. Un intento lleno de gris no " +
          "vale casi nada, así que una prueba a la ligera te cuesta puntos de verdad.",
      },
      solving: {
        term: "Resolver multiplica la ronda",
        text:
          "Acierta la palabra y todo el montón acumulado en la ronda, no solo el intento que " +
          "la resolvió, se multiplica por 1 + los intentos que te quedaban. Después la ronda " +
          "termina de inmediato, hayas llegado al objetivo o no.",
      },
      farming:
        "Ese es el juego: cada intento que gastas cultivando hace crecer el montón, y " +
        "encoge el multiplicador que lo espera.",
      solveLine: {
        term: "Así que vigila la línea de resolución",
        text:
          "Debajo del tablero muestra el multiplicador que ganarías resolviendo ahora mismo, " +
          "y lo que ya vale el montón con él. Cuando se pone verde, resolver gana la ronda.",
      },
      runHeading: "La partida",
      target: {
        term: "Supera el objetivo",
        text: (stages, rounds) =>
          `${stages} fases de ${rounds} rondas. Quédate corto en el objetivo de una ronda y ` +
          "la partida acaba. Es la única forma de perder.",
      },
      endless: {
        term: "Y luego sigue, si te atreves",
        text: (stages) =>
          `Superar la fase ${stages} gana la partida, y puedes quedarte ahí o seguir hacia ` +
          "fases que nadie equilibró. Los objetivos siguen creciendo al mismo ritmo, y morir " +
          "ahí fuera no te quita la victoria.",
      },
      bosses: {
        term: "Jefes",
        text: "Cada tercera ronda dobla una regla. Léela antes de jugar.",
      },
      ascensions: {
        term: "Ascensiones",
        text: (authored) =>
          "El dial de dificultad de la pantalla de inicio, y el número que merece la pena " +
          `comparar. Los primeros ${authored} niveles añaden cada uno una regla permanente: ` +
          "a lo que puedes adivinar, a lo que paga una ronda, a lo altos que son los objetivos, " +
          "a cuántas reliquias puedes guardar, a cuántos intentos tienes. Por encima de eso la " +
          "escalera no acaba: cada nivel más vuelve a subir todos los objetivos. Ganar en uno " +
          "gana el siguiente, y subir peldaño a peldaño es la forma prevista, no un candado.",
      },
      money: {
        term: "Dinero",
        text: (payouts, perGuess, per, cap) =>
          `Las rondas pagan ${payouts}, más ${perGuess} por cada intento sin usar, más $1 de ` +
          `interés por cada ${per} que tengas, hasta ${cap}. Sentarse sobre el dinero es una ` +
          "estrategia.",
      },
      relics: {
        term: "Reliquias",
        text: (slots) =>
          `Hasta ${slots}, y actúan de izquierda a derecha, así que el orden en que las ` +
          "compras importa. Toca una para leerla.",
      },
      packs: {
        term: "Sobres",
        text:
          "Una ranura vende una elección en vez de una carta. Un sobre saca tres y te quedas " +
          "con una, gratis. El resto de la tienda espera a que elijas o te marches, y marcharse " +
          "no se queda con nada.",
      },
      mods: {
        term: "Modificadores de letra",
        text:
          "La tienda vende modificadores y tú eliges a qué letra se pega cada uno durante el " +
          "resto de la partida. Cada vez que juegas esa letra, hace esto. Una letra ×mult " +
          "multiplica lo que la palabra ha puntuado hasta donde ella está, así que vale más al " +
          "final de una palabra que al principio. Los sobres los reparten más baratos, con la " +
          "letra ya elegida.",
      },
      codexNote:
        "El códice tiene todas las reliquias, jefes, formas de palabra y modificadores del juego, en una lista completa.",
      openCodex: "Abrir el códice",
      gotIt: "Entendido",
    },

    shapes: {
      title: "Formas de palabra",
      scoresAs: (word, shape) => `${word.toUpperCase()} puntúa como ${shape}`,
      anyWord: "Todo intento tiene una forma.",
      note:
        "Un intento puntúa como la forma más rara con la que encaja, que es la primera de " +
        "estas con la que encaja. Subir de nivel una forma sube todos los intentos futuros de " +
        "esa forma. El nivel 1 no paga nada, así que un nivel es lo que hace que una forma " +
        "merezca la pena.",
      scoring: "puntúa",
      alsoMatches: "también encaja",
      payNow: (chips, mult) => `ahora +${chips} fichas, +${mult} mult`,
      payPerLevel: (chips, mult) => `+${chips} fichas, +${mult} mult por nivel`,
    },

    codex: {
      title: "Códice",
      lead: "Todo lo que hay en el juego, lo hayas visto o no.",
      relics: {
        title: "Reliquias",
        blurb: (slots) =>
          `Hasta ${slots} a la vez, actuando de izquierda a derecha, así que el orden en que ` +
          "las compras es parte de la construcción.",
      },
      bosses: {
        title: "Jefes",
        blurb:
          "Cada tercera ronda. Cada banda se saca sin reposición, así que una partida nunca " +
          "se encuentra dos veces al mismo jefe.",
      },
      ascensions: {
        title: "Ascensiones",
        blurb:
          "La dificultad de la propia partida, elegida antes de empezar y fija durante toda " +
          "ella. Una partida en un nivel juega todas las reglas hasta él, y ganar en uno " +
          "desbloquea el siguiente. Estas son las escritas; pasada la última, cada nivel sube " +
          "simplemente todos los objetivos otro 8\u00A0%, y no hay último nivel.",
      },
      shapes: {
        title: "Formas de palabra",
        blurb:
          "Todo intento puntúa como exactamente una forma: la más rara con la que encaja, que " +
          "es la primera de esta lista. Subir de nivel una forma sube todos los intentos " +
          "futuros de ella.",
      },
      mods: {
        title: "Modificadores de letra",
        blurb:
          "Pegados a una sola letra durante el resto de la partida. Uno por letra a la vez, y " +
          "el teclado lleva la marca. Una letra ×mult multiplica lo que la palabra ha puntuado " +
          "hasta donde ella está, así que vale más al final de una palabra que al principio. El " +
          "precio de la tienda compra la carta y te deja elegir la letra; un sobre la reparte " +
          "por el precio de al lado, con la letra ya elegida.",
      },
      upgrades: {
        title: "Mejoras de letra",
        blurb:
          "Dos vías que suman fichas a las letras, y se acumulan: los grabados suben un tipo " +
          "de letra, los rangos suben un tramo del alfabeto. Cada letra está en exactamente un " +
          "tramo.",
      },
      consumables: {
        title: "Consumibles",
        blurb: (slots) => `Se usan una vez, cuando quieras. Puedes llevar ${slots}.`,
      },
      packs: {
        title: "Sobres",
        blurb:
          "Se venden en una ranura propia. Un sobre saca sus cartas y te quedas con una, " +
          "gratis. La tienda espera a que elijas o te marches.",
      },
      rarity: {
        common: "común",
        uncommon: "poco común",
        rare: "rara",
        legendary: "legendaria",
      },
      tier: { early: "Inicio", mid: "Medio", late: "Final" },
      tierBand: (tier, first, last) => `${tier} · fases ${first}–${last}`,
      shapePer: (chips, mult) => `+${chips} / +${mult} por nivel`,
      modName: (name, pip) => `${name} ${pip}`,
      modText: (text) => `La letra ${text}.`,
      modTextOnly: (text, letters) => `La letra ${text}. Solo en ${letters}.`,
      packText: (text) => `${text}.`,
      packTextPicks: (text, picks) => `${text}, y te quedas ${picks}.`,
    },

    pause: {
      title: "En pausa",
      musicOn: "Música activada",
      musicOff: "Música desactivada",
      speed: (speed) => `Velocidad de animación ×${speed}`,
      language: "Idioma",
      wordsNextRun: "Las palabras cambian cuando empieces una partida nueva.",
      quit: "Abandonar partida",
      resume: "Continuar",
    },

    quit: {
      title: "¿Abandonar esta partida?",
      body: (stage, total, round) =>
        `Estás en la fase ${stage} de ${total}, ${round}. Abandonarla la borra. ` +
        "No hay vuelta atrás a esta partida.",
      bodyEndless: (stage, round) =>
        `Estás en la fase ${stage}, ${round}. Abandonarla la borra. ` +
        "No hay vuelta atrás a esta partida.",
      confirm: "Abandonar partida",
      cancel: "Seguir jugando",
    },

    coach: {
      chips:
        "Cada intento se puntúa. Escribe una palabra, y la línea de debajo del tablero cuenta lo que valen sus letras.",
      rare: (chips) =>
        `${chips} fichas por ahora. Las letras raras pagan más: la A vale 1, la K vale 5, la Z vale 10.`,
      mult: "El ? es el mult, y solo la respuesta lo sabe: verde +3, amarillo +1, gris nada. Pulsa INTRO para averiguarlo.",
      banked: (chips, mult, score, target) =>
        `${chips} × ${mult} = ${score}, guardado hacia ${target}. Cada intento suma al mismo montón.`,
      solve: (now, next) =>
        `Resolver multiplica todo el montón por ×${now}, no solo el intento que lo consigue, ` +
        `y termina la ronda. Un intento más y sería ×${next}.`,
    },
  },
}
