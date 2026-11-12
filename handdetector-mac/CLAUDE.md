# handdetector-mac

Educational experiment: the `../handdetector` palm detector (OpenCV zoo
ONNX, MediaPipe palm detection) run natively on macOS through ONNX
Runtime, PyTorch and Core ML, on CPU, GPU and Neural Engine, with a live
camera app. Not for the ESP32; the goal is to learn how models run.

- Runs **natively** with uv (Python 3.12, `pyproject.toml`), not in
  Docker: the camera and the Neural Engine aren't reachable from the
  Linux VM. (`../handdetector`'s container rule doesn't apply here.)
- Pipeline = numbered stages in `Makefile`, one script each in
  `pipeline/`, outputs in `build/` (git-ignored, nothing fetched is
  committed). `common.py` = everything outside the network,
  `engines.py` = every way to run it behind `infer(x) -> boxes, scores`.
- New ways to run the model go into `engines.py` and then show up in
  bench, detect and the app. Findings with numbers go into README.md.
- torch pinned to 2.7.0 (coremltools 9.0's tested version).
