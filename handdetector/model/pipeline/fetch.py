"""Stage 1: fetch the model and test data, pinned by commit and SHA-256.

Nothing fetched here is committed to the repo (licences differ: model
Apache 2.0, test images CC BY-SA 4.0) -- it all lands in build/fetch/.
"""
import hashlib
import sys
import urllib.request
import zipfile

from common import FETCH

ZOO_COMMIT = "8de36535ea29e8f9d41e6e3fa5a0df14bab00ec5"  # 2023-02-14, model added
ZOO = f"https://github.com/opencv/opencv_zoo/raw/{ZOO_COMMIT}/models/palm_detection_mediapipe"

ARTIFACTS = [
    # The detector itself (Apache 2.0). Converted by OpenCV from
    # MediaPipe's TFLite with tf2onnx + onnx-simplifier.
    dict(file="palm_detection_mediapipe_2023feb.onnx",
         url=f"{ZOO}/palm_detection_mediapipe_2023feb.onnx",
         sha256="78ff51c38496b7fc8b8ebdb6cc8c1abb02fa6c38427c6848254cdaba57fcce7c"),
    # The zoo's reference wrapper -- only used to cross-check our anchor
    # generation against its hard-coded 192x192 anchor table.
    dict(file="mp_palmdet.py",
         url=f"https://raw.githubusercontent.com/opencv/opencv_zoo/{ZOO_COMMIT}"
             "/models/palm_detection_mediapipe/mp_palmdet.py",
         sha256="3224c6165a1391144012962f0dd12b435758b7037baf3c77677fe639d3c30268"),
    # Zoo benchmark data: 3 palm photos (256x256) with ground truth.
    dict(file="palm_detection_20230125.zip",
         url="https://drive.usercontent.google.com/download"
             "?id=1Z4KvccTZPeZ0qFLZ6saBt_TvcKYyo9JE&export=download&confirm=t",
         sha256="1247239c02e3d3fc40fc74222ceb448dc942ea2c22e4255be677505e4d4fde51",
         unzip=True),
]


def sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for block in iter(lambda: f.read(1 << 16), b""):
            h.update(block)
    return h.hexdigest()


def main():
    FETCH.mkdir(parents=True, exist_ok=True)
    ok = True
    for a in ARTIFACTS:
        dst = FETCH / a["file"]
        if not dst.exists() or sha256(dst) != a["sha256"]:
            print(f"fetch {a['file']}")
            urllib.request.urlretrieve(a["url"], dst)
        got = sha256(dst)
        if got != a["sha256"]:
            print(f"  SHA-256 mismatch: {got}", file=sys.stderr)
            ok = False
            continue
        print(f"  ok {a['file']} ({dst.stat().st_size} bytes)")
        if a.get("unzip"):
            with zipfile.ZipFile(dst) as z:
                z.extractall(FETCH)
    sys.exit(0 if ok else 1)


if __name__ == "__main__":
    main()
