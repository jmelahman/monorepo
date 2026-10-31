shader_type spatial;
// A pane of glass with a room behind it, worked out by tracing the view ray through a box
// behind the wall. The kit's windows sit in modules 3 m wide, running from x -3 to 0 and y 0
// to 3, facing +z. INSTANCE_CUSTOM.xy picks the room, and many are dark: a dark pane is just
// glass, and shows the street in it.

global uniform float ride_time;

const vec3 ROOM_LOW = vec3(-3.3, -0.1, -4.0);
const vec3 ROOM_HIGH = vec3(0.3, 3.1, 0.2);

varying vec3 local;
varying vec3 eye;
varying vec2 seed;

float hash(vec2 p) {
	return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453);
}

void vertex() {
	local = VERTEX;
	eye = (inverse(MODEL_MATRIX) * vec4(CAMERA_POSITION_WORLD, 1.0)).xyz;
	seed = INSTANCE_CUSTOM.xy;
}

void fragment() {
	vec3 ray = normalize(local - eye);
	vec3 far_side = mix(ROOM_LOW, ROOM_HIGH, step(0.0, ray));
	vec3 to_wall = (far_side - local) / ray;
	float t = min(min(to_wall.x, to_wall.y), to_wall.z);
	vec3 hit = local + ray * t;
	float kind = hash(seed);
	float tone = hash(seed + 3.1);
	vec3 paint = mix(vec3(0.82, 0.74, 0.6), vec3(0.45, 0.62, 0.66), tone) * (0.5 + hash(seed + 1.7) * 0.5);
	vec3 lamp = vec3(1.0, 0.68, 0.38);
	if (kind > 0.86) {
		lamp = vec3(1.0, 0.18, 0.55);
	} else if (kind > 0.74) {
		lamp = vec3(0.2, 0.75, 1.0);
	} else if (kind > 0.66) {
		lamp = vec3(0.85, 0.95, 1.0);
	}
	vec3 surface = paint;
	float fitting = 0.0;
	if (to_wall.y <= to_wall.x && to_wall.y <= to_wall.z) {
		if (ray.y > 0.0) {
			float across = abs(hit.x + 1.5);
			float deep = abs(hit.z + 2.0);
			fitting = step(across, 0.6) * step(deep, 0.08);
			surface = paint * 0.8;
		} else {
			surface = paint * vec3(0.3, 0.22, 0.16);
		}
	} else if (to_wall.z < to_wall.x) {
		// The far wall: a tall cupboard or a shelf, and a poster or a screen.
		float cupboard = step(hash(seed + floor(hit.x * 0.9)), 0.5) * step(hit.y, 1.9);
		surface = mix(paint, paint * 0.3, cupboard);
		vec2 poster = abs(vec2(hit.x + 1.2 + (tone - 0.5), hit.y - 1.9)) - vec2(0.45, 0.3);
		float screen = 1.0 - step(0.0, max(poster.x, poster.y));
		float flicker = 0.75 + 0.25 * sin(ride_time * (3.0 + tone * 9.0) + kind * 40.0);
		surface += screen * lamp.bgr * 2.2 * flicker * step(0.45, tone);
	} else {
		// Side walls fade to dark away from the light.
		surface = paint * 0.7;
	}
	// Someone standing back from the glass, on some floors.
	if (hash(seed + 6.3) > 0.72) {
		float depth = (-1.6 - local.z) / ray.z;
		vec3 at = local + ray * depth;
		vec2 from = vec2(at.x + 0.9 + hash(seed + 2.2) * -1.2, at.y - 0.8);
		float body = step(abs(from.x), 0.2) * step(abs(from.y), 0.75);
		float head = step(length(from - vec2(0.0, 0.95)), 0.13);
		if (depth > 0.0 && depth < t && body + head > 0.5) {
			surface = vec3(0.012);
		}
	}
	float fall = mix(0.35, 1.0, smoothstep(-4.0, 0.0, hit.z)) * mix(0.55, 1.0, smoothstep(0.0, 3.0, hit.y));
	vec3 lit = surface * lamp * fall * (0.55 + hash(seed + 8.0) * 0.65) * 1.3 + fitting * lamp * 5.0;
	// A blind or curtain partway down, glowing with the room behind it.
	float blind = hash(seed + 5.0);
	float covered = step(mix(2.4, 0.9, blind * blind), local.y) * step(0.35, blind);
	lit = mix(lit, lamp * vec3(0.8, 0.75, 0.65) * 0.55, covered);
	float on = step(0.52, hash(seed + 11.0));
	ALBEDO = vec3(0.012, 0.014, 0.018);
	ROUGHNESS = 0.06;
	METALLIC = 0.4;
	SPECULAR = 0.7;
	EMISSION = lit * on;
}
