shader_type spatial;
// Raw concrete decades after anyone last looked after it. A scanned finish laid out in world
// metres, so it runs on across every fin and slab, with rain, rust and patching on top. The
// patches of light on it are painted in: a canyon this dark needs them more than it needs
// them to be accurate. COLOR is the tint, with the building's age in alpha.

uniform sampler2D albedo : source_color, filter_linear_mipmap_anisotropic, repeat_enable;
uniform sampler2D normals : hint_normal, filter_linear_mipmap_anisotropic, repeat_enable;
uniform sampler2D rough : hint_roughness_g, filter_linear_mipmap_anisotropic, repeat_enable;
uniform float tile = 3.0;
uniform vec3 wash_colour = vec3(0.45, 0.62, 0.8);
uniform vec3 sodium_colour = vec3(1.0, 0.55, 0.22);

varying vec3 world;
varying vec3 world_normal;

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
}

void fragment() {
	vec3 n = abs(world_normal);
	vec2 at = n.y > 0.5
			? world.xz
			: vec2(world.x * n.z + world.z * n.x, world.y);
	float age = COLOR.a;
	vec3 scan = texture(albedo, at / tile).rgb;
	float grey = dot(scan, vec3(0.3, 0.6, 0.1));
	vec3 surface = mix(vec3(grey), scan, 0.55) * COLOR.rgb;
	float roughness = texture(rough, at / tile).g;

	// Patches where somebody cast a repair, each a slightly different pour.
	vec2 pour = floor(at / vec2(3.7, 2.3));
	float repair = step(0.9, hash(pour + 4.0)) * (0.8 + hash(pour) * 0.5);
	surface *= mix(1.0, repair, step(0.01, repair));

	// Long dark runs, thin and close together, the wettest of them shining.
	float runs = noise(vec2(at.x * 2.6, at.y * 0.07));
	float fine = noise(vec2(at.x * 11.0, at.y * 0.22));
	float blotch = noise(at * 0.13);
	float dirt = smoothstep(0.35, 0.8, runs * 0.6 + fine * 0.25 + blotch * 0.3);
	dirt = clamp(dirt * (0.45 + age * 0.75), 0.0, 1.0);
	surface *= mix(1.0, 0.5, dirt);
	roughness = mix(roughness, 0.22, dirt * 0.8);

	// Rust bleeding from the rebar and the fixings, in rare narrow lines.
	float line_of_rust = smoothstep(0.82, 0.92, noise(vec2(at.x * 4.3, at.y * 0.05) + 19.0));
	float bleed = line_of_rust * noise(vec2(at.x * 19.0, at.y * 0.4)) * age;
	surface = mix(surface, vec3(0.2, 0.085, 0.035), clamp(bleed * 1.4, 0.0, 0.85));

	// Green at the foot, where the wet never quite leaves.
	float damp = smoothstep(9.0, 4.0, world.y) * noise(vec2(at.x * 0.8, at.y * 1.3) + 7.0);
	surface = mix(surface, surface * vec3(0.55, 0.68, 0.5), clamp(damp * age * 1.3, 0.0, 1.0));

	// Pools of light up the walls, far enough apart that the dark between them stays dark.
	float side = n.y > 0.5 ? 0.25 : 1.0;
	float bay = floor(at.x / 23.0);
	float centre = (bay + 0.2 + hash(vec2(bay, 2.0)) * 0.6) * 23.0;
	float height = 7.0 + hash(vec2(bay, 5.0)) * 14.0;
	float across = (at.x - centre) / 6.5;
	float rise = (world.y - height) / 7.0;
	float pool = exp(-(across * across + rise * rise));
	vec3 tone = hash(vec2(bay, 9.0)) > 0.78 ? sodium_colour : wash_colour;
	// And a cold wash coming down from the slots in the deck.
	float sky = smoothstep(12.0, 34.0, world.y);
	vec3 light = wash_colour * (0.16 + sky * 0.5) + tone * pool * 1.5;

	ALBEDO = surface;
	ROUGHNESS = roughness;
	NORMAL_MAP = texture(normals, at / tile).xyz;
	NORMAL_MAP_DEPTH = 1.4;
	EMISSION = surface * light * side;
}
