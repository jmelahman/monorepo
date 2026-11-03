shader_type spatial;
// The forest floor: needle litter with moss creeping through it. The texture gives
// only brightness, at two scales to hide its tiling, and the colours come from here:
// its own red-brown, multiplied by itself, reads as bare clay. The vertex colour
// carries the broad light and dark, and its alpha where the moss grows.

uniform sampler2D albedo : source_color, filter_linear_mipmap_anisotropic, repeat_enable;
uniform sampler2D normals : hint_normal, filter_linear_mipmap_anisotropic, repeat_enable;
uniform sampler2D roughness_map : hint_roughness_g, filter_linear_mipmap_anisotropic, repeat_enable;
uniform sampler2D ao_map : hint_default_white, filter_linear_mipmap_anisotropic, repeat_enable;
uniform float tile = 2.0;
uniform vec3 litter = vec3(0.2, 0.17, 0.12);
uniform vec3 moss = vec3(0.1, 0.13, 0.05);

varying vec2 plan;

void vertex() {
	plan = (MODEL_MATRIX * vec4(VERTEX, 1.0)).xz;
	TANGENT = normalize(cross(vec3(0.0, 0.0, 1.0), NORMAL));
	BINORMAL = normalize(cross(TANGENT, NORMAL));
}

void fragment() {
	vec2 uv = plan / tile;
	vec3 fine = texture(albedo, uv).rgb;
	vec3 broad = texture(albedo, uv * 0.27 + 0.31).rgb;
	float grain = dot(fine, vec3(0.33));
	float detail = grain * 2.2 * (0.55 + dot(broad, vec3(0.33)) * 1.3);
	float mossy = smoothstep(0.32, 0.6, COLOR.a + (grain - 0.3) * 0.7);
	ALBEDO = mix(litter, moss, mossy) * detail * COLOR.rgb;
	NORMAL_MAP = texture(normals, uv).rgb;
	ROUGHNESS = texture(roughness_map, uv).g;
	AO = texture(ao_map, uv).r;
	AO_LIGHT_AFFECT = 0.3;
}
