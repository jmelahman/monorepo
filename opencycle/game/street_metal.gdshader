shader_type spatial;
// Sheet steel at street level: a scan laid on in UVs, tinted by COLOR, with what standing in
// the wet for decades does to it. Grime climbs from the pavement and runs down from the top.

uniform sampler2D albedo : source_color, filter_linear_mipmap_anisotropic, repeat_enable;
uniform sampler2D normals : hint_normal, filter_linear_mipmap_anisotropic, repeat_enable;
uniform sampler2D rough : hint_roughness_g, filter_linear_mipmap_anisotropic, repeat_enable;
uniform sampler2D occlusion : hint_default_white, filter_linear_mipmap_anisotropic, repeat_enable;
uniform float gain = 1.0;

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
	float along = world.x + world.z;
	vec3 scan = texture(albedo, UV).rgb * mix(0.6, 1.0, texture(occlusion, UV).r);
	float streaks = noise(vec2(along * 5.0, world.y * 0.25));
	float fine = noise(vec2(along * 23.0, world.y * 0.6));
	// Splashed up from the pavement, unevenly, and run down from the housing.
	float foot = smoothstep(1.3, 0.2, world.y - streaks * 0.5);
	float head = smoothstep(1.4, 2.9, world.y) * smoothstep(0.45, 0.8, streaks * 0.7 + fine * 0.4);
	float dirt = clamp(foot * 0.85 + head * 0.6, 0.0, 1.0);
	ALBEDO = scan * COLOR.rgb * 1.15 * gain * mix(1.0, 0.45, dirt);
	ROUGHNESS = mix(texture(rough, UV).g, 0.3, foot * 0.6);
	METALLIC = 0.1;
	NORMAL_MAP = texture(normals, UV).xyz;
	NORMAL_MAP_DEPTH = 1.6;
}
