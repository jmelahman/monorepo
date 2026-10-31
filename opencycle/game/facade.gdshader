shader_type spatial;
// A building's wall: a real finish in metres of UV, with windows cut into it that have a
// reveal, a frame and a room behind them, worked out by tracing the view ray in tangent
// space. That parallax, not geometry, is what makes a flat box read as a building.
// UV2 is [bay, storey], and a bay of 0 means a blank party wall. COLOR is the tint, with
// the building's age in alpha.

uniform sampler2D albedo : source_color, filter_linear_mipmap_anisotropic, repeat_enable;
uniform sampler2D normals : hint_normal, filter_linear_mipmap_anisotropic, repeat_enable;
uniform sampler2D rough : hint_roughness_g, filter_linear_mipmap_anisotropic, repeat_enable;
uniform float tile = 2.0;
uniform float first_storey = 4.2;
uniform float recess = 0.2;
uniform float room_depth = 4.0;

varying vec2 layout;
varying vec3 tint;
varying float age;

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
	layout = UV2;
	tint = COLOR.rgb;
	age = COLOR.a;
}

// What the view ray sees in the room behind a window, as light given off.
vec3 room(vec2 at, vec3 ray, vec2 half_room, vec2 id, float lit) {
	vec3 start = vec3(at, 0.0);
	vec3 inverse_ray = 1.0 / ray;
	vec3 to_wall = (sign(ray) * vec3(half_room, room_depth * 0.5)
						   - (start - vec3(0.0, 0.0, -room_depth * 0.5))) * inverse_ray;
	float t = min(min(to_wall.x, to_wall.y), to_wall.z);
	vec3 hit = start + ray * t;
	float tone = hash(id + 3.1);
	vec3 paint = mix(vec3(0.85, 0.8, 0.68), vec3(0.6, 0.72, 0.8), tone) * (0.55 + hash(id) * 0.45);
	vec3 surface = paint;
	if (to_wall.y <= to_wall.x && to_wall.y <= to_wall.z) {
		if (ray.y > 0.0) {
			// The ceiling, with the light fitting that lights the room.
			float fitting = smoothstep(0.34, 0.2, length(hit.xz + vec2(0.0, room_depth * 0.5)));
			surface = paint * 0.9 + vec3(5.0) * fitting;
		} else {
			surface = paint * vec3(0.35, 0.28, 0.22);
		}
	} else if (to_wall.z < to_wall.x) {
		// The far wall, and the shelves or pictures against it.
		float furniture = step(0.5, hash(id + floor(hit.x * 1.3))) * step(hit.y, 0.1);
		surface = mix(paint, paint * 0.35, furniture);
	}
	float falloff = mix(0.55, 1.0, smoothstep(-room_depth, 0.0, hit.z));
	vec3 lamp = mix(vec3(1.0, 0.72, 0.42), vec3(0.85, 0.95, 1.0), step(0.62, hash(id + 9.7)));
	return surface * lamp * falloff * lit;
}

void fragment() {
	vec2 at = UV;
	vec2 wall_uv = at / tile;
	vec3 wall = texture(albedo, wall_uv).rgb * tint;
	vec3 wall_normal = texture(normals, wall_uv).xyz;
	float wall_rough = texture(rough, wall_uv).g;
	// Rain carries dirt down from every ledge. Older buildings have more of it.
	float column = noise(vec2(at.x * 2.3, at.y * 0.09));
	float patchy = noise(at * 0.35);
	float grime = smoothstep(0.42, 0.8, column * 0.65 + patchy * 0.45) * age;
	wall *= mix(1.0, 0.42, grime);
	wall_rough = mix(wall_rough, 0.92, grime);

	vec3 colour = wall;
	vec3 glow = vec3(0.0);
	float roughness = wall_rough;
	float metallic = 0.0;
	vec3 normal_map = wall_normal;
	float occlusion = 1.0;

	if (layout.x > 0.1 && at.y > first_storey) {
		vec2 cell_size = layout;
		vec2 grid = vec2(at.x, at.y - first_storey) / cell_size;
		vec2 cell = floor(grid);
		vec2 centred = (fract(grid) - vec2(0.5, 0.56)) * cell_size;
		vec2 id = cell + layout * 17.0;
		vec2 half_opening = vec2(cell_size.x * 0.34, cell_size.y * 0.27);
		// A dirty streak runs down from each sill.
		float below = -centred.y - half_opening.y;
		if (abs(centred.x) < half_opening.x && below > 0.0) {
			float streak = noise(vec2(at.x * 11.0, cell.y));
			colour *= 1.0 - smoothstep(1.1, 0.0, below) * streak * 0.5 * (0.35 + age);
		}
		float pixel = fwidth(at.x);
		// Beyond where a window is a few pixels wide, tracing it would only sparkle.
		float detail = 1.0 - smoothstep(0.08, 0.2, pixel);
		vec3 ray = vec3(
				dot(-VIEW, TANGENT), dot(-VIEW, BINORMAL), min(dot(-VIEW, NORMAL), -0.02));
		vec2 at_glass = centred + ray.xy * (recess / -ray.z);
		vec2 outer = abs(centred) - half_opening;
		float in_opening = 1.0 - step(0.0, max(outer.x, outer.y));
		float lit = step(0.62, hash(id));
		if (in_opening > 0.5) {
			vec2 inner = abs(at_glass) - half_opening;
			float on_glass = 1.0 - step(0.0, max(inner.x, inner.y));
			if (on_glass > 0.5) {
				float frame = step(abs(at_glass.x), 0.022)
						+ step(half_opening.x - 0.035, abs(at_glass.x))
						+ step(half_opening.y - 0.035, abs(at_glass.y));
				if (frame > 0.5) {
					colour = vec3(0.09);
					roughness = 0.45;
					normal_map = vec3(0.5, 0.5, 1.0);
				} else {
					vec3 seen = room(at_glass, ray, cell_size * 0.5 + vec2(0.4, 0.3), id, lit);
					// Most windows have a blind or a curtain part of the way across.
					float blind = hash(id + 5.0);
					float drawn = smoothstep(0.02, 0.0, at_glass.y
							- half_opening.y * (1.0 - 2.0 * blind * blind));
					vec3 cloth = mix(vec3(0.95, 0.78, 0.55), vec3(0.72, 0.8, 0.82), hash(id + 2.0));
					seen = mix(cloth * lit * 0.9, seen, drawn);
					colour = vec3(0.012);
					roughness = 0.08;
					metallic = 0.0;
					normal_map = vec3(0.5, 0.5, 1.0);
					glow = seen * (0.45 + hash(id + 8.0) * 0.55) * detail;
					// Further off, a lit window is a flat glow of about the same strength.
					glow += (1.0 - detail) * lit * vec3(0.8, 0.62, 0.42) * 0.6 * (1.0 - drawn * 0.5);
				}
			} else {
				// The reveal: the thickness of the wall.
				colour = wall * 0.45;
				occlusion = 0.45;
				normal_map = vec3(0.5, 0.5, 1.0);
			}
		}
		// A ledge above and below each window catches the eye before anything else does.
		float ledge = step(abs(centred.x), half_opening.x + 0.08)
				* (step(abs(-centred.y - half_opening.y - 0.03), 0.04));
		if (ledge > 0.5) {
			colour = wall * 0.75;
		}
	}
	ALBEDO = colour;
	ROUGHNESS = roughness;
	METALLIC = metallic;
	NORMAL_MAP = normal_map;
	AO = occlusion;
	AO_LIGHT_AFFECT = 0.7;
	EMISSION = glow;
}
