shader_type spatial;
// Things on shop shelves: every box a colour with a printed band and a pale label, which
// at shelf distance is all that the eye needs to read it as packaging.

varying vec3 local;

void vertex() {
	local = VERTEX;
}

void fragment() {
	float print = step(0.5, fract(local.y * 34.0 + COLOR.r * 7.0));
	float label = step(abs(local.x * 11.0 + (COLOR.g - 0.5)), 0.5)
			* step(abs(local.y * 16.0 + COLOR.b - 0.5), 0.5);
	vec3 pack = COLOR.rgb * mix(1.0, 0.72, print);
	ALBEDO = mix(pack, vec3(0.92, 0.9, 0.82), label);
	ROUGHNESS = 0.42;
}
