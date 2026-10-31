shader_type spatial;
// A lane of air traffic: lights a long way overhead, moving off the ride's clock. UV.x is
// metres along the lane, UV2 holds its speed and phase, and only some cells hold a vehicle.
render_mode unshaded, blend_add, cull_disabled, depth_draw_never, shadows_disabled;

global uniform float ride_time;

uniform float spacing = 34.0;
uniform float cover = 0.55;
uniform float brightness = 7.0;

varying vec2 lane;

float hash(float x) {
	return fract(sin(x * 91.17) * 43758.5453);
}

void vertex() {
	lane = UV2;
}

void fragment() {
	float x = UV.x - ride_time * lane.x + lane.y;
	float cell = floor(x / spacing);
	float into = fract(x / spacing) * spacing;
	float there = step(1.0 - cover, hash(cell));
	// A bright nose and a tail fading behind it.
	float along = smoothstep(0.0, 0.5, into) * (1.0 - smoothstep(0.5, 7.0, into));
	float across = 1.0 - smoothstep(0.1, 1.0, abs(UV.y * 2.0 - 1.0));
	ALBEDO = COLOR.rgb * brightness * there * along * across;
}
