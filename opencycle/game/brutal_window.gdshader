shader_type spatial;
// A pane set deep in the concrete, UV in metres. Most are dark and dirty and just catch the
// street. The few still lit glow through old blinds, and don't look like anyone's home.
// COLOR is the light's colour, with how lit it is in alpha.

varying vec4 pane;

float hash(vec2 p) {
	return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453);
}

float noise(vec2 p) {
	vec2 i = floor(p);
	vec2 f = fract(p);
	f = f * f * (3.0 - 2.0 * f);
	return mix(
			mix(hash(i), hash(i + vec2(1.0, 0.0)), f.x),
			mix(hash(i + vec2(0.0, 1.0)), hash(i + vec2(1.0, 1.0)), f.x),
			f.y);
}

void vertex() {
	pane = COLOR;
}

void fragment() {
	float grime = noise(UV * vec2(3.0, 0.8)) * 0.6 + noise(UV * 17.0) * 0.4;
	float pixel = fwidth(UV.x) + fwidth(UV.y);
	float keep = 1.0 - smoothstep(0.02, 0.07, pixel);
	// Blinds, some of them hanging crooked or partway down.
	float slats = mix(0.6, 0.35 + 0.65 * smoothstep(0.2, 0.7, abs(fract(UV.y * 9.0) - 0.5) * 2.0), keep);
	float cell = floor(UV.x / 1.25);
	float drop = 0.7 + hash(vec2(cell, 3.0)) * 1.5;
	float blind = mix(1.0, slats * 0.7, smoothstep(0.03, 0.0, UV.y - drop));
	float dim = 0.75 + 0.25 * noise(UV * 0.9);
	vec3 glow = pane.rgb * pane.a * blind * dim * (1.0 - grime * 0.45);

	ALBEDO = mix(vec3(0.018, 0.02, 0.023), vec3(0.06, 0.055, 0.05), grime * 0.35);
	ROUGHNESS = mix(0.1, 0.6, clamp(grime * 0.6, 0.0, 1.0));
	METALLIC = 0.0;
	SPECULAR = 0.7;
	EMISSION = glow;
}
