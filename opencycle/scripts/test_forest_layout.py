import itertools
import math
import unittest

from forest_layout import RoadLayout


class RoadLayoutTests(unittest.TestCase):
    def test_start_pose_preserves_existing_camera(self):
        position, rotation = RoadLayout().camera_pose(0, height_cm=150)
        self.assertEqual(position, (0.0, 0.0, 150.0))
        self.assertAlmostEqual(rotation[0], 0.0)
        self.assertAlmostEqual(rotation[1], 0.0)
        self.assertAlmostEqual(rotation[2], 0.0)

    def test_centre_and_heading_are_continuous(self):
        road = RoadLayout()
        for x in (0, 1000, 10000, 40000):
            self.assertAlmostEqual(road.centre_y(x - 0.001), road.centre_y(x + 0.001), delta=0.001)
            self.assertAlmostEqual(
                road.heading_deg(x - 0.001), road.heading_deg(x + 0.001), delta=0.001
            )

    def test_lateral_offset_is_normal_to_tangent(self):
        road = RoadLayout()
        x = 12345
        centre = road.point(x)
        left = road.point(x, 500)
        distance = math.dist(centre, left)
        self.assertAlmostEqual(distance, 500)
        heading = math.radians(road.heading_deg(x))
        expected = (centre[0] - 500 * math.sin(heading), centre[1] + 500 * math.cos(heading))
        self.assertAlmostEqual(left[0], expected[0])
        self.assertAlmostEqual(left[1], expected[1])

    def test_segment_pose_follows_chord(self):
        road = RoadLayout()
        x, length = 7000, 2000
        start = (x - length / 2, road.centre_y(x - length / 2))
        end = (x + length / 2, road.centre_y(x + length / 2))
        midpoint, yaw = road.segment_pose(x, length)
        self.assertAlmostEqual(midpoint[0], (start[0] + end[0]) / 2)
        self.assertAlmostEqual(midpoint[1], (start[1] + end[1]) / 2)
        expected_yaw = math.degrees(math.atan2(end[1] - start[1], end[0] - start[0]))
        self.assertAlmostEqual(yaw, expected_yaw)

    def test_zero_amplitude_is_straight(self):
        road = RoadLayout(amplitude=0)
        self.assertEqual(road.point(2000, 300), (2000, 300))
        self.assertEqual(road.heading_deg(2000), 0)
        self.assertEqual(road.segment_pose(2000, 1000), ((2000, 0), 0))

    def test_invalid_inputs(self):
        with self.assertRaises(ValueError):
            RoadLayout(amplitude=-1)
        with self.assertRaises(ValueError):
            RoadLayout(wavelength=0)
        with self.assertRaises(ValueError):
            RoadLayout().segment_pose(0, 0)
        with self.assertRaises(ValueError):
            list(RoadLayout().segments(0, 1000, 0))

    def polyline_length(self, road, x0, x1, step):
        """Arc length of the centreline, sampled every `step` cm."""
        count = max(1, int((x1 - x0) / step))
        points = [road.point(x0 + (x1 - x0) * i / count) for i in range(count + 1)]
        return sum(math.dist(a, b) for a, b in itertools.pairwise(points))

    def test_camera_rides_the_centreline(self):
        road = RoadLayout(amplitude=1500, wavelength=30000)
        height, pitch = 150.0, -3.0
        start = road.camera_pose(0, height_cm=height, pitch_deg=pitch)
        self.assertEqual(start[0], (0.0, 0.0, height))
        self.assertAlmostEqual(start[1][0], pitch)
        self.assertAlmostEqual(start[1][1], 0.0)
        self.assertAlmostEqual(start[1][2], 0.0)
        # Drawn as short straight moves, the camera walks the centreline's own length.
        travelled, previous = 0.0, None
        for x in range(0, 20001, 1000):
            position, rotation = road.camera_pose(x, height_cm=height, pitch_deg=pitch)
            self.assertEqual(position[2], height)
            self.assertAlmostEqual(rotation[0], pitch)
            self.assertAlmostEqual(rotation[2], 0.0)
            if previous is not None:
                travelled += math.dist(previous, (position[0], position[1]))
                self.assertGreater(math.dist(previous, (position[0], position[1])), 0)
            previous = (position[0], position[1])
        arc = self.polyline_length(road, 0, 20000, 1)
        self.assertAlmostEqual(travelled, arc, delta=arc * 0.001)

    def test_heading_matches_the_tangent(self):
        road = RoadLayout(amplitude=1200, wavelength=45000)
        h = 0.01
        for x in (0, 3123, 11250, 33750, 40000):
            slope = (road.centre_y(x + h) - road.centre_y(x - h)) / (2 * h)
            self.assertAlmostEqual(road.heading_deg(x), math.degrees(math.atan(slope)), places=6)
        # A segment-scale chord leans by nearly the same yaw, so segments follow the tangent.
        chord = (road.centre_y(33750 + 1000) - road.centre_y(33750 - 1000)) / 2000
        self.assertAlmostEqual(road.heading_deg(33750), math.degrees(math.atan(chord)), delta=0.05)
        # The bend peaks where the sine's slope is steepest, and a shorter wavelength bends harder.
        self.assertAlmostEqual(
            road.heading_deg(0.75 * 45000),
            math.degrees(math.atan(1200 * 2 * math.pi / 45000)),
            places=4,
        )
        self.assertGreater(
            RoadLayout(amplitude=1200, wavelength=20000).heading_deg(0.75 * 20000),
            road.heading_deg(0.75 * 45000),
        )

    def test_world_pose_defaults_to_the_road_frame(self):
        road = RoadLayout(amplitude=900)
        position, rotation = road.world_pose(7777, height_cm=151, pitch_deg=-2.5)
        self.assertEqual(position, road.camera_pose(7777, height_cm=151, pitch_deg=-2.5)[0])
        self.assertEqual(rotation, road.camera_pose(7777, height_cm=151, pitch_deg=-2.5)[1])

    def test_override_rides_stay_on_the_old_straight_line(self):
        """With amplitude 0, world_pose is the pre-curve ride formula, unchanged.

        That is the contract behind OC_AT and OC_YAW: an explicitly placed camera keeps
        moving along origin + heading * distance at a fixed height, pitch and start yaw,
        whatever OC_CURVE and OC_WAVELENGTH say.
        """
        road = RoadLayout(amplitude=0)
        height, pitch = 151.0, -2.5
        ox, oy, yaw = -300.0, 420.0, 33.0
        heading = (math.cos(math.radians(yaw)), math.sin(math.radians(yaw)))
        for along in (0.0, 1234.0, 99999.0):
            position, rotation = road.world_pose(
                along, height_cm=height, pitch_deg=pitch, origin=(ox, oy), base_yaw=yaw
            )
            self.assertAlmostEqual(position[0], ox + heading[0] * along, delta=1e-6)
            self.assertAlmostEqual(position[1], oy + heading[1] * along, delta=1e-6)
            self.assertEqual(position[2], height)
            self.assertEqual(rotation, (pitch, yaw, 0.0))
        # A straight road is bent nowhere by a shifted, turned start frame either.
        road = RoadLayout(amplitude=0, wavelength=12345)
        position, _ = road.world_pose(
            2000, height_cm=height, pitch_deg=pitch, origin=(ox, oy), base_yaw=yaw
        )
        self.assertAlmostEqual(position[0], ox + heading[0] * 2000, delta=1e-6)
        self.assertAlmostEqual(position[1], oy + heading[1] * 2000, delta=1e-6)

    def test_offset_segments_chain_and_tilt(self):
        road = RoadLayout(amplitude=1200, wavelength=45000)
        lateral = 335.0  # ROAD / 2 + 35: where make_forest puts its gravel shoulders
        pieces = list(road.segments(0, 31000, 2000, lateral))
        self.assertEqual(len(pieces), 16)
        self.assertAlmostEqual(sum(span for _, _, span in pieces), 31000)
        # Chain and chord agree at every joint, so the shoulder has no seams to hide.
        x, previous_end = 0.0, None
        for centre, yaw, span in pieces:
            self.assertEqual(
                (centre, yaw, span), (*road.segment_pose(x + span / 2.0, span, lateral), span)
            )
            start, end = road.point(x, lateral), road.point(x + span, lateral)
            if previous_end is not None:
                self.assertAlmostEqual(start[0], previous_end[0])
                self.assertAlmostEqual(start[1], previous_end[1])
            previous_end, x = end, x + span
        # The shoulder follows the same bend as the road instead of a straight rail.
        straight = RoadLayout(amplitude=0).segments(0, 10000, 2000, lateral)
        for centre, yaw, _ in straight:
            self.assertEqual((centre[1], yaw), (lateral, 0.0))
        bent = {round(yaw, 6) for _, yaw, _ in pieces}
        self.assertTrue(any(abs(yaw) > 0.1 for yaw in bent))
        # The last piece is the leftover, not a full tile.
        self.assertAlmostEqual(pieces[-1][2], 1000.0)


if __name__ == "__main__":
    unittest.main()
