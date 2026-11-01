// Value noise, and how far decades of tyres have taken road paint off. Shared by the paint and
// the wet sheen over it, so what has worn away stops mirroring as well.

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

// Wrapped, since the hash runs out of float precision far along the route.
vec2 wear_at(vec2 position) {
	return mod(position, 64.0);
}

// The finest wear, settling to its average once a pixel covers more than the grit, so a far
// line holds still. [param footprint] is fwidth of the position, in metres.
float wear_grit(vec2 at, float footprint) {
	float grit = noise(at * 41.0) * 0.5 + noise(at * 113.0) * 0.5;
	return mix(grit, 0.5, smoothstep(0.012, 0.05, footprint));
}

bool worn_through(vec2 at, float grit, float wear) {
	float patches = noise(at * 0.9) * 0.6 + noise(at * 3.7) * 0.25;
	return patches + grit * 0.45 < wear * 0.9;
}
