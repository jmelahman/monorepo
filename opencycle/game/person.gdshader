shader_type spatial;
// One person on a quad that turns to the viewer about its upright. The sheet holds every
// side of them through a walk, so this picks the side the viewer is on and the walk's
// frame, and takes light from the normals baked with it. INSTANCE_CUSTOM is
// (heading, phase, height, walking).

uniform sampler2D albedo : source_color, filter_linear_mipmap_anisotropic;
uniform sampler2D normals : filter_linear_mipmap_anisotropic;
uniform int walk_frames = 8;
uniform int stand_frames = 2;
uniform int views = 8;

varying vec2 frame_a;
varying vec2 frame_b;
varying float blend;
varying vec3 view_right;
varying vec3 view_up;
varying vec3 view_back;

void vertex() {
	vec3 origin = MODEL_MATRIX[3].xyz;
	vec3 back = CAMERA_POSITION_WORLD - origin;
	back.y = 0.0;
	back = normalize(back);
	vec3 right = cross(vec3(0.0, 1.0, 0.0), back);
	float height = INSTANCE_CUSTOM.b;
	VERTEX = (right * VERTEX.x + vec3(0.0, VERTEX.y, 0.0)) * height;

	float seen_from = atan(back.x, back.z) - INSTANCE_CUSTOM.r;
	float view = mod(round(seen_from / TAU * float(views)), float(views));
	float first = 0.0;
	float count = float(walk_frames);
	if (INSTANCE_CUSTOM.a < 0.5) {
		first = float(walk_frames);
		count = float(stand_frames);
	}
	float along = INSTANCE_CUSTOM.g * count;
	float column = floor(along);
	blend = along - column;
	vec2 cell = vec2(1.0 / float(walk_frames + stand_frames), 1.0 / float(views));
	frame_a = (vec2(first + column, view) + UV) * cell;
	frame_b = (vec2(first + mod(column + 1.0, count), view) + UV) * cell;

	view_right = (VIEW_MATRIX * vec4(right, 0.0)).xyz;
	view_up = (VIEW_MATRIX * vec4(0.0, 1.0, 0.0, 0.0)).xyz;
	view_back = (VIEW_MATRIX * vec4(back, 0.0)).xyz;
}

void fragment() {
	vec4 sampled = mix(texture(albedo, frame_a), texture(albedo, frame_b), blend);
	// Imported as a normal map, which keeps only the two channels that can't be worked out.
	vec2 flat_normal = mix(texture(normals, frame_a), texture(normals, frame_b), blend).xy * 2.0 - 1.0;
	vec3 baked = vec3(flat_normal, sqrt(max(1.0 - dot(flat_normal, flat_normal), 0.0)));
	ALBEDO = sampled.rgb;
	ALPHA = sampled.a;
	ALPHA_SCISSOR_THRESHOLD = 0.45;
	NORMAL = normalize(view_right * baked.x + view_up * baked.y + view_back * baked.z);
	ROUGHNESS = 0.75;
	SPECULAR = 0.25;
}
