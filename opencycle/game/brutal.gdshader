shader_type spatial;
// Raw concrete decades after anyone last looked after it. A scanned finish laid out in world
// metres, so it runs on across every fin and slab, broken into the formwork panels it was
// cast in, with rain, rust and patching on top. COLOR is the tint, with the building's age
// in alpha. UV2 is [floor height, height of the first floor], or 0 where there are no ledges.

uniform sampler2D albedo : source_color, filter_linear_mipmap_anisotropic, repeat_enable;
uniform sampler2D normals : hint_normal, filter_linear_mipmap_anisotropic, repeat_enable;
uniform sampler2D rough : hint_roughness_g, filter_linear_mipmap_anisotropic, repeat_enable;
uniform sampler2D occlusion : hint_default_white, filter_linear_mipmap_anisotropic, repeat_enable;
uniform float tile = 3.0;
uniform vec3 wash_colour = vec3(0.45, 0.62, 0.8);
// A modelled surface undulates, and metres from the origin times a slightly different normal
// is a wildly different place in the scan, so it takes its run from the model's axes instead.
uniform bool modelled = false;

varying vec3 world;
varying vec3 world_normal;
varying vec2 ledges;
varying vec3 local_normal;
varying vec2 run_x;
varying vec2 run_z;

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
	world_normal = normalize(mat3(MODEL_MATRIX) * NORMAL);
	ledges = UV2;
	local_normal = NORMAL;
	run_x = MODEL_MATRIX[0].xz;
	run_z = MODEL_MATRIX[2].xz;
}

// The dip of a joint between formwork panels, and of the tie holes along it. 0 on the face.
float formwork(vec2 at) {
	vec2 panel = at / vec2(2.4, 1.2);
	vec2 course = vec2(panel.x + 0.5 * mod(floor(panel.y), 2.0), panel.y);
	vec2 from_joint = abs(fract(course) - 0.5) * vec2(2.4, 1.2);
	float joint = smoothstep(0.012, 0.0, min(1.2 - from_joint.x, 0.6 - from_joint.y));
	vec2 tie = abs(fract(at / vec2(0.6, 0.6)) - 0.5) * 0.6;
	float hole = smoothstep(0.024, 0.016, length(tie)) * step(0.7, hash(floor(at / 0.6)));
	return max(joint, hole);
}

void fragment() {
	vec3 n = abs(world_normal);
	// Along the wall whichever way it faces, so an angled one doesn't smear.
	vec2 run = normalize(vec2(-world_normal.z, world_normal.x) + vec2(0.0001));
	if (modelled) {
		run = normalize(abs(local_normal.x) > abs(local_normal.z) ? run_z : run_x);
	}
	vec2 at = n.y > 0.5 ? world.xz : vec2(dot(world.xz, run), world.y);
	float age = COLOR.a;
	vec2 close = at / tile;
	// Two scales of the same scan: one breaks up the repeat, the other holds up close.
	vec3 scan = texture(albedo, close).rgb;
	float broad = dot(texture(albedo, close * 0.19 + 0.37).rgb, vec3(0.33));
	float finer = dot(texture(albedo, close * 3.3).rgb, vec3(0.33));
	scan *= (0.55 + broad * 1.0) * (0.7 + finer * 0.65);
	scan *= mix(0.75, 1.0, texture(occlusion, close).r);
	float grey = dot(scan, vec3(0.3, 0.6, 0.1));
	vec3 surface = mix(vec3(grey), scan, 0.55) * COLOR.rgb * 2.0;
	float roughness = texture(rough, close).g;
	vec3 normal_map = texture(normals, close).xyz;
	vec3 finer_normal = texture(normals, close * 3.3).xyz;
	normal_map.xy += (finer_normal.xy - 0.5) * 0.6;

	// The panels it was poured in: each a slightly different tone, with real relief at the
	// joints. Only on walls, and only while the joint is wider than a pixel or so.
	if (n.y < 0.5) {
		float keep = 1.0 - smoothstep(0.012, 0.03, fwidth(at.x) + fwidth(at.y));
		float dip = formwork(at);
		vec2 pour_cell = floor(at / vec2(2.4, 1.2));
		surface *= 0.92 + 0.16 * hash(pour_cell + 11.0);
		surface *= 1.0 - dip * 0.65 * keep;
		float e = 0.006;
		vec2 slope = vec2(formwork(at + vec2(e, 0.0)) - dip, formwork(at + vec2(0.0, e)) - dip);
		normal_map.xy += slope * 1.2 * keep;
	}

	// Patches where somebody cast a repair, each a slightly different pour.
	vec2 pour = floor(at / vec2(3.7, 2.3));
	float repair = step(0.9, hash(pour + 4.0)) * (0.8 + hash(pour) * 0.5);
	surface *= mix(1.0, repair, step(0.01, repair));

	// Long dark runs, thin and close together, the wettest of them shining.
	float runs = noise(vec2(at.x * 2.6, at.y * 0.07));
	float fine = noise(vec2(at.x * 11.0, at.y * 0.22));
	float blotch = noise(at * 0.13);
	// Water runs down a wall. A soffit or a ledge just goes blotchy, or it'd stripe like tartan.
	float streaks = n.y < 0.5 ? runs * 0.6 + fine * 0.25 : noise(at * 0.9) * 0.85;
	float dirt = smoothstep(0.35, 0.8, streaks + blotch * 0.3);
	dirt = clamp(dirt * (0.45 + age * 0.75), 0.0, 1.0);
	// Heaviest right under each ledge, where the water comes off it.
	if (ledges.x > 0.1 && n.y < 0.5) {
		float below = ledges.x - mod(world.y - ledges.y, ledges.x);
		float run_off = exp(-below / 1.3) * (0.35 + fine * 1.1);
		dirt = clamp(dirt + run_off * (0.4 + age), 0.0, 1.0);
	}
	surface *= mix(1.0, 0.42, dirt);
	roughness = mix(roughness, 0.22, dirt * 0.8);

	// Rust bleeding from the rebar and the fixings, in rare narrow lines.
	float line_of_rust = smoothstep(0.82, 0.92, noise(vec2(at.x * 4.3, at.y * 0.05) + 19.0));
	float bleed = line_of_rust * noise(vec2(at.x * 19.0, at.y * 0.4)) * age;
	surface = mix(surface, vec3(0.2, 0.085, 0.035), clamp(bleed * 1.4, 0.0, 0.85));

	// Green at the foot, where the wet never quite leaves.
	float damp = smoothstep(9.0, 4.0, world.y) * noise(vec2(at.x * 0.8, at.y * 1.3) + 7.0);
	surface = mix(surface, surface * vec3(0.55, 0.68, 0.5), clamp(damp * age * 1.3, 0.0, 1.0));

	// A little cold light down from the slots in the deck. Everything else is real light,
	// since this ignores shadow and shape and flattens whatever it touches.
	float sky = smoothstep(14.0, 34.0, world.y);
	vec3 light = wash_colour * (0.04 + sky * 0.14);

	ALBEDO = surface;
	ROUGHNESS = roughness;
	NORMAL_MAP = normal_map;
	NORMAL_MAP_DEPTH = 1.5;
	AO_LIGHT_AFFECT = 0.6;
	EMISSION = surface * light;
}
