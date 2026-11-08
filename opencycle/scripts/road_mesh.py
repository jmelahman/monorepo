"""Roads as meshes for the generated worlds: a cross-section swept along a centreline.

Used by make_trial_world.py and make_island.py, with system Python.
"""

import itertools
import math

import gltf_writer


def section(half):
    """Across the road, in cm: offset from the centre and height under the centreline. The
    tarmac is 2 cm under it; a 20 cm skirt each side drops below the ground so the edge doesn't
    float."""
    return ((-half - 20, -12.0), (-half, -2.0), (half, -2.0), (half + 20, -12.0))


def road_pieces(centre, half, closed=False, piece=200):
    """The road as meshes, one per `piece` centreline points (a metre apart), each in world
    coordinates. `centre` is (x, y, z) in cm; UVs are in metres, u across and v along, for
    markings later. A closed road joins its end to its start."""
    across = section(half)
    points = list(centre) + ([centre[0]] if closed else [])
    pieces = []
    for start in range(0, len(points) - 1, piece):
        chunk = points[start : start + piece + 1]
        mesh = gltf_writer.Mesh()
        part = mesh.part("Road")
        rows = []
        for i, (x, y, z) in enumerate(chunk):
            k = start + i
            a = points[k - 1] if k > 0 else (points[-2] if closed else points[0])
            b = points[k + 1] if k + 1 < len(points) else (points[1] if closed else points[-1])
            dx, dy = b[0] - a[0], b[1] - a[1]
            length = math.hypot(dx, dy) or 1.0
            # Left of travel, in UE's left-handed axes (X forward, Y right).
            side = (dy / length, -dx / length)
            rows.append(
                [
                    part.vertex(
                        (x + side[0] * off, y + side[1] * off, z + dz),
                        (0, 0, 1),
                        (off / 100.0, k * 1.0),
                    )
                    for off, dz in across
                ]
            )
        for r0, r1 in itertools.pairwise(rows):
            for j in range(len(across) - 1):
                part.quad(r0[j], r0[j + 1], r1[j + 1], r1[j])
        pieces.append(mesh)
    return pieces
