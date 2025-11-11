"""
Toy PLONK implementation that mirrors the Groth16 example under draft-grorth16.

This module keeps the algebraic backbone of PLONK (selectors, permutation
arguments, vanishing polynomial checks) but omits elliptic-curve commitments so
that the end-to-end flow is easy to read.  Use it as a didactic reference, not
as production code.
"""

from __future__ import annotations

import os
import random
import tempfile
from dataclasses import dataclass, field
from typing import Dict, List, Tuple

import numpy as np

cache_dir = os.path.join(tempfile.gettempdir(), "numba_cache")
os.makedirs(cache_dir, exist_ok=True)
os.environ.setdefault("NUMBA_CACHE_DIR", cache_dir)

import galois  # noqa: E402
from py_ecc.optimized_bn128 import curve_order

p = curve_order
FP = galois.GF(p)

COLUMNS = ("w_L", "w_R", "w_O")


def to_field_vector(values) -> galois.FieldArray:
    return FP([int(v) % p for v in values])


@dataclass
class EvaluationDomain:
    size: int
    points: galois.FieldArray = field(init=False)
    vanishing_polynomial: galois.Poly = field(init=False)

    def __post_init__(self) -> None:
        self.points = FP(np.arange(1, self.size + 1, dtype=np.int64))
        poly = galois.Poly.One(field=FP)
        for point in self.points:
            poly *= galois.Poly([-point, 1], field=FP)  # (X - point)
        self.vanishing_polynomial = poly


@dataclass
class ToyCircuit:
    selectors: Dict[str, galois.FieldArray]
    wire_labels: Dict[str, List[str]]
    coset_shifts: Dict[str, galois.FieldArray]
    public_size: int = 2
    domain: EvaluationDomain = field(init=False)
    sigma: Dict[str, List[Tuple[str, int]]] = field(init=False)

    def __post_init__(self) -> None:
        self.size = len(self.wire_labels["w_L"])
        self.domain = EvaluationDomain(self.size)
        self.sigma = build_sigma_map(self.wire_labels)


@dataclass
class ToyWitness:
    wires: Dict[str, galois.FieldArray]
    public_inputs: galois.FieldArray


@dataclass
class ProvingKey:
    circuit: ToyCircuit


@dataclass
class VerifierKey:
    circuit: ToyCircuit


@dataclass
class Proof:
    wire_polys: Dict[str, galois.Poly]
    permutation_evaluations: galois.FieldArray
    beta: galois.FieldArray
    gamma: galois.FieldArray


def build_sigma_map(
    wire_labels: Dict[str, List[str]]
) -> Dict[str, List[Tuple[str, int]]]:
    sigma: Dict[str, List[Tuple[str, int]]] = {
        col: [("", 0)] * len(rows) for col, rows in wire_labels.items()
    }
    occurrences: Dict[str, List[Tuple[str, int]]] = {}
    for col in COLUMNS:
        for row, label in enumerate(wire_labels[col]):
            occurrences.setdefault(label, []).append((col, row))
    for cycle in occurrences.values():
        if len(cycle) == 1:
            col, row = cycle[0]
            sigma[col][row] = (col, row)
            continue
        for idx, (col, row) in enumerate(cycle):
            sigma[col][row] = cycle[(idx + 1) % len(cycle)]
    return sigma


def keygen(circuit: ToyCircuit) -> Tuple[ProvingKey, VerifierKey]:
    return ProvingKey(circuit), VerifierKey(circuit)


def prove(pk: ProvingKey, witness: ToyWitness) -> Proof:
    circuit = pk.circuit
    assert len(witness.public_inputs) == circuit.public_size

    wire_polys = {
        col: galois.lagrange_poly(circuit.domain.points, witness.wires[col])
        for col in COLUMNS
    }

    gate_eval = compute_gate_evaluations(circuit.selectors, witness.wires)
    assert np.all(gate_eval == 0), "Gate constraints not satisfied"

    beta = FP(random.randint(2, p - 1))
    gamma = FP(random.randint(2, p - 1))
    z_evals = permutation_accumulator(
        witness.wires, circuit, beta=beta, gamma=gamma
    )

    assert z_evals[0] == 1 and z_evals[-1] == 1, "Permutation boundary failed"

    return Proof(
        wire_polys=wire_polys,
        permutation_evaluations=z_evals,
        beta=beta,
        gamma=gamma,
    )


