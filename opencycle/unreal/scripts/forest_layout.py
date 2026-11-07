"""Shared centreline for the forest road, vegetation offsets and ride camera.

Distances are Unreal centimetres and angles are degrees. +X is forward and a positive
lateral offset is to the road's left (+Y at the straight start). The module deliberately
has no Unreal dependency so its geometry can be tested with system Python.

Integration notes for the level builder (make_forest.py), for when it switches from one
straight plane to segmented geometry; the ride camera already calls these:

    road = RoadLayout(amplitude=..., wavelength=...)  # OC_CURVE, OC_WAVELENGTH

    # Road and gravel shoulders as a chain of plane tiles. Each tile follows the chord
    # between two centreline samples; stretch it ~2 cm along its own X to hide the gaps.
    for centre, yaw, span in road.segments(0, LENGTH, SEGMENT):
        # plane at (*centre, 1), rotator (0, yaw, 0), scale (span / 100 + 0.02, ROAD / 100, 1)
    for side in (-1, 1):
        for centre, yaw, span in road.segments(0, LENGTH, SEGMENT, side * (ROAD / 2 + 35)):
            # plane at (*centre, 0.7), rotator (0, yaw, 0), scale (span / 100 + 0.02, 0.7, 1)

    # Trees and undergrowth keep their existing scatter; only their world (x, y) moves,
    # from side * (ROAD / 2 + ...) to the same offset curve, which stays heading-free.
    px, py = road.point(x, side * (ROAD / 2 + verge(x, side) + back))
    px, py = road.point(x, side * (ROAD / 2 + distance))
"""

import math
from dataclasses import dataclass


@dataclass(frozen=True)
class RoadLayout:
    """A gentle sine-wave road with zero offset and heading at x=0."""

    amplitude: float = 1200.0
    wavelength: float = 45000.0

    def __post_init__(self):
        if self.amplitude < 0:
            raise ValueError("amplitude must be non-negative")
        if self.wavelength <= 0:
            raise ValueError("wavelength must be positive")

    @property
    def _wave_number(self):
        return 2.0 * math.pi / self.wavelength

    def centre_y(self, x):
        """World-Y centre of the road at world X.

        A quarter-wavelength phase offset makes the initial tangent horizontal while
        keeping both the offset and tangent exactly continuous.
        """
        return self.amplitude * (math.sin(self._wave_number * x + math.pi / 2.0) - 1.0)

    def heading_deg(self, x):
        """Unreal yaw of the centreline tangent at x."""
        slope = self.amplitude * self._wave_number * math.cos(self._wave_number * x + math.pi / 2.0)
        return math.degrees(math.atan(slope))

    def point(self, x, lateral=0.0):
        """Point on the centreline, offset along its local left normal."""
        heading = math.radians(self.heading_deg(x))
        return (
            x - lateral * math.sin(heading),
            self.centre_y(x) + lateral * math.cos(heading),
        )

    def segment_pose(self, x, length, lateral=0.0):
        """Return ((x, y), yaw) for a road or shoulder segment centred at x.

        `lateral` moves the segment onto the verge, so shoulder strips follow the same bend
        as the road with their width unchanged across their own local Y. The segment's +X
        axis follows the chord between its (offset) centreline endpoints. When integrating
        with plane meshes, add roughly 2 cm of length to hide tiny chord gaps.
        """
        if length <= 0:
            raise ValueError("segment length must be positive")
        start = self.point(x - length / 2.0, lateral)
        end = self.point(x + length / 2.0, lateral)
        midpoint = ((start[0] + end[0]) / 2.0, (start[1] + end[1]) / 2.0)
        yaw = math.degrees(math.atan2(end[1] - start[1], end[0] - start[0]))
        return midpoint, yaw

    def segments(self, x_start, x_end, length, lateral=0.0):
        """Tile the road or a shoulder strip from x_start to x_end with chained segments.

        Yields ((x, y), yaw, span) per tile, in order; each tile starts where the previous
        one ends and the last one is whatever length is left, so spans sum to x_end -
        x_start. Stretch each tile ~2 cm along its own X when spawning plane meshes.
        """
        if length <= 0:
            raise ValueError("segment length must be positive")
        x = x_start
        while x < x_end:
            span = min(length, x_end - x)
            yield (*self.segment_pose(x + span / 2.0, span, lateral), span)
            x += span

    def camera_pose(self, x, height_cm=160.0, pitch_deg=0.0, lateral=0.0):
        """Return ((x, y, z), (pitch, yaw, roll)) in Unreal rotator order."""
        px, py = self.point(x, lateral)
        return (px, py, height_cm), (pitch_deg, self.heading_deg(x), 0.0)

    def world_pose(
        self, x, height_cm=160.0, pitch_deg=0.0, lateral=0.0, origin=(0.0, 0.0), base_yaw=0.0
    ):
        """camera_pose moved into a level frame that owns the road's start.

        The road frame starts at the level origin running along +X; `base_yaw` turns it and
        `origin` shifts it, so a PlayerStart can lead the ride off at any heading while the
        bend still comes from the same centreline that lays out the level. With amplitude 0
        this is exactly the old straight ride: origin + heading * x at a constant height.
        """
        px, py = self.point(x, lateral)
        angle = math.radians(base_yaw)
        cos, sin = math.cos(angle), math.sin(angle)
        position = (origin[0] + px * cos - py * sin, origin[1] + px * sin + py * cos, height_cm)
        rotation = (pitch_deg, base_yaw + self.heading_deg(x), 0.0)
        return position, rotation
