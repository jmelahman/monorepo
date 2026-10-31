shader_type spatial;
// Rain in front of the camera: fine streaks falling in three layers, each picking up the
// colour of the street behind it, so it only shows where something's lit. Runs off the
// ride's clock, so a replay falls the same way every time.
render_mode unshaded, blend_add, cull_disabled, depth_test_disabled, depth_draw_never, shadows_disabled;

global uniform float ride_time;

uniform sampler2D screen : hint_screen_texture, filter_linear_mipmap, repeat_disable;
uniform float strength = 2.2;

float hash(vec2 p) {
	return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453);
}

void vertex() {
	POSITION = vec4(VERTEX.xy, 1.0, 1.0);
}

float streaks(vec2 uv, float columns, float length_of, float speed, float cover) {
	vec2 slanted = vec2(uv.x * columns + uv.y * columns * 0.06, uv.y / length_of);
	float column = floor(slanted.x);
	float x = fract(slanted.x) - 0.5;
	float fall = slanted.y - ride_time * speed * (0.8 + 0.4 * hash(vec2(column, 3.0)));
	fall += hash(vec2(column, 9.0)) * 40.0;
	float cell = floor(fall);
	float y = fract(fall);
	float falling = step(1.0 - cover, hash(vec2(column, cell)));
	float offset = (hash(vec2(column, cell + 5.0)) - 0.5) * 0.7;
	float thin = 1.0 - smoothstep(0.0, 0.22, abs(x - offset));
	return falling * thin * smoothstep(0.0, 0.25, y) * (1.0 - smoothstep(0.55, 1.0, y));
}

void fragment() {
	vec2 uv = SCREEN_UV * vec2(VIEWPORT_SIZE.x / VIEWPORT_SIZE.y, 1.0);
	float rain = streaks(uv, 90.0, 0.2, 7.0, 0.14) * 0.35
			+ streaks(uv + 3.0, 160.0, 0.11, 8.0, 0.16) * 0.22
			+ streaks(uv + 7.0, 280.0, 0.06, 9.0, 0.18) * 0.14;
	vec3 behind = textureLod(screen, SCREEN_UV, 4.0).rgb;
	ALBEDO = (behind + vec3(0.12, 0.14, 0.2)) * rain * strength;
	ALPHA = 1.0;
}
