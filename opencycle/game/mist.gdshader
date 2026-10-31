shader_type fog;
// Air you can see: mist lying low along the wet street, drifting, thick enough that every
// light standing in it makes a pool of colour.

global uniform float ride_time;

uniform float thickness = 0.014;

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
	vec3 drift = WORLD_POSITION * vec3(0.09, 0.3, 0.09) + vec3(ride_time * 0.05, 0.0, ride_time * 0.03);
	float clouds = noise(drift) * 0.65 + noise(drift * 2.3) * 0.35;
	float low = 1.0 - smoothstep(0.0, 1.0, UVW.y);
	DENSITY = thickness * low * low * smoothstep(0.38, 0.68, clouds) * 2.0;
	ALBEDO = vec3(0.9, 0.9, 1.0);
}
