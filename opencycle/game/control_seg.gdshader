shader_type spatial;
// The segmentation control pass: one flat colour for everything drawn with the same shader,
// cut where the surface it stands in for is cut.
render_mode unshaded, cull_disabled, shadows_disabled, fog_disabled;

uniform vec3 colour : source_color;
uniform sampler2D cutout : source_color, filter_linear_mipmap;
uniform bool cut = false;
uniform float cut_at = 0.35;

void fragment() {
	ALBEDO = colour;
	if (cut && texture(cutout, UV).a < cut_at) {
		discard;
	}
}
