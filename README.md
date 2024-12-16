# Nature Sounds

[![Test status](https://github.com/jmelahman/nature-sounds/actions/workflows/test.yml/badge.svg)](https://github.com/jmelahman/nature-sounds/actions)
[![Deploy Status](https://github.com/jmelahman/nature-sounds/actions/workflows/release.yml/badge.svg)](https://github.com/jmelahman/nature-sounds/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/jmelahman/nature-sounds.svg)](https://pkg.go.dev/github.com/jmelahman/nature-sounds)
[![Arch User Repsoitory](https://img.shields.io/aur/version/nature-sounds)](https://aur.archlinux.org/packages/nature-sounds)
[![PyPI](https://img.shields.io/pypi/v/nature-sounds.svg)]()
[![Go Report Card](https://goreportcard.com/badge/github.com/jmelahman/nature-sounds)](https://goreportcard.com/report/github.com/jmelahman/nature-sounds)

A lightweight, nature sounds player for the command-line.

<p align="center">
  <picture align="center">
    <source media="(prefers-color-scheme: dark)" srcset="https://github.com/jmelahman/nature-sounds/blob/master/demo_dark.png">
    <source media="(prefers-color-scheme: light)" srcset="https://github.com/jmelahman/nature-sounds/blob/master/demo_light.png">
    <img alt="Welcome to nature-sounds (v0.1.0). Press ? for a list of commands." src="https://github.com/jmelahman/nature-sounds/blob/master/demo_light.png">
  </picture>
</p>

`nature-sounds` uses sounds from the National Park Service's [A Symphony of Sounds](https://www.nps.gov/subjects/sound/index.htm).
Previews of the sounds are available from the [Yellowstone](https://www.nps.gov/yell/learn/photosmultimedia/sounds-soundscapes.htm) or [Rocky Mountain National Park](https://www.nps.gov/romo/learn/photosmultimedia/sounds-ambient-soundscapes.htm) soundscapes collections.
If you enjoy these sounds, consider [donating to the NPS](https://www.nps.gov/getinvolved/donate.htm).
All rights are reserved by the respective owners.

## Install

**AUR:**

`nature-sounds` is available from the [Arch User Repository](https://aur.archlinux.org/packages/nature-sounds).

```shell
yay -S nature-sounds
```

**pip:**

`nature-sounds` is available as a [pypi package](https://pypi.org/project/nature-sounds/).

```shell
pip install nature-sounds
```

**go:**

```shell
go install github.com/jmelahman/nature-sounds@latest
```

**github:**

Prebuilt packages are available from [Github Releases](https://github.com/jmelahman/nature-sounds/releases).

## Credits

`nature-sounds`'s UI is inspired by my favorite command-line tool, [pianobar](https://github.com/PromyLOPh/pianobar).
