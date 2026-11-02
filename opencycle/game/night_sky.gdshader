shader_type sky;
// Low cloud over a city that never turns its lights off: lit from underneath, warm where it comes
// down to the rooftops and going cold and dark overhead. Most of the ride never sees it.

const vec3 HIGH = vec3(0.03, 0.04, 0.07);
const vec3 LOW = vec3(0.12, 0.08, 0.1);
const vec3 CLOUD_HIGH = vec3(0.34, 0.36, 0.52);
const vec3 CLOUD_LOW = vec3(1.1, 0.5, 0.34);

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

float clouds(vec2 p) {
	float sum = 0.0;
	float weight = 0.5;
	for (int i = 0; i < 5; i++) {
		sum += noise(p) * weight;
		p = p * 2.07 + vec2(11.3, 4.7);
		weight *= 0.5;
	}
	return sum;
}

void sky() {
	float up = max(EYEDIR.y, 0.0);
	// A flat layer overhead: further off it crowds down towards the horizon, as cloud does.
	vec2 layer = EYEDIR.xz / (up + 0.12) * 0.9;
	float cover = smoothstep(0.34, 0.72, clouds(layer));
	float near_ground = pow(1.0 - up, 5.0);
	vec3 clear = mix(HIGH, LOW, near_ground);
	vec3 lit = mix(CLOUD_HIGH, CLOUD_LOW, near_ground);
	COLOR = mix(clear, lit, cover) * smoothstep(-0.05, 0.02, EYEDIR.y);
}
