shader_type spatial;
// Tree cards. Lit as if the crown were one rounded mass: the fan's normals point
// outward from the trunk, and unlike StandardMaterial3D nothing flips them on a
// card's far side, which would light each tree in hard two-tone stripes.
render_mode cull_disabled;

uniform sampler2D albedo : source_color, filter_linear_mipmap_anisotropic;
uniform float alpha_cut = 0.35;
uniform vec3 translucency = vec3(0.07, 0.09, 0.04);
uniform vec3 tint = vec3(0.4, 0.46, 0.36);
// The card's half width, to find how deep in the crown a pixel is.
uniform float half_width = 4.0;
// How dark the bottom of the tree and the inside of its crown get from their own shade.
uniform float base_shade = 0.6;
uniform float core_shade = 0.62;

varying vec3 crown_normal;
varying float height;
varying float reach;

void vertex() {
	crown_normal = (MODELVIEW_MATRIX * vec4(NORMAL, 0.0)).xyz;
	height = 1.0 - UV.y;
	reach = length(VERTEX.xz) / half_width;
}

void fragment() {
	vec4 sampled = texture(albedo, UV);
	float depth_shade = mix(core_shade, 1.0, smoothstep(0.0, 0.85, reach));
	float low_shade = mix(base_shade, 1.0, smoothstep(0.0, 0.75, height));
	ALBEDO = sampled.rgb * COLOR.rgb * tint * depth_shade * low_shade;
	ALPHA = sampled.a;
	ALPHA_SCISSOR_THRESHOLD = alpha_cut;
	NORMAL = normalize(crown_normal);
	ROUGHNESS = 0.85;
	BACKLIGHT = translucency;
}
