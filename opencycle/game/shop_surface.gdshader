shader_type spatial;
// A surface in a shop: tile or canvas painted the trade's colour, glowing as if lit by the
// ceiling tubes, so it falls off towards the floor. UV.y runs up a wall from the floor.

uniform sampler2D surface : source_color, filter_linear_mipmap_anisotropic;
uniform sampler2D normals : hint_normal, filter_linear_mipmap_anisotropic;
uniform float glow = 0.5;
uniform float roughness = 0.4;
// How much of the fall-off to use, and the UV height it takes to reach full brightness.
uniform float fall_off = 0.0;
uniform float full_at = 1.0;

void fragment() {
	ALBEDO = texture(surface, UV).rgb * COLOR.rgb;
	NORMAL_MAP = texture(normals, UV).rgb;
	ROUGHNESS = roughness;
	float lit = mix(1.0, mix(0.3, 1.0, smoothstep(0.0, full_at, UV.y)), fall_off);
	EMISSION = ALBEDO * glow * lit;
}
