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
// Over road paint: as worn as the paint, so bare patches in a line don't mirror twice over.
uniform float worn = 0.0;

const int TAPS = 9;
// What the near road sees straight above it is darker than the top of the frame.
const float OVERHEAD = 0.6;

varying vec3 world;

#include "res://game/road_wear.gdshaderinc"

float standing_water(vec2 at) {
	float broad = noise(at * 0.13) * 0.6 + noise(at * 0.47) * 0.32 + noise(at * 1.9) * 0.08;
	return smoothstep(0.62, 0.67, broad) * puddles;
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
	vec2 tilt = mix(vec2(grain, grain_across) * 0.009, ripple * 0.006, water);
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
	float beyond = 0.0;
	// A different offset for each pixel, or a small bright light shows up as one rung per tap.
	float jitter = fract(52.9829189 * fract(dot(FRAGCOORD.xy, vec2(0.06711056, 0.00583715))));
	for (int i = 0; i < TAPS; i++) {
		float along = (float(i) + jitter) / float(TAPS);
		vec2 tap = uv + vec2(0.0, (along - 0.3) * spread);
		// Off the top of the frame there's nothing to read, so it takes the overall light up there.
		float inside = smoothstep(1.0, 0.86, 1.0 - tap.y) * smoothstep(0.0, 0.03, tap.x)
				* smoothstep(1.0, 0.97, tap.x);
		float share = 1.0 - abs(along - 0.3);
		seen += textureLod(screen, clamp(tap, 0.0, 1.0), blur).rgb * inside * share;
		beyond += (1.0 - inside) * share;
		weight += share;
	}
	seen += textureLod(screen, vec2(clamp(uv.x, 0.0, 1.0), 0.12), 6.0).rgb * beyond * OVERHEAD;
	seen /= weight;
	float amount = mix(mix(0.16, 0.65, fresnel), mix(0.3, 0.8, fresnel), water);
	float kept = 1.0;
	if (worn > 0.0) {
		vec2 worn_at = wear_at(at);
		kept = worn_through(worn_at, wear_grit(worn_at, length(fwidth(at))), worn) ? 0.0 : 1.0;
	}
	ALBEDO = seen * amount * strength * kept * step(0.0, clip.w);
	ALPHA = 1.0;
}