def verify(vk: VerifierKey, public_inputs, proof: Proof) -> bool:
    circuit = vk.circuit
    domain_points = circuit.domain.points
    wires = {col: proof.wire_polys[col](domain_points) for col in COLUMNS}

    # Public inputs occupy witness slots w_O[-1] (the circuit output) and a fixed 1.
    if public_inputs[0] != 1:
        return False
    if public_inputs[1] != wires["w_O"][-1]:
        return False

    gate_eval = compute_gate_evaluations(circuit.selectors, wires)
    if not np.all(gate_eval == 0):
        return False

    recomputed_z = permutation_accumulator(
        wires, circuit, beta=proof.beta, gamma=proof.gamma
    )
    if not np.array_equal(recomputed_z, proof.permutation_evaluations):
        return False

    return recomputed_z[0] == 1 and recomputed_z[-1] == 1


def compute_gate_evaluations(
    selectors: Dict[str, galois.FieldArray],
    wires: Dict[str, galois.FieldArray],
) -> galois.FieldArray:
    size = len(wires["w_L"])
    results = FP.Zeros(size, dtype=object)
    for i in range(size):
        term = selectors["q_M"][i] * wires["w_L"][i] * wires["w_R"][i]
        term += selectors["q_L"][i] * wires["w_L"][i]
        term += selectors["q_R"][i] * wires["w_R"][i]
        term += selectors["q_O"][i] * wires["w_O"][i]
        term += selectors["q_C"][i]
        results[i] = term
    return results


def permutation_accumulator(
    wires: Dict[str, galois.FieldArray],
    circuit: ToyCircuit,
    beta,
    gamma,
) -> galois.FieldArray:
    domain_points = circuit.domain.points
    acc = FP.Zeros(circuit.size + 1, dtype=object)
    acc[0] = FP(1)
    for row in range(circuit.size):
        numerator = FP(1)
        denominator = FP(1)
        for col in COLUMNS:
            id_term = circuit.coset_shifts[col] * domain_points[row]
            numerator *= wires[col][row] + beta * id_term + gamma
            target_col, target_row = circuit.sigma[col][row]
            sigma_term = circuit.coset_shifts[target_col] * domain_points[target_row]
            denominator *= wires[target_col][target_row] + beta * sigma_term + gamma
        acc[row + 1] = acc[row] * numerator / denominator
    return acc


def build_example_circuit() -> ToyCircuit:
    selectors = {
        "q_M": to_field_vector([1, 1, 5, 4, 13, 0, 0, 0, 0, 0]),
        "q_L": to_field_vector([0, 0, 0, 0, 0, 1, 1, 1, 10, 1]),
        "q_R": to_field_vector([0, 0, 0, 0, 0, -1, 1, 1, 0, -1]),
        "q_O": to_field_vector([-1] * 10),
        "q_C": to_field_vector([0] * 10),
    }

    wire_labels = {
        "w_L": [
            "x",
            "y",
            "x",
            "t1",
            "x",
            "t3",
            "t5",
            "t7",
            "y",
            "t8",
        ],
        "w_R": [
            "x",
            "y",
            "t1",
            "t2",
            "t2",
            "t4",
            "t6",
            "t1",
            "zero",
            "t10",
        ],
        "w_O": [
            "t1",
            "t2",
            "t3",
            "t4",
            "t5",
            "t6",
            "t7",
            "t8",
            "t10",
            "out",
        ],
    }

    coset_shifts = {col: FP(shift) for col, shift in zip(COLUMNS, (1, 2, 3))}

    return ToyCircuit(
        selectors=selectors,
        wire_labels=wire_labels,
        coset_shifts=coset_shifts,
    )


def example_witness(x_value: int, y_value: int) -> ToyWitness:
    x = FP(x_value)
    y = FP(y_value)
    t1 = x * x
    t2 = y * y
    t3 = 5 * x * t1
    t4 = 4 * t1 * t2
    t5 = 13 * x * t2
    t6 = t3 - t4
    t7 = t5 + t6
    t8 = t7 + t1
    t10 = 10 * y
    out = t8 - t10
    zero = FP(0)

    wires = {
        "w_L": FP([x, y, x, t1, x, t3, t5, t7, y, t8]),
        "w_R": FP([x, y, t1, t2, t2, t4, t6, t1, zero, t10]),
        "w_O": FP([t1, t2, t3, t4, t5, t6, t7, t8, t10, out]),
    }
    public_inputs = FP([1, out])
    return ToyWitness(wires=wires, public_inputs=public_inputs)


def get_public_inputs(witness: ToyWitness) -> galois.FieldArray:
    return witness.public_inputs
