shader_type fog;
// Steam rising from a cover in the road: a thin column that billows out as it climbs and
// thins away. It takes its colour from whatever lights it.

global uniform float ride_time;

uniform float thickness = 2.2;
uniform float phase = 0.0;

float hash(vec3 p) {
	return fract(sin(dot(p, vec3(127.1, 311.7, 74.7))) * 43758.5453);
}

float noise(vec3 p) {
	vec3 i = floor(p);
	vec3 f = fract(p);
	f = f * f * (3.0 - 2.0 * f);
	return mix(
			mix(mix(hash(i), hash(i + vec3(1.0, 0.0, 0.0)), f.x),
					mix(hash(i + vec3(0.0, 1.0, 0.0)), hash(i + vec3(1.0, 1.0, 0.0)), f.x), f.y),
			mix(mix(hash(i + vec3(0.0, 0.0, 1.0)), hash(i + vec3(1.0, 0.0, 1.0)), f.x),
					mix(hash(i + vec3(0.0, 1.0, 1.0)), hash(i + vec3(1.0, 1.0, 1.0)), f.x), f.y),
			f.z);
}

void fog() {
	float rise = UVW.y;
	vec2 across = (UVW.xz - 0.5) * 2.0;
	vec3 at = WORLD_POSITION * vec3(0.8, 0.45, 0.8) - vec3(0.0, ride_time * 0.9 + phase * 40.0, 0.0);
	float wisps = noise(at) * 0.6 + noise(at * 2.1) * 0.4;
	float width = mix(0.18, 0.95, sqrt(rise));
	float column = 1.0 - smoothstep(width * 0.3, width, length(across));
	DENSITY = thickness * column * smoothstep(0.3, 0.62, wisps) * (1.0 - rise) * (1.0 - rise);
	ALBEDO = vec3(1.0);
}
