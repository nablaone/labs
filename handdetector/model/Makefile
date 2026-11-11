# ONNX -> TFLite Micro pipeline for the palm detector. Every Python/TF step
# runs inside the handdetector-model container (see Dockerfile); only
# `capture` (USB serial) and `flash-model` (esptool) run on the host.
#
#   make                 fetch -> inspect -> surgery -> calib -> convert -> validate
#   make RES=128         same at a smaller input size (outputs in build/r128/)
#   make tflm            generate the TFLM source tree for the firmware
#   make capture N=20    pull N camera frames from the board into data/camera/
#   make flash-model     write build/rRES/model_int8.tflite to the model partition
#   make flash-golden    write build/rRES/golden.bin (host test vectors) after it
#
# The project root is mounted at /proj so the tflm target can write into
# ../components/.

IMAGE ?= handdetector-model
# 96: what the board runs (34 s/inference with reference kernels); 192 is the
# zoo original (137 s). See ../docs/model-pipeline.md.
RES ?= 96
ROOT := $(abspath $(CURDIR)/..)
RUN = docker run --rm -v $(ROOT):/proj -w /proj/model -e RES=$(RES) $(IMAGE)
PY = $(RUN) python pipeline

OUT := build/r$(RES)

.PHONY: all image fetch inspect surgery calib convert validate tflm capture flash-model flash-golden clean

all: fetch inspect surgery calib convert validate

image:
	docker build -t $(IMAGE) .

fetch: image
	$(PY)/fetch.py

inspect: image
	$(PY)/inspect_model.py build/fetch/palm_detection_mediapipe_2023feb.onnx build/fetch/original.md

surgery: image
	$(PY)/surgery.py
	$(PY)/inspect_model.py $(OUT)/palm_det_surgered.onnx $(OUT)/surgered.md

calib: image
	$(PY)/calib.py

convert: image
	$(PY)/convert.py

validate: image
	$(PY)/validate.py

# ---- TFLite Micro sources -------------------------------------------------
# Upstream tflite-micro at a pinned commit, reference kernels only (no
# OPTIMIZED_KERNEL_DIR): plain portable C++, no vendor libraries. The
# generated tree is gitignored; the component's CMakeLists.txt is not.
TFLM_COMMIT ?= a361a52575a7e2e2104c296ce367b88a7e7232e1
TFLM_TREE := ../components/tflite-micro/tflm

tflm: image
	$(RUN) sh -c 'set -e; \
	  rm -rf build/tflite-micro && mkdir -p build/tflite-micro && cd build/tflite-micro && \
	  git init -q && git remote add origin https://github.com/tensorflow/tflite-micro.git && \
	  git fetch -q --depth 1 origin $(TFLM_COMMIT) && git checkout -q FETCH_HEAD && \
	  rm -rf /proj/components/tflite-micro/tflm && \
	  python tensorflow/lite/micro/tools/project_generation/create_tflm_tree.py /proj/components/tflite-micro/tflm && \
	  echo $(TFLM_COMMIT) > /proj/components/tflite-micro/tflm/COMMIT'
	@echo "TFLM tree: $(TFLM_TREE) ($$(find $(TFLM_TREE) -type f | wc -l) files)"

# ---- host-side helpers ------------------------------------------------------
# Same esptool as the firmware Makefile; its Python already has pyserial.
ESPTOOL := $(shell command -v esptool.py 2>/dev/null || command -v esptool 2>/dev/null || echo esptool.py)
SERIAL_PY ?= $(shell head -1 $(ESPTOOL) 2>/dev/null | sed -n 's/^\#!//p')
PORT ?= $(shell ls /dev/cu.usbmodem* /dev/ttyACM* 2>/dev/null | head -1)
N ?= 20

capture:
	$(if $(PORT),,$(error No PORT found -- is the board plugged in?))
	$(SERIAL_PY) pipeline/capture.py $(PORT) data/camera $(N)

# Model partition offset comes from ../partitions.csv.
MODEL_OFFSET ?= $(shell awk -F, '$$1 ~ /^model/ {gsub(/ /,"",$$4); print $$4}' ../partitions.csv)

flash-model:
	$(if $(PORT),,$(error No PORT found -- is the board plugged in?))
	$(ESPTOOL) --chip esp32s3 -p $(PORT) write_flash $(MODEL_OFFSET) $(OUT)/model_int8.tflite

# Golden vectors (validate.py) go in the last 1 MB of the model partition,
# read by `det golden` on the device.
GOLDEN_OFFSET ?= $(shell printf '0x%x' $$(( $(MODEL_OFFSET) + 0x300000 )))

flash-golden:
	$(if $(PORT),,$(error No PORT found -- is the board plugged in?))
	$(ESPTOOL) --chip esp32s3 -p $(PORT) write_flash $(GOLDEN_OFFSET) $(OUT)/golden.bin

clean:
	rm -rf build
