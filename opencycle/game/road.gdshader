shader_type spatial;
// The road: black asphalt after rain, darker in the low spots and along the crown's edge,
// with puddles where it dips. What it reflects comes from game/wet_sheen.gdshader.

uniform sampler2D albedo : source_color, filter_linear_mipmap_anisotropic, repeat_enable;
uniform sampler2D normals : hint_normal, filter_linear_mipmap_anisotropic, repeat_enable;
uniform sampler2D rough : hint_roughness_g, filter_linear_mipmap_anisotropic, repeat_enable;
uniform float tile = 3.0;
uniform float wetness = 0.55;
uniform vec3 tint : source_color = vec3(1.0);

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
	vec2 uv = UV / tile;
	vec3 base = texture(albedo, uv).rgb;
	vec3 normal_map = texture(normals, uv).xyz;
	float dry = texture(rough, uv).g;
	vec2 at = world.xz;
	float broad = noise(at * 0.13) * 0.6 + noise(at * 0.47) * 0.32 + noise(at * 1.9) * 0.08;
	// Tyres wear a sheen down the middle of each lane, and water stands at the edges.
	float gutter = smoothstep(2.6, 4.0, abs(UV.x));
	float damp = clamp(broad * 1.2 - 0.25 + gutter * 0.3, 0.0, 1.0) * wetness;
	float puddle = smoothstep(0.62, 0.67, broad);
	base *= tint * mix(0.5, 0.22, damp);
	base = mix(base, base * 0.25, puddle);
	ALBEDO = base;
	ROUGHNESS = mix(mix(dry, 0.22, damp), 0.035, puddle);
	SPECULAR = 0.55;
	NORMAL_MAP = mix(normal_map, vec3(0.5, 0.5, 1.0), max(puddle, damp * 0.45));
}
