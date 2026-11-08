# OpenCycle

A free, open cycling sim, starting with what makes one worth riding: the world. The ride is a
fictional Japanese countryside in Unreal Engine 5.8: a narrow bike path through paddies, a small
town and jungle, by a mountain and the sea, with weather that changes along the way. Power comes
from the keyboard or a simulated smart trainer for now, and real Bluetooth trainers are next.

Everything lives in [`unreal/`](unreal/README.md): the C++ module, the scripts that generate and
build the ride, and the tests. The working Unreal project is outside git, at
`~/.cache/opencycle-neural/ue`, because its content runs to tens of gigabytes.

## Play it

```sh
unreal/scripts/game.sh        # the packaged build that unreal/scripts/package.sh made
```

Esc, P or Space pauses. The start and pause menus hold the video settings.

An earlier version of OpenCycle, a neon city and a forest in Godot 4, is in the git history before
it was removed.

## License

See [LICENSE](LICENSE).
