shader_type spatial;
// Paint on a road decades old: tyres have taken it off in patches and along the grain of the
// traffic, so the asphalt shows through, and what's left is stained. Mapped in world space
// so no two arrows wear alike.

uniform vec3 paint = vec3(0.45, 0.45, 0.42);
// How much of it has gone, from 0 to 1.
uniform float worn = 0.4;

varying vec3 world;

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
	world = (MODEL_MATRIX * vec4(VERTEX, 1.0)).xyz;
}

void fragment() {
	float patches = noise(world.xz * 0.9) * 0.6 + noise(world.xz * 3.7) * 0.25;
	float grit = noise(world.xz * 41.0) * 0.5 + noise(world.xz * 113.0) * 0.5;
	if (patches + grit * 0.45 < worn * 0.9) {
		discard;
	}
	ALBEDO = paint * mix(0.55, 1.0, noise(world.xz * 7.0)) * mix(0.75, 1.0, grit);
	ROUGHNESS = 0.55;
	SPECULAR = 0.4;
}
