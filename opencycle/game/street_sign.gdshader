shader_type spatial;
// An enamelled road sign, long past its best: a white border and symbol on a colour, chipped
// and scratched, with rust weeping from the top where the bolts go through. The film is
// faintly retroreflective, so it lifts in whatever light is about.

uniform vec3 ground = vec3(0.08, 0.2, 0.42);
// 0 is an arrow ahead, 1 chevrons, 2 a clearway's cross.
uniform int symbol = 0;

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

float box(vec2 p, vec2 half_size) {
	vec2 d = abs(p) - half_size;
	return max(d.x, d.y);
}

float mark(vec2 p) {
	if (symbol == 1) {
		float y = mod(p.y + abs(p.x) * 0.9 + 0.06, 0.26) - 0.13;
		return max(abs(y) - 0.05, max(abs(p.x) - 0.26, abs(p.y) - 0.27));
	}
	if (symbol == 2) {
		float c = 0.7071;
		vec2 turned = vec2(p.x * c - p.y * c, p.x * c + p.y * c);
		return min(box(turned, vec2(0.34, 0.05)), box(turned, vec2(0.05, 0.34)));
	}
	float head = max(p.y - 0.3 + abs(p.x) * 1.15, -p.y + 0.02);
	return min(box(p + vec2(0.0, 0.14), vec2(0.07, 0.17)), head);
}

void fragment() {
	vec2 p = vec2(UV.x - 0.5, 0.5 - UV.y);
	float rim = box(p, vec2(0.46));
	float white = max(step(0.0, rim), step(mark(p), 0.0));
	vec3 colour = mix(ground, vec3(0.62, 0.62, 0.58), white);
	float grain = noise(UV * 70.0);
	float chips = smoothstep(0.76, 0.8, noise(UV * 13.0 + 4.0) * 0.7 + grain * 0.35);
	float weep = noise(vec2(UV.x * 24.0, 1.0)) * smoothstep(0.75, 0.0, UV.y);
	float rust = clamp(chips + smoothstep(0.55, 0.85, weep) * 0.7, 0.0, 1.0);
	ALBEDO = mix(colour * mix(0.72, 1.0, grain), vec3(0.16, 0.08, 0.04), rust);
	ROUGHNESS = mix(0.38, 0.9, rust);
	SPECULAR = 0.55;
	float head_on = pow(max(dot(NORMAL, VIEW), 0.0), 6.0);
	EMISSION = colour * 0.035 * head_on * (1.0 - rust);
}
