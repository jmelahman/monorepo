# The neon street: what counts as real

The goal is a street that a viewer can't tell from footage. This is the list a frame is
scored against, written before the art so the art can't quietly move the goalposts. It was
written from these photos of Mong Kok, Hong Kong, at night:

- [Shantung Street neon signs](https://commons.wikimedia.org/wiki/File:Mong_Kok_Neon_Signs_Night_(48127904386).jpg),
  the main reference.
- [Mong Kok nightlife 2](https://commons.wikimedia.org/wiki/File:Mong_Kok_nightlife_2.jpg),
  for building massing and crowds.
- [Night bus, Mong Kok](https://commons.wikimedia.org/wiki/File:Night_bus_Mongkok_-_Flickr_-_Franck_Michel.jpg).
- [The rest of the category](https://commons.wikimedia.org/wiki/Category:Mong_Kok).

The photos stay out of the repo; their licences are per file.

## What the photos show that a first guess gets wrong

- It is **bright at street level**. Shops pour white light over the pavement, and faces and
  clothes are well exposed. It is not a dark street with coloured highlights.
- It is **not very wet**. Damp, patched asphalt with a soft sheen, not a mirror.
- **Signs face the viewer.** They hang out over the road on steel frames, overlap one another
  and fill the view above 4 m. They are the architecture.
- **Sign text is real**: big readable characters over small print, floor numbers and phone
  numbers, LED-dot borders, lightboxes and printed boards side by side.
- **Shops are full**: goods from floor to ceiling, spilling out onto the pavement.
- **Everything is attached to something**: poles, frames, brackets, railings, conduit.
- **The street is messy**: barriers, crates, road signs, cracked patches, painted arrows.
- **People blur.** A real exposure smears walkers into streaks. Nobody sees a sharp face.

## Scoring a frame

Score each 0 (absent), 1 (reads as a game), 2 (holds up at a glance), 3 (matches the photo).
A frame passes at 30 of 39 with no zero, and passing frames still need the clip test below.

| # | Item | What a 3 looks like |
|---|------|---------------------|
| 1 | Exposure | Street level bright, sky dark, faces and clothes readable |
| 2 | Sign density | Signs fill the upper view and overlap in depth |
| 3 | Sign content | Real characters and words, several styles, nothing repeated in view |
| 4 | Sign mounting | Every sign has a visible frame, bracket or wall behind it |
| 5 | Shopfronts | Lit interiors with depth, goods to the ceiling, each business distinct |
| 6 | Street-level clutter | Railings, crates, poles, barriers, nothing evenly spaced |
| 7 | Road surface | Patches, cracks, paint and a broken sheen, not one material |
| 8 | Pavement and kerb | A real kerb step, paving pattern, drains, stains |
| 9 | Upper facades | Tile or plaster, recessed windows, rooms behind them, air-con units, grime |
| 10 | Massing | Buildings of different ages and heights, a skyline, depth down the street |
| 11 | Light | Contact shadows, coloured spill on nearby surfaces, no floating glow |
| 12 | People | Density, variety of clothes and height, walking believably |
| 13 | Variety | Nothing obviously repeats within a screen or within ten seconds of riding |

## Checking motion

Stills lie. A capture is also checked for shimmer on signs and window grids, popping as
things appear, flicker in reflections, sliding feet, and anything repeating in time.

## The real test

Mix ten-second clips of the render with real footage and ask people who haven't seen the
project which are real. Everything above only predicts this result.

Fallback if it fails: a deliberately film-like stylised look. If it passes only on a stronger
GPU, the owner chooses between realism and their hardware; the Radeon 890M at 30 fps stays
the requirement until then.
