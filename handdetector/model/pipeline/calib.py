"""Stage 4: build the int8 calibration set.

Post-training quantization runs these images through the float model to
find each activation's value range, which then fixes its int8 scale and
zero point. The set should look like what the device will see:

- the 3 zoo palm photos, augmented: flips, smaller/offset hands (zoom out
  into the letterbox), brightness/contrast, sensor noise including the
  OV2640's horizontal row noise (see ../docs/frame-grabbing.md);
- every JPEG in data/camera/ -- frames from our own board, captured with
  `make capture` (hands and no hands, our lighting);
- every JPEG in data/extra/ -- anything else you drop in.

Output: build/rRES/calib.npy, float32 [n, RES, RES, 3] (NHWC, the layout
onnx2tf's -cind expects), values in [0, 1].
"""
import numpy as np

from common import DATA, OUT, RES, letterbox, load_rgb, palm_samples

rng = np.random.default_rng(0)


def augment(rgb):
    """A handful of variants of one source image (HWC uint8)."""
    import cv2
    out = [rgb, rgb[:, ::-1]]
    h, w = rgb.shape[:2]
    for zoom in (0.5, 0.7):  # hand further away, at random offsets
        canvas = np.zeros_like(rgb)
        sw, sh = int(w * zoom), int(h * zoom)
        small = cv2.resize(rgb, (sw, sh))
        x, y = rng.integers(0, w - sw), rng.integers(0, h - sh)
        canvas[y:y + sh, x:x + sw] = small
        out.append(canvas)
    for gain, bias in ((0.6, 0), (1.3, 10), (0.9, -20)):
        out.append(np.clip(rgb.astype(np.float32) * gain + bias, 0, 255).astype(np.uint8))
    noisy = rgb.astype(np.float32)
    noisy += rng.normal(0, 8, noisy.shape)                    # pixel noise
    noisy += rng.normal(0, 12, (h, 1, 3))                     # row noise
    out.append(np.clip(noisy, 0, 255).astype(np.uint8))
    return out


def main():
    OUT.mkdir(parents=True, exist_ok=True)
    samples, counts = [], {}
    for jpg, _ in palm_samples():
        for v in augment(load_rgb(jpg)):
            samples.append(letterbox(v, RES)[0])
        counts["zoo palms (augmented)"] = counts.get("zoo palms (augmented)", 0) + 9
    for sub in ("camera", "extra"):
        files = sorted((DATA / sub).glob("*.jpg"))
        for jpg in files:
            samples.append(letterbox(load_rgb(jpg), RES)[0])
        counts[f"data/{sub}"] = len(files)
    calib = np.stack(samples).astype(np.float32)
    np.save(OUT / "calib.npy", calib)
    print(f"  calibration set {calib.shape}: " + ", ".join(f"{k} {v}" for k, v in counts.items()))


if __name__ == "__main__":
    main()
