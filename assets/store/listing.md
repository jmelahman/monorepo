# Play Store listing — 5 Wild

Draft copy. Paste into Play Console › Grow users › Store presence › Main store listing.
Character limits are Google's; the counts in brackets are what the text below uses.

## App name (30 max) [6 used]

5 Wild

## Short description (80 max) [76 used]

A word-guessing roguelike. Solve the word, spend the gold, climb the ladder.

## Full description (4000 max) [1669 used once unwrapped]

Guess a five-letter word in six tries. That part you know.

What you may not know is what the word is worth. Every letter has a chip value,
every guess scores whether it is right or wrong, and a round is not won by
finding the answer — it is won by clearing a target. Find the word too early and
you may not have scored enough to survive it.

Between rounds there is a shop. Relics that change the arithmetic, modifiers
bolted onto individual letters, word shapes that pay for the patterns you spell,
consumables held in reserve, packs that deal you a choice of three. Gold comes
from clearing rounds and from the guesses you did not need. Spend it or bank it;
both are wrong sometimes.

Eight stages, and every third round is a boss that breaks one of the rules you
had been relying on. The Margin stops the first and last letters scoring. The
Drought thins the shop. The Silence takes the keyboard's help away. Fifteen of
them, and you do not get to pick which one you meet.

Clear a run and the ladder opens. Ten ascensions, each adding a rule that is
strictly worse for you than the one before, and the game keeps a record of how
far up you have been.

- 28 relics, 15 bosses, 10 ascensions, 9 letter modifiers, 8 letter upgrades,
  5 word shapes, 4 consumables, 3 pack types
- Runs of roughly fifteen to forty minutes, saved as you go
- Plays entirely offline. No ads, no purchases, no account, no network calls
- English, Spanish, French and German word lists
- Adjustable animation speed, a reduced-motion mode, and a plain mode that
  turns off the scoring flourishes if you would rather just read the numbers

Free, and finished. There is nothing to buy inside it.

## Notes for whoever pastes this

- The paragraphs are hard-wrapped here to stay readable in a diff. Play renders
  newlines literally, so unwrap each paragraph to a single line on paste, and
  keep only the blank lines between them. The bullets keep their line breaks;
  the hyphens are literal, since Play does not render markdown. The count above
  is what Play counted after unwrapping, which is four short of the wrapped
  source: the newlines this file carries mid-paragraph become nothing.
- Pasting the description into the console field is not enough on its own if it
  is done programmatically. The form is Angular, and setting a textarea's value
  in JS updates what is on the screen without updating the form control behind
  it, so the field reads full and validates empty — "Add a full description for
  your app" in red under 1669/4000. One real keystroke in the field syncs it.
  The cleaner fix, if the paste has to be scripted at all, is to assign through
  the prototype's own setter and then dispatch the event by hand:

      const set = Object.getOwnPropertyDescriptor(
        HTMLTextAreaElement.prototype, 'value').set;
      set.call(ta, text);
      ta.dispatchEvent(new Event('input', { bubbles: true }));

  Angular overwrites the element's `value` property with its own accessor to
  notice writes, so `ta.value = x` goes to the framework's shadow of the field
  and a plain `dispatchEvent` afterwards reports the *old* value. Reaching past
  it to the prototype setter writes the real one, and the event then carries
  what is actually in the box. This is what filled the release notes on the
  closed-testing track in one step, with the counter reading "provided for 1
  language" and no keystroke needed.
- The three named bosses are accurate to the current build. If a boss is
  renamed or its rule changes, fix it here — a store listing that describes
  mechanics the app does not have is a listing problem, not a copy problem.
- The counts in the first bullet come from the Codex screen and should be
  re-read off it after any content change.
- "No ads, no purchases, no account, no network calls" has to stay true. It is
  also what the Data safety declaration says, so the two move together.
