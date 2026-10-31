shader_type spatial;
// A neon sign with no art: rows of blocky made-up glyphs, bright on a dark panel, in
// whichever language the city speaks. Each sign's text comes from where it hangs.
render_mode unshaded, cull_disabled;

uniform vec3 glow : source_color = vec3(1.0, 0.2, 0.6);
uniform vec2 size = vec2(2.0, 8.0);
uniform float text = 0.0;

float hash(vec2 p) {
	return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453);
}

void fragment() {
	vec2 metres = UV * size;
	float glyph_size = min(size.x, size.y) * 0.72;
	vec2 inset = (size - glyph_size * floor(size / glyph_size)) * 0.5;
	vec2 cell = floor((metres - inset) / glyph_size);
	vec2 inside = fract((metres - inset) / glyph_size);
	// A 4 by 4 grid of strokes, half of them on, makes something that reads as script.
	vec2 stroke = floor(inside * 4.0);
	float on = step(0.48, hash(stroke + cell * 17.0 + text * 3.1));
	vec2 pad = step(vec2(0.14), inside) * step(inside, vec2(0.86));
	float lit = on * pad.x * pad.y;
	vec2 edge = smoothstep(vec2(0.0), vec2(0.08), metres) * smoothstep(size, size - 0.08, metres);
	ALBEDO = mix(glow * 0.05, glow * 2.6, lit) * edge.x * edge.y + glow * 0.5 * (1.0 - edge.x * edge.y);
}
