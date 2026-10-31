extends GdUnitTestSuite

const DATA := {
	"speed": [{"at_m": 0.0, "kmh": 36.0}, {"at_m": 100.0, "kmh": 36.0}],
	"camera":
	[
		{"at_m": 0.0, "lateral": -1.0, "height": 0.0},
		{"at_m": 100.0, "lateral": 1.0, "height": 0.0},
	],
}


func test_constant_speed_takes_the_expected_time() -> void:
	assert_float(Replay.from_dict(DATA).duration()).is_equal_approx(10.0, 0.001)


func test_distance_follows_time_and_stops_at_the_end() -> void:
	var replay := Replay.from_dict(DATA)
	assert_float(replay.distance_at(5.0)).is_equal_approx(50.0, 0.01)
	assert_float(replay.distance_at(99.0)).is_equal(100.0)


func test_same_time_gives_the_same_camera() -> void:
	var route := Route.generate(2)
	var a := Replay.from_dict(DATA).camera_at(route, 3.3)
	var b := Replay.from_dict(DATA).camera_at(route, 3.3)
	assert_bool(a.is_equal_approx(b)).is_true()


func test_camera_stays_above_the_road() -> void:
	var route := Route.generate(2)
	var replay := Replay.from_dict(DATA)
	var camera := replay.camera_at(route, 4.0)
	assert_float(camera.origin.y).is_greater(route.at(replay.distance_at(4.0)).y + 1.0)
