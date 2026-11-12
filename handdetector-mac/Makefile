# The pipeline, one stage per target, each writing into build/.
# `make` runs stages 1-6; `make run` opens the camera app.
#
#   1 fetch    model + test images, pinned by SHA-256    build/fetch/
#   2 inspect  what's inside the ONNX model              build/inspect.md
#   3 surgery  PRelu slope shapes, tensor names          build/palm_surgered.onnx
#   4 convert  ONNX -> PyTorch -> Core ML (+ what runs where)  build/*.mlpackage, build/convert.md
#   5 bench    every engine, same input                  build/bench.md
#   6 detect   full chain on the test palms              build/detect/<engine>/
#     run      live camera                               (window)

PY := uv run --quiet python
P := pipeline
ONNX := build/fetch/palm_detection_mediapipe_2023feb.onnx

.PHONY: all setup fetch inspect surgery convert bench detect run clean

all: detect bench

setup:
	uv sync

$(ONNX): $(P)/fetch.py
	$(PY) $(P)/fetch.py
fetch: $(ONNX)

build/inspect.md: $(ONNX) $(P)/inspect_model.py $(P)/common.py
	$(PY) $(P)/inspect_model.py
inspect: build/inspect.md

build/palm_surgered.onnx: $(ONNX) $(P)/surgery.py build/inspect.md
	$(PY) $(P)/surgery.py
surgery: build/palm_surgered.onnx

build/convert.md: build/palm_surgered.onnx $(P)/convert.py
	$(PY) $(P)/convert.py
convert: build/convert.md

bench: build/convert.md
	$(PY) $(P)/bench.py

detect: build/convert.md
	$(PY) $(P)/detect.py -e coreml-ane
	$(PY) $(P)/detect.py -e onnx-cpu

ENGINE ?= coreml-ane
run: build/convert.md
	$(PY) app.py -e $(ENGINE)

clean:
	rm -rf build
