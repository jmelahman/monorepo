"""The sea's wave normal map: a tiling 1024 px square of wind waves, made from a wave spectrum.

Writes /workspace/ue/textures/T_SeaWaves_N.png (or the path given), which climb_world.py's
sea_material samples at two scales, drifting. Run with numpy and pillow:

    uv run -q --no-project --with numpy --with pillow python -I scripts/make_sea_waves.py
"""

import sys

import numpy as np
from PIL import Image

SIZE = 1024
OUT = sys.argv[1] if len(sys.argv) > 1 else "/workspace/ue/textures/T_SeaWaves_N.png"
# Waves from 4 to 90 repeats across the tile, mostly running with the wind (+x, a little +y),
# their height falling off as the wavelength shortens, as a sea's does.
LOW, HIGH = 4, 90
WIND = np.array([0.95, 0.3]) / np.hypot(0.95, 0.3)
SLOPE = 0.32  # the normals' RMS slope: steeper reads as chop, flatter as glass


def main():
    rng = np.random.default_rng(7)
    k = np.fft.fftfreq(SIZE, 1.0 / SIZE)
    kx, ky = np.meshgrid(k, k)
    length = np.hypot(kx, ky)
    band = (length >= LOW) & (length <= HIGH)
    along = np.where(length > 0, (kx * WIND[0] + ky * WIND[1]) / np.maximum(length, 1e-9), 0)
    # Mostly with the wind, some across it, none against it.
    spread = np.clip(along, 0, 1) ** 2 + 0.15
    amp = np.where(band, length.clip(1) ** -1.9 * spread, 0)
    amp *= rng.rayleigh(1.0, amp.shape)
    spectrum = amp * np.exp(2j * np.pi * rng.random(amp.shape))
    # The slopes, as the height's derivative: i k times the spectrum.
    gx = np.real(np.fft.ifft2(spectrum * 2j * np.pi * kx / SIZE))
    gy = np.real(np.fft.ifft2(spectrum * 2j * np.pi * ky / SIZE))
    scale = SLOPE / np.sqrt(np.mean(gx**2 + gy**2))
    n = np.stack([-gx * scale, -gy * scale, np.ones_like(gx)], axis=-1)
    n /= np.linalg.norm(n, axis=-1, keepdims=True)
    rgb = np.round((n * 0.5 + 0.5) * 255).clip(0, 255).astype(np.uint8)
    Image.fromarray(rgb, "RGB").save(OUT)
    print(f"{OUT}: {SIZE} px, slope {SLOPE}, z {n[..., 2].min():.2f}..1")


if __name__ == "__main__":
    main()
