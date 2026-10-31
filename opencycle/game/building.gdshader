shader_type spatial;
// Tower walls, without a texture: floors and bays worked out from metres in UV, each
// window lit or dark from a hash of which window it is, in the tower's colour.

uniform vec2 window = vec2(1.7, 3.3);
uniform float wall_tone = 0.035;

varying vec3 hue;

float hash(vec2 p) {
	return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453);
}

void vertex() {
	hue = COLOR.rgb;
}

void fragment() {
	vec2 cell = floor(UV / window);
	vec2 inside = fract(UV / window);
	vec2 pane = smoothstep(vec2(0.14, 0.26), vec2(0.18, 0.3), inside)
			* smoothstep(vec2(0.86, 0.8), vec2(0.82, 0.76), inside);
	float glass = pane.x * pane.y;
	float lit = step(0.74, hash(cell + COLOR.a * 91.0));
	float warmth = hash(cell.yx + 7.0);
	vec3 light = mix(hue, vec3(1.0, 0.82, 0.55), warmth * 0.7);
	ALBEDO = mix(vec3(wall_tone), vec3(0.02), glass);
	ROUGHNESS = mix(0.8, 0.12, glass);
	METALLIC = glass * 0.6;
	EMISSION = light * lit * glass * (0.12 + hash(cell) * hash(cell) * 0.9);
}
