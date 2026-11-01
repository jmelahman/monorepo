class_name Lattice
extends RefCounted
## The texture behind [code]game/lattice.gdshaderinc[/code]: one random value per texel.

const SIZE := 256

static var _texture: ImageTexture


static func texture() -> ImageTexture:
	if _texture:
		return _texture
	var values := PackedByteArray()
	values.resize(SIZE * SIZE)
	var derived := Rng.derive(0, [SIZE])
	for index: int in values.size():
		values[index] = int(Rng.unit(derived, index) * 255.0)
	_texture = ImageTexture.create_from_image(
		Image.create_from_data(SIZE, SIZE, false, Image.FORMAT_R8, values)
	)
	return _texture
