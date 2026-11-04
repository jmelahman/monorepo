shader_type spatial;
// Grass, ferns and moss from Poly Haven. Their glTF diffuse is a JPEG, and each blade's cut-out is
// in a map of its own, which StandardMaterial3D can't read, so it drew every card solid, a leaf
// as broad as the card. The vertex colour is the instance's own shade.
render_mode cull_disabled;

uniform sampler2D albedo : source_color, filter_linear_mipmap_anisotropic;
uniform sampler2D cutout : hint_default_white, filter_linear_mipmap_anisotropic;
uniform sampler2D normals : hint_normal, filter_linear_mipmap_anisotropic;
// Occlusion, roughness and metal, as glTF packs them.
uniform sampler2D arm : hint_default_white, filter_linear_mipmap_anisotropic;
uniform float alpha_cut = 0.5;
uniform vec3 translucency = vec3(0.06, 0.08, 0.03);
// Blades are a few pixels wide, so the smaller mips average them away; as in
// game/needles.gdshader, this much more alpha per mip keeps a distant tuft as full as a near one.
uniform float mip_fill = 0.12;

void fragment() {
	ALBEDO = texture(albedo, UV).rgb * COLOR.rgb;
	float mip = textureQueryLod(cutout, UV).y;
	ALPHA = texture(cutout, UV).r * (1.0 + max(mip, 0.0) * mip_fill);
	ALPHA_SCISSOR_THRESHOLD = alpha_cut;
	NORMAL_MAP = texture(normals, UV).rgb;
	vec3 packed = texture(arm, UV).rgb;
	AO = packed.r;
	ROUGHNESS = packed.g;
	BACKLIGHT = translucency;
}
