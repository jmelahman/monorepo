"""Roads as meshes for the generated worlds: a cross-section swept along a centreline.

Used by make_trial_world.py and make_island.py, with system Python.
"""

import itertools
import math

import gltf_writer


def section(half, bank=0.0):
    """Across the road, in cm: offset from the centre and height under the centreline. The
    tarmac is 2 cm under it; a 20 cm skirt each side drops below the ground so the edge doesn't
    float. A `bank` (cm) carries the skirt on down at about 30 degrees, for ground too coarse to
    meet the road: where it sits low, the road stands on an earth bank, not a slab with a shadow
    under its edge."""
    inner = ((-half - 20, -12.0), (-half, -2.0), (half, -2.0), (half + 20, -12.0))
    if not bank:
        return inner
    out = 20 + bank * 1.7
    return ((-half - out, -12.0 - bank), *inner, (half + out, -12.0 - bank))


def road_pieces(centre, half, closed=False, piece=200, bank=0.0, wear=None):
    """The road as meshes, one per `piece` centreline points (a metre apart), each in world
    coordinates. `centre` is (x, y, z) in cm; UVs are in metres, u across and v along, for
    markings later. A closed road joins its end to its start. `wear`, 0 (kept like new) to 1
    (old country tarmac), one per centre point, goes in the vertex colour for the road
    material (climb_world.road_material); without it every vertex is 1, the old look."""
    across = section(half, bank)
    points = list(centre) + ([centre[0]] if closed else [])
    pieces = []
    for start in range(0, len(points) - 1, piece):
        chunk = points[start : start + piece + 1]
        mesh = gltf_writer.Mesh()
        part = mesh.part("Road")
        rows = []
        for i, (x, y, z) in enumerate(chunk):
            k = start + i
            w = 1.0 if wear is None else float(wear[k % len(centre)])
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
                        (w, w, w, 1.0),
                    )
                    for off, dz in across
                ]
            )
        for r0, r1 in itertools.pairwise(rows):
            for j in range(len(across) - 1):
                part.quad(r0[j], r0[j + 1], r1[j + 1], r1[j])
        pieces.append(mesh)
    return pieces
