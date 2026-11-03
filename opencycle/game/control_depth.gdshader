shader_type spatial;
// The depth control pass: a quad pinned over the screen that turns what's been drawn into
// inverse distance, white within near_metres and falling to black at the sky, as depth
// models see it. Reading the depth buffer keeps a tree card's cut-out edges.
render_mode unshaded, cull_disabled, depth_test_disabled, depth_draw_never, shadows_disabled, fog_disabled;

uniform sampler2D depth : hint_depth_texture, filter_nearest;
uniform float near_metres = 2.0;

void vertex() {
	POSITION = vec4(VERTEX.xy, 1.0, 1.0);
}

void fragment() {
	float raw = texture(depth, SCREEN_UV).r;
	vec4 view = INV_PROJECTION_MATRIX * vec4(SCREEN_UV * 2.0 - 1.0, raw, 1.0);
	float metres = -view.z / view.w;
	ALBEDO = vec3(clamp(near_metres / metres, 0.0, 1.0));
	// Drawn with the transparent pass, after the depth buffer is complete.
	ALPHA = 1.0;
}
