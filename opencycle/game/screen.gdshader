shader_type spatial;
// A giant LED wall running a loop of adverts off the ride's clock: a sign from the sheet,
// colour washing through it, and hard bars sweeping across. Up close, the LEDs show.

global uniform float ride_time;

uniform sampler2D sheet : source_color, filter_linear_mipmap_anisotropic;
uniform vec4 picture = vec4(0.0, 0.0, 1.0, 1.0);
uniform vec3 colour_a : source_color = vec3(1.0, 0.1, 0.6);
uniform vec3 colour_b : source_color = vec3(0.1, 0.8, 1.0);
uniform vec2 size = vec2(10.0, 10.0);
uniform float phase = 0.0;
uniform float brightness = 2.4;

const float SCENE_SECONDS = 4.5;

void fragment() {
	float t = ride_time + phase * 60.0;
	float scene = floor(t / SCENE_SECONDS);
	float into = fract(t / SCENE_SECONDS);
	// Fills slowly and at a slant, like a video zooming in.
	vec2 at = UV;
	vec2 zoomed = (at - 0.5) * (1.0 - into * 0.08) + 0.5;
	vec3 art = texture(sheet, picture.xy + zoomed * picture.zw).rgb;
	float wave = 0.5 + 0.5 * sin(at.y * 5.0 + t * 1.2 + 2.0 * sin(at.x * 4.0 - t * 0.7));
	vec3 wash = mix(colour_a, colour_b, wave);
	float kind = mod(scene, 3.0);
	vec3 shown = art;
	if (kind > 1.5) {
		// Just the sign's lettering, cut out of the wash.
		float ink = dot(art, vec3(0.33));
		shown = wash * (0.25 + smoothstep(0.35, 0.8, ink) * 1.3);
	} else if (kind > 0.5) {
		float bars = step(0.5, fract((at.x + at.y * 0.35) * 6.0 - t * 0.9));
		shown = mix(colour_a, colour_b, bars) * (0.5 + 0.5 * wave);
		shown = mix(shown, art, smoothstep(0.3, 0.7, dot(art, vec3(0.33))));
	}
	// One scene wipes on over the last, in a flash of white.
	float wipe = smoothstep(0.08, 0.0, abs(at.y - (1.0 - into * 12.0)));
	shown += vec3(1.5) * wipe;
	vec2 led = abs(fract(at * size * 7.0) - 0.5);
	float dots = smoothstep(0.5, 0.3, max(led.x, led.y));
	float pixel = max(fwidth(at.x) * size.x, fwidth(at.y) * size.y) * 7.0;
	dots = mix(dots, 0.65, smoothstep(0.25, 0.7, pixel));
	float roll = 0.92 + 0.08 * sin(at.y * 40.0 - t * 5.0);
	ALBEDO = vec3(0.01);
	ROUGHNESS = 0.35;
	EMISSION = shown * dots * roll * brightness;
}
