shader_type spatial;
// The road's dirt shoulder, tinted to sit with the ground it meets. Its edge is
// eaten away by the texture's own grain so it never reads as a ruled line.

uniform sampler2D albedo : source_color, filter_linear_mipmap_anisotropic, repeat_enable;
uniform sampler2D normals : hint_normal, filter_linear_mipmap_anisotropic, repeat_enable;
uniform sampler2D roughness_map : hint_roughness_g, filter_linear_mipmap_anisotropic, repeat_enable;
uniform vec3 tint = vec3(0.5, 0.45, 0.38);
// Half the shoulder's width in UV units, and how much of it may be eaten away.
uniform float half_width = 1.0;
uniform float fray = 0.2;

void fragment() {
	vec4 sampled = texture(albedo, UV);
	ALBEDO = mix(sampled.rgb, vec3(dot(sampled.rgb, vec3(0.33))), 0.6) * tint;
	NORMAL_MAP = texture(normals, UV).rgb;
	ROUGHNESS = texture(roughness_map, UV).g;
	float grain = texture(albedo, UV * 0.37).r + texture(albedo, UV * 0.11).g - 0.4;
	float edge = (half_width - abs(UV.x)) / fray;
	ALPHA = edge + grain - 0.5;
	ALPHA_SCISSOR_THRESHOLD = 0.5;
}
