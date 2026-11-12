"""Stage 6: full detection on still images, with ground truth.

Runs the whole chain (load -> letterbox -> engine -> decode -> NMS) on
the zoo's test palms, draws the result into build/detect/<engine>/, and
reports the IoU of the best box with the ground-truth box.

    python pipeline/detect.py                       # coreml-ane on the zoo palms
    python pipeline/detect.py -e onnx-cpu my.jpg    # any engine, any image
"""
import argparse
import pathlib

import cv2

from common import BUILD, decode, draw, iou, letterbox, load_rgb, palm_samples
from engines import make


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-e", "--engine", default="coreml-ane")
    ap.add_argument("images", nargs="*", type=pathlib.Path)
    args = ap.parse_args()

    samples = [(p, None) for p in args.images] or list(palm_samples())
    engine = make(args.engine)
    out_dir = BUILD / "detect" / args.engine
    out_dir.mkdir(parents=True, exist_ok=True)

    for path, gt in samples:
        rgb = load_rgb(path)
        x, lb = letterbox(rgb)
        dets = decode(*engine.infer(x), lb)
        bgr = draw(cv2.cvtColor(rgb, cv2.COLOR_RGB2BGR), dets)
        if gt:
            cv2.rectangle(bgr, (int(gt[0]), int(gt[1])), (int(gt[2]), int(gt[3])), (255, 128, 0), 1)
        cv2.imwrite(str(out_dir / path.name), bgr)
        best = f"score {dets[0]['score']:.3f}" if dets else "no hand"
        match = f", IoU with ground truth {iou(dets[0]['box'], gt):.2f}" if dets and gt else ""
        print(f"{path.name}: {len(dets)} detection(s), {best}{match}")
    print(f"images in {out_dir.relative_to(BUILD.parent)}/")


if __name__ == "__main__":
    main()
