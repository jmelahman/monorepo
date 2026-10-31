shader_type spatial;
// Tree cards. Lit as if the crown were one rounded mass: the fan's normals point
// outward from the trunk, and unlike StandardMaterial3D nothing flips them on a
// card's far side, which would light each tree in hard two-tone stripes.
render_mode cull_disabled;

uniform sampler2D albedo : source_color, filter_linear_mipmap_anisotropic;
uniform float alpha_cut = 0.35;
uniform vec3 translucency = vec3(0.3, 0.35, 0.15);
// How dark the bottom of the tree gets from its own shade.
uniform float base_shade = 0.55;
uniform float brightness = 0.8;

varying vec3 crown_normal;
varying float height;

void vertex() {
	crown_normal = (MODELVIEW_MATRIX * vec4(NORMAL, 0.0)).xyz;
	height = 1.0 - UV.y;
}

void fragment() {
	vec4 sampled = texture(albedo, UV);
	ALBEDO = sampled.rgb * COLOR.rgb * brightness * mix(base_shade, 1.0, smoothstep(0.0, 0.75, height));
	ALPHA = sampled.a;
	ALPHA_SCISSOR_THRESHOLD = alpha_cut;
	NORMAL = normalize(crown_normal);
	ROUGHNESS = 0.85;
	BACKLIGHT = translucency;
}
