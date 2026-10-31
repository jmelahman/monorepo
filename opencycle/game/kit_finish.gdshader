shader_type spatial;
// A part of the building kit, repainted: the kit's plaster and trim keep their shape and
// wear, but lose their brown, so a building can be any colour. COLOR is the paint. Rain
// carries dirt down the wall in streaks, worked out in the world so modules don't repeat.

uniform sampler2D albedo : source_color, filter_linear_mipmap_anisotropic, repeat_enable;
uniform sampler2D normals : hint_normal, filter_linear_mipmap_anisotropic, repeat_enable;
uniform sampler2D packed : filter_linear_mipmap_anisotropic, repeat_enable;
uniform float grey = 0.85;
uniform float grime = 0.6;

varying vec3 world;
varying vec3 paint;

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
	paint = COLOR.rgb;
}

void fragment() {
	vec3 base = texture(albedo, UV).rgb;
	vec3 arm = texture(packed, UV).rgb;
	float value = dot(base, vec3(0.3, 0.59, 0.11));
	vec3 colour = mix(base, vec3(value), grey) * paint * 1.7;
	float along = world.x + world.z;
	float streaks = noise(vec2(along * 2.1, world.y * 0.1)) * 0.65 + noise(vec2(along, world.y) * 0.4) * 0.45;
	float dirt = smoothstep(0.42, 0.8, streaks) * grime;
	ALBEDO = colour * mix(1.0, 0.35, dirt);
	NORMAL_MAP = texture(normals, UV).xyz;
	// It's raining: anything clean is a bit shiny, and the dirt holds the water less.
	ROUGHNESS = arm.g * mix(0.6, 1.0, dirt);
	METALLIC = arm.b;
	AO = arm.r;
}
