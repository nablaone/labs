"""Stage 5: the same input through every engine.

For each engine: load time, first call (compilation / weight upload),
then the median and 90th percentile of N timed calls, and how far its
output is from ONNX Runtime CPU (float32), the reference. Writes
build/bench.md.

    python pipeline/bench.py                 # all engines
    python pipeline/bench.py coreml-ane onnx-cpu -n 500
"""
import argparse
import statistics
import time

import numpy as np

from common import BUILD, letterbox, load_rgb, md_table, palm_samples, decode
from engines import ENGINES, make


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("engines", nargs="*", default=list(ENGINES))
    ap.add_argument("-n", type=int, default=200, help="timed calls per engine")
    args = ap.parse_args()

    jpg, _ = next(palm_samples())
    x, lb = letterbox(load_rgb(jpg))
    ref = make("onnx-cpu")
    rb, rs = ref.infer(x)
    ref_det = decode(rb, rs, lb)[0]

    rows = []
    for name in args.engines:
        e = make(name)
        for _ in range(10):                          # warm up caches/clocks
            e.infer(x)
        times = []
        for _ in range(args.n):
            t = time.perf_counter()
            boxes, scores = e.infer(x)
            times.append((time.perf_counter() - t) * 1e3)
        times.sort()
        det = decode(boxes, scores, lb)
        score = f"{det[0]['score']:.4f}" if det else "none"
        rows.append([name, ENGINES[name][0], f"{e.load_s:.2f} s", f"{e.first_s * 1e3:.0f} ms",
                     f"{statistics.median(times):.2f}", f"{times[int(0.9 * len(times))]:.2f}",
                     f"{float(np.abs(boxes - rb).max()):.3f}",
                     f"{float(np.abs(scores - rs).max()):.3f}", score])
        print(" | ".join(rows[-1]))

    text = "\n".join([
        "# Benchmark", "",
        f"Input: {jpg.name}, {args.n} timed calls after 10 warm-up calls. "
        f"Reference: onnx-cpu (fp32), best palm score {ref_det['score']:.4f}.", "",
        md_table(["engine", "what", "load", "first call", "median ms", "p90 ms",
                  "boxes diff (px)", "logit diff", "palm score"], rows), "",
    ])
    print(text)
    (BUILD / "bench.md").write_text(text)


if __name__ == "__main__":
    main()
