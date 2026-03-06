#!/usr/bin/env python3
"""
Phase 2: Train MLP to map semantic dimensions -> Blofeld patch parameters.

Input:  data/scored_patches.json, learn/dimensions.json
Output: models/model.pt
"""

import argparse
import json
import sys
from pathlib import Path

import numpy as np
import torch
import torch.nn as nn
from torch.utils.data import DataLoader, TensorDataset, random_split

sys.path.insert(0, str(Path(__file__).parent))
from curated_params import extract, normalize, N_PARAMS, PARAM_NAMES

ROOT = Path(__file__).parent.parent
SCORED_FILE = ROOT / "data" / "scored_patches.json"
DIMENSIONS_FILE = Path(__file__).parent / "dimensions.json"
MODELS_DIR = ROOT / "models"


# ---------------------------------------------------------------------------
# Model
# ---------------------------------------------------------------------------

class SynthMapper(nn.Module):
    def __init__(self, n_dims: int, n_params: int, dropout: float = 0.3):
        super().__init__()
        self.net = nn.Sequential(
            nn.Linear(n_dims, 128),
            nn.ReLU(),
            nn.Dropout(dropout),
            nn.Linear(128, 256),
            nn.ReLU(),
            nn.Dropout(dropout),
            nn.Linear(256, 256),
            nn.ReLU(),
            nn.Dropout(dropout),
            nn.Linear(256, 128),
            nn.ReLU(),
            nn.Linear(128, n_params),
            nn.Sigmoid(),
        )

    def forward(self, x):
        return self.net(x)


# ---------------------------------------------------------------------------
# Data loading
# ---------------------------------------------------------------------------

def load_dataset(dimensions: list[dict]) -> tuple[torch.Tensor, torch.Tensor]:
    dim_names = [d["name"] for d in dimensions]
    records = json.loads(SCORED_FILE.read_text())

    X, Y = [], []
    skipped = 0
    for r in records:
        vector = r.get("vector")
        if not vector:
            skipped += 1
            continue
        try:
            x = np.array([float(vector[d]) for d in dim_names], dtype=np.float32)
            y = normalize(extract(r["patch"]))
            X.append(x)
            Y.append(y)
        except (KeyError, IndexError) as e:
            skipped += 1

    if skipped:
        print(f"  Skipped {skipped} records (missing vector or params)")

    return torch.tensor(np.array(X)), torch.tensor(np.array(Y))


# ---------------------------------------------------------------------------
# Training
# ---------------------------------------------------------------------------

def train(args):
    dimensions = json.loads(DIMENSIONS_FILE.read_text())
    n_dims = len(dimensions)

    print(f"Loading dataset...")
    X, Y = load_dataset(dimensions)
    print(f"  {len(X)} samples, {n_dims} inputs -> {N_PARAMS} outputs")

    # Train / val split
    dataset = TensorDataset(X, Y)
    n_val = max(1, int(len(dataset) * 0.2))
    n_train = len(dataset) - n_val
    train_set, val_set = random_split(dataset, [n_train, n_val],
                                      generator=torch.Generator().manual_seed(42))

    train_loader = DataLoader(train_set, batch_size=args.batch_size, shuffle=True)
    val_loader = DataLoader(val_set, batch_size=args.batch_size)

    print(f"  Train: {n_train}  Val: {n_val}")

    model = SynthMapper(n_dims, N_PARAMS, dropout=args.dropout)
    optimizer = torch.optim.Adam(model.parameters(), lr=args.lr)
    scheduler = torch.optim.lr_scheduler.ReduceLROnPlateau(optimizer, patience=50, factor=0.5)
    loss_fn = nn.MSELoss()

    best_val_loss = float("inf")
    best_epoch = 0

    print(f"\nTraining for {args.epochs} epochs...")
    for epoch in range(1, args.epochs + 1):
        # Train
        model.train()
        train_loss = 0.0
        for xb, yb in train_loader:
            optimizer.zero_grad()
            pred = model(xb)
            loss = loss_fn(pred, yb)
            loss.backward()
            optimizer.step()
            train_loss += loss.item() * len(xb)
        train_loss /= n_train

        # Validate
        model.eval()
        val_loss = 0.0
        with torch.no_grad():
            for xb, yb in val_loader:
                pred = model(xb)
                val_loss += loss_fn(pred, yb).item() * len(xb)
        val_loss /= n_val

        scheduler.step(val_loss)

        if val_loss < best_val_loss:
            best_val_loss = val_loss
            best_epoch = epoch
            MODELS_DIR.mkdir(exist_ok=True)
            torch.save(model.state_dict(), MODELS_DIR / "model.pt")

        if epoch % 100 == 0 or epoch == 1:
            lr = optimizer.param_groups[0]["lr"]
            print(f"  Epoch {epoch:4d}  train={train_loss:.4f}  val={val_loss:.4f}  lr={lr:.2e}  best={best_val_loss:.4f}@{best_epoch}")

    print(f"\nBest val loss: {best_val_loss:.4f} at epoch {best_epoch}")
    print(f"Model saved -> models/model.pt")

    # Save metadata alongside the model
    meta = {
        "n_dims": n_dims,
        "n_params": N_PARAMS,
        "param_names": PARAM_NAMES,
        "dim_names": [d["name"] for d in dimensions],
        "best_val_loss": best_val_loss,
        "best_epoch": best_epoch,
        "args": vars(args),
    }
    (MODELS_DIR / "model_meta.json").write_text(json.dumps(meta, indent=2))


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--epochs",     type=int,   default=1000)
    parser.add_argument("--lr",         type=float, default=1e-3)
    parser.add_argument("--batch-size", type=int,   default=32)
    parser.add_argument("--dropout",    type=float, default=0.3)
    args = parser.parse_args()
    train(args)


if __name__ == "__main__":
    main()
