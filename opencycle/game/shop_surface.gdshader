shader_type spatial;
// An awning's canvas, painted its stripes and glowing with the shop's light coming through it.

uniform float glow = 0.75;
uniform float roughness_of = 0.8;

void fragment() {
	ALBEDO = COLOR.rgb;
	ROUGHNESS = roughness_of;
	EMISSION = ALBEDO * glow;
}
