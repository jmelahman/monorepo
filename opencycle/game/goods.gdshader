shader_type spatial;
// A row of stock on a shelf, from the sheet tools/bake_goods.gd renders out of real models.
// The room's tubes are in the colour, and the glow keeps it bright without a light per shelf.

uniform sampler2D sheet : source_color, filter_linear_mipmap_anisotropic, repeat_disable;
uniform float glow = 0.5;

void fragment() {
	vec4 card = texture(sheet, UV);
	ALBEDO = card.rgb * COLOR.rgb;
	ALPHA = card.a;
	ALPHA_SCISSOR_THRESHOLD = 0.5;
	ROUGHNESS = 0.45;
	EMISSION = card.rgb * COLOR.rgb * glow;
}
