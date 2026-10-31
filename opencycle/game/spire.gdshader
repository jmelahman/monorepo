shader_type spatial;
// The far towers: dark glass seen through smog, a grid of lit windows and bright lines
// of LEDs running up the corners with pulses climbing them. UV is in metres. COLOR is the
// tower's neon, and they're too far off for anything subtler to read.
render_mode unshaded;

global uniform float ride_time;

float hash(vec2 p) {
	return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453);
}

void fragment() {
	vec2 cell = UV / vec2(4.0, 3.6);
	vec2 id = floor(cell);
	vec2 at = fract(cell);
	float pane = step(0.14, at.x) * step(at.x, 0.86) * step(0.2, at.y) * step(at.y, 0.8);
	float lit = step(0.72, hash(id));
	vec3 lamp = mix(vec3(1.0, 0.72, 0.45), vec3(0.55, 0.85, 1.0), step(0.6, hash(id + 4.0)));
	// Past where a window is a pixel, the pane's share of the wall is what the eye gets.
	float pixel = fwidth(UV.y);
	float far = smoothstep(0.5, 2.0, pixel);
	vec3 windows = lamp * lit * mix(pane, 0.22, far) * (0.4 + hash(id + 8.0) * 0.6);
	// Bands of LEDs up and across, each on its own beat.
	float line = floor(UV.x / 24.0);
	float rail = smoothstep(0.9, 0.2, abs(mod(UV.x, 24.0) - 1.0));
	float pulse = fract(UV.y / 90.0 - ride_time * 0.18 - hash(vec2(line, 2.0)));
	vec3 neon = COLOR.rgb * rail * (0.8 + 6.0 * pulse * pulse * pulse * pulse);
	float floors = smoothstep(0.7, 0.2, abs(mod(UV.y + 20.0, 48.0) - 1.0));
	neon += COLOR.rgb * floors * 1.1;
	ALBEDO = vec3(0.012, 0.012, 0.022) + windows * 1.5 + neon * 1.6;
}
