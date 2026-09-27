package safety

import (
	"regexp"
	"strings"
)

// The lexicon favors precision: it catches explicit phrasing instantly and
// leaves indirect phrasing to the model tier. Figures of speech ("this
// deadline is killing me") must never match.
var lexicon = []struct {
	cat Category
	re  *regexp.Regexp
}{
	{InDanger, re(`\b(hiding|hid|locked myself) (in|inside) (the|my|a) (bathroom|closet|bedroom|room|car)\b`)},
	{InDanger, re(`\b(he|she|they|partner|husband|wife|boyfriend|girlfriend|dad|mom|father|mother|ex)('s| is| are| keeps?)? (hitting|beating|choking|strangling|threatening) me\b`)},
	{InDanger, re(`\b(he|she|they)('s| is| are)? (going|trying|gonna) to (kill|hurt) me\b`)},
	{InDanger, re(`\b(he|she|they|partner|husband|wife|boyfriend|girlfriend|dad|mom|father|mother|ex) (just )?(hit|punched|choked|strangled|beat|kicked|slapped) me\b`)},
	{InDanger, re(`\bafraid (he|she|they)('ll| will| might| is going to| are going to) (kill|hurt) me\b`)},

	{Suicide, re(`\bkill(ing|ed)? my ?self\b`)},
	{Suicide, re(`\bun-?aliv(e|ing) (my ?self|me)\b`)},
	{Suicide, re(`\bsuicid(e|al)\b`)},
	{Suicide, re(`\b(end|ending|take|taking) (it all|my (own )?life|my life)\b`)},
	{Suicide, re(`\b(want|wanna|wanted|wish|wishing) (to )?(die|be dead|not be (here|alive)|not wake up|disappear forever)\b`)},
	{Suicide, re(`\b(don'?t|do not) want to (live|be alive|be here anymore|wake up)\b`)},
	{Suicide, re(`\b(everyone|they|people|world|family)('d| would) be better off (without me|if i (was|were)n'?t (here|around|alive)|if i (was|were) (dead|gone))\b`)},
	{Suicide, re(`\bbetter off dead\b`)},
	{Suicide, re(`\b(saving|saved|stockpiling|hoarding|collecting) (up )?(my |the |some )?(pills|meds|medication|tablets)\b`)},
	{Suicide, re(`\b(no|any) (reason|point) (to|in) (live|living|being alive|going on)\b`)},
	{Suicide, re(`\boverdos(e|ing) on\b`)},
	{Suicide, re(`\bkms\b`)},
	{Suicide, re(`\b(tired|sick) of (existing|being alive|living anymore)\b`)},

	{SelfHarm, re(`\bself[- ]?harm(ing|ed)?\b`)},
	{SelfHarm, re(`\b(cut|cutting|burned|burnt|burning) (my ?self|my (arms?|legs?|wrists?|thighs?|skin))\b`)},
	{SelfHarm, re(`\b(want|wants|wanting|wanna|urge|urges|tempted|going|thinking about|thought about) (to )?(hurt|harm|cut|burn|hit|punish) my ?self\b`)},
	{SelfHarm, re(`\b(hurt|hurting|harm|harming|hit|hitting) my ?self (on purpose|again)\b`)},
	{SelfHarm, re(`\burges? to (cut|burn|self[- ]?harm|hurt my ?self)\b`)},

	{HarmOthers, re(`\b(want|wanna|going|gonna|planning|plan) (to )?(seriously |really |actually )?(hurt|harm|kill|stab|shoot|strangle) (him|her|them|someone|somebody|people|my \w+)\b.{0,40}\b(actually|really|seriously|not (just )?(joking|kidding|yell))`)},
	{HarmOthers, re(`\b(want|wanna|going|gonna|planning|plan) (to )?(stab|shoot|strangle) (him|her|them|someone|somebody|people|my \w+)\b`)},
	// A bare "gonna kill my brother" is usually venting; planning isn't.
	{HarmOthers, re(`\b(planning|plan|plotting) (to|how to) kill (him|her|them|someone|somebody|people|my \w+)\b`)},
	{HarmOthers, re(`\bthoughts? (of|about) (hurting|harming|killing) (him|her|them|someone|somebody|people|my \w+)\b`)},
}

// benign strips ordinary phrases that contain lexicon words before matching.
var benign = []*regexp.Regexp{
	re(`\b(accidentally|by accident) (cut|burned|burnt|hurt) my ?self\b`),
	re(`\b(cut|burned|burnt|hurt) my ?self (shaving|cooking|chopping|on|while|when|at|by accident|accidentally)\b`),
	re(`\b(game|movie|film|book|show|character|episode|novel)\b.{0,60}\bsuicid(e|al)\b`),
	re(`\bsuicid(e|al)\b.{0,20}\b(squad|mission|run|doors?|lane)\b`),
	re(`\bsuicide (prevention|awareness)\b`),
	re(`\b(stopped|quit) self[- ]?harm(ing)?\b`),
}

func re(s string) *regexp.Regexp { return regexp.MustCompile(`(?i)` + s) }

// normalize lowercases, straightens quotes and collapses whitespace.
func normalize(s string) string {
	s = strings.NewReplacer("’", "'", "‘", "'", "`", "'").Replace(strings.ToLower(s))
	return strings.Join(strings.Fields(s), " ")
}

// Match runs the lexicon tier alone.
func Match(text string) Result {
	s := normalize(text)
	for _, b := range benign {
		s = b.ReplaceAllString(s, " ")
	}
	for _, e := range lexicon {
		if m := e.re.FindString(s); m != "" {
			return Result{Category: e.cat, Source: SourceLexicon, Reason: m}
		}
	}
	return Result{Category: None}
}
