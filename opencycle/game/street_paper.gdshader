shader_type spatial;
// What gets stuck or sprayed on a shutter, from the sign sheet. Bills are paper, faded by the
// wet and torn by whoever came next. With a spray, only the strokes of the design are kept,
// as paint in the colour of COLOR, the way loan sharks mark a street.

uniform sampler2D sheet : source_color, filter_linear_mipmap_anisotropic;
uniform bool spray = false;

varying vec3 world;

float hash(vec2 p) {
	return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453);
}

float noise(vec2 p) {
	vec2 i = floor(p);
	vec2 f = fract(p);
	f = f * f * (3.0 - 2.0 * f);
	return mix(
			mix(hash(i), hash(i + vec2(1.0, 0.0)), f.x),
			mix(hash(i + vec2(0.0, 1.0)), hash(i + vec2(1.0, 1.0)), f.x),
			f.y);
}

void vertex() {
	world = (MODEL_MATRIX * vec4(VERTEX, 1.0)).xyz;
}

void fragment() {
	vec2 at = vec2(world.x + world.z, world.y);
	vec3 design = texture(sheet, UV).rgb;
	float bright = dot(design, vec3(0.3, 0.6, 0.1));
	float grit = noise(at * 90.0);
	float run = noise(vec2(at.x * 14.0, at.y * 0.8));
	if (spray) {
		// Dry at the edge of each stroke, and thin where the shutter's been hosed.
		if (bright + (grit - 0.5) * 0.3 + (run - 0.5) * 0.25 < 0.42) {
			discard;
		}
		ALBEDO = COLOR.rgb * (0.7 + grit * 0.4);
		ROUGHNESS = 0.55;
	} else {
		float rip = noise(at * 5.0) + noise(at * 31.0) * 0.2;
		if (rip > 1.04) {
			discard;
		}
		// The white of the paper shows along a tear.
		float torn = smoothstep(0.98, 1.03, rip);
		float grey = dot(design, vec3(0.33));
		vec3 faded = mix(vec3(grey), design, 0.6) * 0.55 + vec3(0.09, 0.085, 0.07);
		faded *= mix(1.0, 0.5, smoothstep(0.45, 0.8, run));
		ALBEDO = mix(faded, vec3(0.55, 0.53, 0.47), torn) * COLOR.rgb;
		ROUGHNESS = mix(0.8, 0.35, run);
	}
	METALLIC = 0.0;
}
