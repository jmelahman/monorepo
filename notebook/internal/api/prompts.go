package api

import (
	"fmt"
	"strings"
)

// WordsPerMinute is the speaking rate scripts are written to.
const WordsPerMinute = 150

const discoverSystem = `You are a research librarian. Find sources on the web for someone who will
turn them into a podcast episode.

Search widely, then choose the best sources:
- Prefer substantive, primary, or expert sources: original reporting, papers'
  plain-language pages, official documentation, long-form explainers, and
  well-regarded blogs. Avoid content farms, listicles, SEO pages, and
  aggregators that only summarize others.
- Cover the topic from different angles; don't pick five pages that say the
  same thing.
- Each source must be a web page whose text can be read: no videos, podcasts,
  paywalled articles, login walls, or PDFs unless there is no alternative.
- Open each candidate with WebFetch before you choose it, so you know the
  address works and the page says what you claim it says.

In "why", say in one sentence what this source adds that the others don't.`

var discoverSchema = []byte(`{
  "type": "object",
  "properties": {
    "sources": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "title": {"type": "string"},
          "url": {"type": "string"},
          "why": {"type": "string"}
        },
        "required": ["title", "url", "why"]
      }
    }
  },
  "required": ["sources"]
}`)

func discoverPrompt(topic string, count int, exclude []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Find %d sources on this topic:\n\n%s\n", count, topic)
	if len(exclude) > 0 {
		b.WriteString("\nThe notebook already has these; don't suggest them again:\n")
		for _, u := range exclude {
			b.WriteString("- " + u + "\n")
		}
	}
	return b.String()
}

// Source is a notebook source as the page sends it.
type Source struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url,omitempty"`
	Text  string `json:"text"`
}

// sourcesBlock numbers the sources from 1, as the page shows them.
func sourcesBlock(sources []Source) string {
	var b strings.Builder
	for i, s := range sources {
		fmt.Fprintf(&b, "<source n=\"%d\" title=%q", i+1, s.Title)
		if s.URL != "" {
			fmt.Fprintf(&b, " url=%q", s.URL)
		}
		fmt.Fprintf(&b, ">\n%s\n</source>\n\n", strings.TrimSpace(s.Text))
	}
	return b.String()
}

// Host is one of the episode's two hosts.
type Host struct {
	Name  string `json:"name"`
	About string `json:"about,omitempty"`
}

const scriptSystem = `You write scripts for a podcast in which two hosts talk through a set of
sources, like NotebookLM's audio overviews but better. The script is read
aloud by a text-to-speech voice for each host, word for word.

Make it worth listening to:
- A real conversation, not two people taking turns reading an essay. One host
  usually leads a thread and the other pushes on it: asks the question a
  listener would, offers an example, disagrees, connects it to something
  earlier. Swap who leads.
- Mix short turns with longer explanations. A turn is one to five sentences,
  seldom more.
- Concrete before abstract: specific numbers, names, cases, and quotes from
  the sources, then what they mean.
- Accurate to the sources. Say where a claim comes from when it matters ("the
  survey found", "according to the paper's authors"). Where sources disagree,
  say so. Don't invent facts, quotes, or statistics.
- No filler: skip "great question", "absolutely", "fascinating", "let's dive
  in", "buckle up", and exaggerated enthusiasm. Light humor is fine when it
  arises naturally.

Write for the ear and for text-to-speech:
- Plain spoken English. No Markdown, emoji, stage directions, sound effects,
  speaker labels inside the text, or bracketed asides.
- Spell out what a voice would say: "about forty percent", "twenty twenty-four",
  "doctor", "for example". Expand an acronym the first time it is used. Never
  read out a URL.
- Use punctuation for rhythm: commas and full stops, an occasional dash or
  question. A short reaction ("Right.", "Wait, really?") can be its own turn,
  but don't overuse them.`

