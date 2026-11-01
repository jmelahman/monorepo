shader_type spatial;
// Paint on a road decades old: tyres have taken it off in patches and along the grain of the
// traffic, so the asphalt shows through, and what's left is stained. Mapped in world space
// so no two arrows wear alike.

uniform vec3 paint = vec3(0.45, 0.45, 0.42);
// How much of it has gone, from 0 to 1.
uniform float worn = 0.4;

varying vec3 world;

#include "res://game/road_wear.gdshaderinc"

void vertex() {
	world = (MODEL_MATRIX * vec4(VERTEX, 1.0)).xyz;
}

void fragment() {
	vec2 at = wear_at(world.xz);
	float grit = wear_grit(at, length(fwidth(world.xz)));
	if (worn_through(at, grit, worn)) {
		discard;
	}
	ALBEDO = paint * mix(0.55, 1.0, noise(at * 7.0)) * mix(0.75, 1.0, grit);
	ROUGHNESS = 0.55;
	SPECULAR = 0.4;
}
