shader_type spatial;
// A second pass over black, standing water: what's really above it, found by walking the view
// ray up off the surface through the depth buffer until it runs into something. Columns meet
// their own reflections at the waterline, which a road's cheap mirror can't manage.
render_mode unshaded, blend_add, depth_draw_never, shadows_disabled, fog_disabled;

global uniform float ride_time;

uniform sampler2D screen : hint_screen_texture, filter_linear_mipmap, repeat_disable;
uniform sampler2D depth : hint_depth_texture, filter_nearest, repeat_disable;

const int STEPS = 22;
const int REFINE = 5;
const float FIRST_STEP = 0.3;
const float GROWTH = 1.3;
// Where the ray finds nothing it's looking at the roof, which only gives back a little.
const float ROOF_LIGHT = 0.22;
const float ROOF_BLUR = 5.0;

varying vec3 world;

#include "res://game/road_wear.gdshaderinc"

void vertex() {
	VERTEX.y += 0.003;
	world = (MODEL_MATRIX * vec4(VERTEX, 1.0)).xyz;
}

vec2 screen_at(vec3 view, mat4 projection) {
	vec4 clip = projection * vec4(view, 1.0);
	return clip.xy / max(clip.w, 0.001) * 0.5 + 0.5;
}

float scene_z(vec2 uv, mat4 inverse_projection) {
	vec4 view = inverse_projection * vec4(uv * 2.0 - 1.0, texture(depth, uv).r, 1.0);
	return view.z / view.w;
}

void fragment() {
	vec2 at = world.xz;
	// A slow swell and a finer shiver, and only a little of either: this water hardly moves.
	vec2 flow = vec2(ride_time * 0.11, ride_time * 0.07);
	vec2 swell = vec2(noise(at * 0.6 + flow), noise(at * 0.6 - flow + 17.0)) - 0.5;
	vec2 shiver = vec2(noise(at * 3.1 - flow * 3.0), noise(at * 3.1 + flow * 3.0 + 5.0)) - 0.5;
	vec3 up = normalize(vec3(swell.x * 0.014 + shiver.x * 0.005, 1.0, swell.y * 0.05 + shiver.y * 0.02));
	vec3 incoming = normalize(world - CAMERA_POSITION_WORLD);
	vec3 bounced = reflect(incoming, up);
	vec3 ray = normalize((VIEW_MATRIX * vec4(bounced, 0.0)).xyz);
	// A different start for each pixel, which TAA turns from bands into a smooth reflection.
	float jitter = fract(52.9829189 * fract(dot(FRAGCOORD.xy, vec2(0.06711056, 0.00583715))));
	float near = 0.0;
	float far = FIRST_STEP * (1.0 + jitter * 0.3);
	bool hit = false;
	for (int i = 0; i < STEPS && !hit; i++) {
		vec3 point = VERTEX + ray * far;
		vec2 uv = screen_at(point, PROJECTION_MATRIX);
		if (uv.x < 0.0 || uv.x > 1.0 || uv.y < 0.0 || uv.y > 1.0 || point.z > -0.05) {
			break;
		}
		float behind = scene_z(uv, INV_PROJECTION_MATRIX) - point.z;
		if (behind > 0.0 && behind < max((far - near) * 1.6, 1.0)) {
			hit = true;
		} else {
			near = far;
			far *= GROWTH;
		}
	}
	vec3 seen;
	float confidence = 0.0;
	if (hit) {
		for (int i = 0; i < REFINE; i++) {
			float middle = (near + far) * 0.5;
			vec3 point = VERTEX + ray * middle;
			if (scene_z(screen_at(point, PROJECTION_MATRIX), INV_PROJECTION_MATRIX) > point.z) {
				far = middle;
			} else {
				near = middle;
			}
		}
		vec2 uv = screen_at(VERTEX + ray * far, PROJECTION_MATRIX);
		seen = textureLod(screen, uv, 0.0).rgb;
		// Fading out towards the frame's edges, where the picture to read runs out.
		vec2 edge = min(uv, 1.0 - uv);
		confidence = smoothstep(0.0, 0.06, edge.x) * smoothstep(0.0, 0.08, edge.y);
	}
	vec2 overhead = clamp(screen_at(VERTEX + ray * 30.0, PROJECTION_MATRIX), 0.0, 1.0);
	vec3 roof = textureLod(screen, overhead, ROOF_BLUR).rgb * ROOF_LIGHT;
	seen = mix(roof, seen, confidence);
	float facing = 1.0 - clamp(dot(-incoming, vec3(0.0, 1.0, 0.0)), 0.0, 1.0);
	float fresnel = 0.04 + 0.96 * pow(facing, 5.0);
	ALBEDO = seen * mix(0.3, 1.0, fresnel);
	ALPHA = 1.0;
}
