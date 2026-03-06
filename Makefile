PYTHON := uv run --env-file .env python

.PHONY: all clean

all: data/patches.json

# Phase 1a: parse .syx banks -> data/patches.json
# Note: .syx files have spaces in their paths which make can't track as deps.
# To force a re-parse: rm data/patches.json
data/patches.json: learn/parse_banks.py
	@mkdir -p data
	$(PYTHON) learn/parse_banks.py

# Phase 1b: score patches with Claude -> data/scored_patches.json
data/scored_patches.json: data/patches.json learn/score_patches.py learn/dimensions.json
	$(PYTHON) learn/score_patches.py

# Test scoring on 5 patches only
.PHONY: score-test
score-test: data/patches.json
	$(PYTHON) learn/score_patches.py --limit 5

# Phase 2: train MLP -> models/model.pt + models/model_meta.json
models/model.pt: data/scored_patches.json learn/train.py learn/curated_params.py learn/dimensions.json
	@mkdir -p models
	$(PYTHON) learn/train.py

# Phase 3: export to ONNX -> models/model.onnx
models/model.onnx: models/model.pt learn/export_onnx.py
	$(PYTHON) learn/export_onnx.py

# Phase 4: generate web assets (blofeld_map.json, copy model + dimensions)
web/assets/blofeld_map.json: models/model.onnx learn/gen_web_assets.py learn/curated_params.py learn/dimensions.json
	@mkdir -p web/assets web/models
	$(PYTHON) learn/gen_web_assets.py

# Serve the UI (run inside devcontainer, or locally)
.PHONY: serve
serve: web/assets/blofeld_map.json
	$(PYTHON) web/server.py

clean:
	rm -rf data/ models/ web/assets/ web/models/
