shader_type spatial;
// Sign faces, lit from within; the picture comes from the baked sheet. COLOR.r is a
// sign's own brightness, COLOR.g how badly it flickers, COLOR.b the seed for when.

global uniform float ride_time;

uniform sampler2D sheet : source_color, filter_linear_mipmap_anisotropic;
uniform float brightness = 1.5;

float hash(float p) {
	return fract(sin(p * 91.3458) * 47453.5453);
}

void fragment() {
	vec3 picture = texture(sheet, UV).rgb;
	float t = ride_time + COLOR.b * 97.0;
	float stutter = step(0.93, hash(floor(t * 11.0) + COLOR.b));
	float flicker = 1.0 - COLOR.g * stutter * 0.85;
	ALBEDO = picture * 0.08;
	ROUGHNESS = 0.4;
	SPECULAR = 0.3;
	EMISSION = picture * COLOR.r * brightness * flicker;
}
