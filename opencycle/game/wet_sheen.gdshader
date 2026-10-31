shader_type spatial;
// A second pass over the wet ground: the street's own picture, mirrored and added on top.
// The mirror isn't a true reflection, it just aims the view ray up off the ground and reads
// the screen where that ray lands. Damp ground stretches it into long streaks of colour,
// and standing water holds it nearly sharp. Ripples run off the ride's clock.
render_mode unshaded, blend_add, depth_draw_never, shadows_disabled;

global uniform float ride_time;

uniform sampler2D screen : hint_screen_texture, filter_linear_mipmap, repeat_disable;
uniform float strength = 1.0;
uniform float puddles = 1.0;
// How far off the ground what's being reflected stands, roughly: the signs across the road.
uniform float reach = 28.0;

const int TAPS = 9;

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

float standing_water(vec2 at) {
	float broad = noise(at * 0.13) * 0.55 + noise(at * 0.47) * 0.3 + noise(at * 1.9) * 0.15;
	return smoothstep(0.6, 0.63, broad) * puddles;
}

void vertex() {
	// Just off the surface it's drawn over, which the paint on the road stands above.
	VERTEX.y += 0.003;
	world = (MODEL_MATRIX * vec4(VERTEX, 1.0)).xyz;
}

void fragment() {
	vec2 at = world.xz;
	float water = standing_water(at);
	float grain = noise(at * 14.0) + noise(at * 37.0) * 0.5 - 0.75;
	float grain_across = noise(at.yx * 14.0 + 40.0) + noise(at.yx * 37.0 + 11.0) * 0.5 - 0.75;
	// Rain landing on still water.
	vec2 flow = vec2(ride_time * 0.7, ride_time * 0.5);
	vec2 ripple = vec2(noise(at * 3.0 + flow), noise(at * 3.0 - flow + 17.0)) - 0.5;
	vec2 tilt = mix(vec2(grain, grain_across) * 0.018, ripple * 0.014, water);
	vec3 up = normalize(vec3(tilt.x, 1.0, tilt.y));
	vec3 incoming = normalize(world - CAMERA_POSITION_WORLD);
	vec3 bounced = reflect(incoming, up);
	vec4 clip = PROJECTION_MATRIX * VIEW_MATRIX * vec4(world + bounced * reach, 1.0);
	vec2 uv = clip.xy / max(clip.w, 0.001) * 0.5 + 0.5;
	float facing = 1.0 - clamp(dot(-incoming, vec3(0.0, 1.0, 0.0)), 0.0, 1.0);
	float fresnel = 0.04 + 0.96 * pow(facing, 5.0);
	float spread = mix(0.24, 0.012, water);
	float blur = mix(3.5, 0.5, water);
	vec3 seen = vec3(0.0);
	float weight = 0.0;
	for (int i = 0; i < TAPS; i++) {
		float along = float(i) / float(TAPS - 1);
		vec2 tap = uv + vec2(0.0, (along - 0.3) * spread);
		// Off the top of the frame there's nothing to read: let the reflection fade out.
		float inside = smoothstep(1.0, 0.86, 1.0 - tap.y) * smoothstep(0.0, 0.03, tap.x)
				* smoothstep(1.0, 0.97, tap.x);
		float share = 1.0 - abs(along - 0.3);
		seen += textureLod(screen, clamp(tap, 0.0, 1.0), blur).rgb * inside * share;
		weight += share;
	}
	seen /= weight;
	float amount = mix(mix(0.16, 0.65, fresnel), mix(0.45, 1.0, fresnel), water);
	ALBEDO = seen * amount * strength * step(0.0, clip.w);
	ALPHA = 1.0;
}
