shader_type spatial;
render_mode unshaded;
// The glazing over the slot in the deck, seen from a long way under it. Filthy panes on a grid
// of bars, each a different brightness with what's above it, and some of them painted out.
// COLOR is the light coming through.

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
	vec2 at = world.xz / vec2(2.2, 3.4);
	vec2 cell = floor(at);
	vec2 edge = abs(fract(at) - 0.5) * vec2(2.2, 3.4);
	float bars = smoothstep(1.02, 0.96, edge.x) * smoothstep(1.62, 1.56, edge.y);
	// The main frames, every few panes.
	vec2 frame = abs(fract(world.xz / 13.0) - 0.5) * 13.0;
	bars *= smoothstep(6.1, 5.9, max(frame.x, frame.y));
	float pane = 0.35 + hash(cell) * 0.65;
	pane *= step(0.14, hash(cell + 31.0));
	float grime = noise(world.xz * 0.7) * 0.6 + noise(world.xz * 3.1) * 0.4;
	ALBEDO = COLOR.rgb * pane * bars * mix(1.0, 0.25, smoothstep(0.3, 0.75, grime));
}