var outlineSchema = []byte(`{
  "type": "object",
  "properties": {
    "title": {"type": "string", "description": "The episode's title, under 70 characters"},
    "description": {"type": "string", "description": "Two or three sentences for a podcast app"},
    "segments": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "heading": {"type": "string"},
          "plan": {"type": "string", "description": "What this segment covers and which sources it draws on"},
          "minutes": {"type": "number"}
        },
        "required": ["heading", "plan", "minutes"]
      }
    }
  },
  "required": ["title", "description", "segments"]
}`)

var linesSchema = []byte(`{
  "type": "object",
  "properties": {
    "lines": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "speaker": {"type": "string", "enum": ["A", "B"]},
          "text": {"type": "string"}
        },
        "required": ["speaker", "text"]
      }
    }
  },
  "required": ["lines"]
}`)

// ScriptRequest asks for an episode script.
type ScriptRequest struct {
	Notebook string   `json:"notebook"`
	Sources  []Source `json:"sources"`
	Minutes  float64  `json:"minutes"`
	Focus    string   `json:"focus"`
	Style    string   `json:"style"`
	Hosts    [2]Host  `json:"hosts"`
	Model    string   `json:"model"`
	Effort   string   `json:"effort"`
}

func hostsBlock(hosts [2]Host) string {
	var b strings.Builder
	for i, h := range hosts {
		fmt.Fprintf(&b, "- Speaker %c is %s", 'A'+i, h.Name)
		if h.About != "" {
			b.WriteString(": " + h.About)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func outlinePrompt(req ScriptRequest) string {
	var b strings.Builder
	b.WriteString("The sources:\n\n")
	b.WriteString(sourcesBlock(req.Sources))
	fmt.Fprintf(&b, "Plan an episode of about %g minutes (about %d words of dialogue) on these sources.\n\n", req.Minutes, int(req.Minutes*WordsPerMinute))
	b.WriteString("The hosts:\n" + hostsBlock(req.Hosts) + "\n")
	if req.Focus != "" {
		b.WriteString("What to focus on: " + req.Focus + "\n\n")
	}
	if req.Style != "" {
		b.WriteString("Style: " + req.Style + "\n\n")
	}
	b.WriteString(`Break it into segments, each a few minutes long, whose minutes add up to the
episode's length. The first segment opens the show: the hosts introduce
themselves by first name only and say what the episode is about and why it's
worth the listener's time. The last segment closes it with the takeaways. In
between, build an arc: start from what a curious newcomer needs, and go deeper.
In each segment's plan, name the specific facts, examples, and tensions from
the sources it will use, so that no two segments repeat each other.`)
	return b.String()
}

func segmentPrompt(i int, seg Segment, total int, prevTail []Line, hosts [2]Host) string {
	var b strings.Builder
	words := int(seg.Minutes * WordsPerMinute)
	fmt.Fprintf(&b, "Write segment %d of %d, %q: about %d words of dialogue.\n\nIts plan: %s\n\n", i+1, total, seg.Heading, words, seg.Plan)
	switch {
	case i == 0:
		b.WriteString("This opens the episode.\n")
	case len(prevTail) > 0:
		b.WriteString("Continue straight on from where the conversation left off, without a recap or a fresh greeting. It ended:\n\n")
		for _, l := range prevTail {
			fmt.Fprintf(&b, "%s: %s\n", hostName(hosts, l.Speaker), l.Text)
		}
		b.WriteString("\n")
	}
	if i == total-1 {
		b.WriteString("This closes the episode: wrap up with the takeaways and a brief goodbye.\n")
	} else {
		b.WriteString("Don't wrap up or say goodbye; more segments follow. Don't announce the segment's heading.\n")
	}
	b.WriteString("Speaker is \"A\" or \"B\" as in the outline; the text is only what is said aloud.")
	return b.String()
}

func hostName(hosts [2]Host, speaker string) string {
	if speaker == "B" {
		return hosts[1].Name
	}
	return hosts[0].Name
}
