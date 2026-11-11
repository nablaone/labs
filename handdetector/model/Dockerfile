# Model tooling: everything TensorFlow / ONNX lives in here, nothing on the
# host. Same pattern as the firmware build (../Dockerfile): the model/
# directory is bind-mounted at /work, so outputs land in model/build/.
#
# onnx2tf pins its whole stack (TensorFlow, onnx, onnxruntime, onnxsim,
# LiteRT interpreter) -- installing its "tensorflow" extra at a fixed
# version keeps the conversion reproducible.
FROM python:3.12-slim

# build-essential + cmake: on linux/arm64 (Apple Silicon hosts) a few of
# onnx2tf's pinned dependencies (onnxoptimizer, psutil) have no wheels and
# are compiled from source.
RUN apt-get update && apt-get install -y --no-install-recommends \
        git make curl unzip ca-certificates build-essential cmake \
    && rm -rf /var/lib/apt/lists/*

ARG ONNX2TF_VERSION=2.6.9
RUN pip install --no-cache-dir "onnx2tf[tensorflow]==${ONNX2TF_VERSION}"

# opencv-python (pinned by onnx2tf, not the -headless build) links against
# X11/GL even when only used for image I/O.
RUN apt-get update && apt-get install -y --no-install-recommends \
        libxcb1 libgl1 libglib2.0-0 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /work
