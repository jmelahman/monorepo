shader_type spatial;
// The needle cards of a modelled tree, from scripts/blender/pine.py. Like the baked cards in
// game/foliage.gdshader, they light as one rounded crown: their normals lean out from the trunk
// and nothing flips them on a card's far side. The vertex colour is how deep in the crown a card
// sits, times the instance's own shade.
render_mode cull_disabled;

uniform sampler2D albedo : source_color, filter_linear_mipmap_anisotropic;
uniform float alpha_cut = 0.4;
uniform vec3 translucency = vec3(0.07, 0.09, 0.04);
uniform vec3 tint = vec3(0.62, 0.8, 0.5);
// Needles are a pixel wide in the atlas, so its smaller mips average them into thin air. This
// much more alpha per mip keeps a distant crown as dense as a near one.
uniform float mip_fill = 0.15;

varying vec3 crown_normal;

void vertex() {
	crown_normal = (MODELVIEW_MATRIX * vec4(NORMAL, 0.0)).xyz;
}

void fragment() {
	vec4 sampled = texture(albedo, UV);
	float mip = textureQueryLod(albedo, UV).y;
	ALBEDO = sampled.rgb * COLOR.rgb * tint;
	ALPHA = sampled.a * (1.0 + max(mip, 0.0) * mip_fill);
	ALPHA_SCISSOR_THRESHOLD = alpha_cut;
	NORMAL = normalize(crown_normal);
	ROUGHNESS = 0.85;
	BACKLIGHT = translucency;
}
