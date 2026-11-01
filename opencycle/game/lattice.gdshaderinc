// Smooth noise read from a tiling texture, which costs one lookup where hashing the four
// corners in the shader costs four sines, and holds its precision kilometres from the origin.
// The material needs Lattice.texture() set as lattice.

uniform sampler2D lattice : filter_linear, repeat_enable;

float noise(vec2 p) {
	vec2 cell = floor(p);
	vec2 f = p - cell;
	f = f * f * (3.0 - 2.0 * f);
	return textureLod(lattice, (mod(cell, 256.0) + f + 0.5) / 256.0, 0.0).r;
}
