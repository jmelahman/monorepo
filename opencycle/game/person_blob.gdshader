shader_type spatial;
// The soft shade a person's feet make on the ground.
render_mode unshaded, blend_mul, depth_draw_never, shadows_disabled;

void fragment() {
	float from_middle = length(UV - 0.5) * 2.0;
	ALBEDO = vec3(mix(0.35, 1.0, smoothstep(0.1, 1.0, from_middle)));
}
