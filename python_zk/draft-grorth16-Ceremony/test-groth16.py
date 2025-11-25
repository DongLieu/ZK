
# Import module
import random
import groth16
import numpy as np
import galois

FP = groth16.FP
p = groth16.p

NUM_CONTRIBUTORS = 3
TRANSCRIPT_NUM_PUBLIC = 2


def fp_to_int(value):
    return int(value) % p


def _normalize_g1(point):
    coords = groth16.normalize(point)
    if len(coords) == 3:
        x, y, _ = coords
    elif len(coords) == 2:
        x, y = coords
    else:
        raise ValueError("Unexpected G1 coordinate format.")
    return x, y


def _normalize_g2(point):
    coords = groth16.normalize(point)
    if len(coords) == 3:
        x, y, _ = coords
    elif len(coords) == 2:
        x, y = coords
    else:
        raise ValueError("Unexpected G2 coordinate format.")
    return x, y


def _fq_int(value):
    return int(getattr(value, "n", value))


def serialize_g1(point):
    x, y = _normalize_g1(point)
    return [_fq_int(x), _fq_int(y)]


def serialize_g2(point):
    x, y = _normalize_g2(point)
    return [
        [_fq_int(x.coeffs[0]), _fq_int(x.coeffs[1])],
        [_fq_int(y.coeffs[0]), _fq_int(y.coeffs[1])],
    ]


def random_scalar():
    return FP(random.randint(2, p - 1))


def initialize_state(tau_length):
    return {
        "tau_g1": [groth16.G1 for _ in range(tau_length)],
        "tau_g2": [groth16.G2 for _ in range(tau_length)],
        "alpha_g1": groth16.G1,
        "beta_g1": groth16.G1,
        "beta_g2": groth16.G2,
        "gamma_g2": groth16.G2,
        "delta_g1": groth16.G1,
        "delta_g2": groth16.G2,
        "beta_tau_over_gamma_g1": [groth16.G1 for _ in range(tau_length)],
        "alpha_tau_over_gamma_g1": [groth16.G1 for _ in range(tau_length)],
        "tau_over_gamma_g1": [groth16.G1 for _ in range(tau_length)],
        "beta_tau_over_delta_g1": [groth16.G1 for _ in range(tau_length)],
        "alpha_tau_over_delta_g1": [groth16.G1 for _ in range(tau_length)],
        "tau_over_delta_g1": [groth16.G1 for _ in range(tau_length)],
    }


def apply_contribution(state, contribution):
    tau = contribution["tau"]
    alpha = contribution["alpha"]
    beta = contribution["beta"]
    gamma = contribution["gamma"]
    delta = contribution["delta"]

    gamma_inv = FP(1) / gamma
    delta_inv = FP(1) / delta

    state["alpha_g1"] = groth16.multiply(state["alpha_g1"], fp_to_int(alpha))
    state["beta_g1"] = groth16.multiply(state["beta_g1"], fp_to_int(beta))
    state["beta_g2"] = groth16.multiply(state["beta_g2"], fp_to_int(beta))
    state["gamma_g2"] = groth16.multiply(state["gamma_g2"], fp_to_int(gamma))
    state["delta_g1"] = groth16.multiply(state["delta_g1"], fp_to_int(delta))
    state["delta_g2"] = groth16.multiply(state["delta_g2"], fp_to_int(delta))

    tau_pow = FP(1)
    for i in range(len(state["tau_g1"])):
        scalar_tau = fp_to_int(tau_pow)
        state["tau_g1"][i] = groth16.multiply(state["tau_g1"][i], scalar_tau)
        state["tau_g2"][i] = groth16.multiply(state["tau_g2"][i], scalar_tau)

        gamma_factor = tau_pow * gamma_inv
        beta_gamma_factor = gamma_factor * beta
        alpha_gamma_factor = gamma_factor * alpha

        delta_factor = tau_pow * delta_inv
        beta_delta_factor = delta_factor * beta
        alpha_delta_factor = delta_factor * alpha

        state["tau_over_gamma_g1"][i] = groth16.multiply(
            state["tau_over_gamma_g1"][i], fp_to_int(gamma_factor)
        )
        state["beta_tau_over_gamma_g1"][i] = groth16.multiply(
            state["beta_tau_over_gamma_g1"][i], fp_to_int(beta_gamma_factor)
        )
        state["alpha_tau_over_gamma_g1"][i] = groth16.multiply(
            state["alpha_tau_over_gamma_g1"][i], fp_to_int(alpha_gamma_factor)
        )

        state["tau_over_delta_g1"][i] = groth16.multiply(
            state["tau_over_delta_g1"][i], fp_to_int(delta_factor)
        )
        state["beta_tau_over_delta_g1"][i] = groth16.multiply(
            state["beta_tau_over_delta_g1"][i], fp_to_int(beta_delta_factor)
        )
        state["alpha_tau_over_delta_g1"][i] = groth16.multiply(
            state["alpha_tau_over_delta_g1"][i], fp_to_int(alpha_delta_factor)
        )

        tau_pow *= tau


def compute_target_points(state, qap):
    coeffs = qap.T.coefficients()[::-1]
    required_len = qap.T.degree - 1 + len(coeffs)
    assert len(state["tau_over_delta_g1"]) >= required_len

    target_points = []
    for i in range(qap.T.degree - 1):
        acc = None
        for j, coeff in enumerate(coeffs):
            idx = i + j
            point = state["tau_over_delta_g1"][idx]
            scalar = fp_to_int(coeff)
            if scalar == 0:
                continue
            term = groth16.multiply(point, scalar)
            acc = term if acc is None else groth16.add(acc, term)
        target_points.append(acc if acc is not None else groth16.G1)
    return target_points


def serialize_transcript(state, target_points, qap, num_public):
    basis_len = qap.T.degree
    return {
        "num_public": num_public,
        "tau_g1": [serialize_g1(pt) for pt in state["tau_g1"][:basis_len]],
        "tau_g2": [serialize_g2(pt) for pt in state["tau_g2"][:basis_len]],
        "alpha_g1": serialize_g1(state["alpha_g1"]),
        "beta_g1": serialize_g1(state["beta_g1"]),
        "beta_g2": serialize_g2(state["beta_g2"]),
        "gamma_g2": serialize_g2(state["gamma_g2"]),
        "delta_g1": serialize_g1(state["delta_g1"]),
        "delta_g2": serialize_g2(state["delta_g2"]),
        "target_g1": [serialize_g1(pt) for pt in target_points],
        "beta_tau_over_gamma_g1": [
            serialize_g1(pt) for pt in state["beta_tau_over_gamma_g1"][:basis_len]
        ],
        "alpha_tau_over_gamma_g1": [
            serialize_g1(pt) for pt in state["alpha_tau_over_gamma_g1"][:basis_len]
        ],
        "tau_over_gamma_g1": [
            serialize_g1(pt) for pt in state["tau_over_gamma_g1"][:basis_len]
        ],
        "beta_tau_over_delta_g1": [
            serialize_g1(pt) for pt in state["beta_tau_over_delta_g1"][:basis_len]
        ],
        "alpha_tau_over_delta_g1": [
            serialize_g1(pt) for pt in state["alpha_tau_over_delta_g1"][:basis_len]
        ],
        "tau_over_delta_g1": [
            serialize_g1(pt) for pt in state["tau_over_delta_g1"][:basis_len]
        ],
    }

# f(x,y) = 5*x^3 - 4*x^2*y^2 +13*x*y^2+x^2-10y 
# cacu _witness 1
_x = FP(2)
_y = FP(3)
_v1 = _x * _x
_v2 = _y * _y
_v3 = 5 * _x * _v1
_v4 = 4 * _v1 * _v2
out = 5*_x**3 - 4*_x**2*_y**2 + 13*_x*_y**2 + _x**2 - 10*_y
_witness1 = FP([1, out, _x, _y, _v1, _v2, _v3, _v4])
# witness chứa cả public (1, out) và private (_x, _y, _v1, _v2, _v3, _v4)

# cacu _witness 2
_x2 = FP(4)
_y2 = FP(5)
_v1_2 = _x2 * _x2
_v2_2 = _y2 * _y2
_v3_2 = 5 * _x2 * _v1_2
_v4_2 = 4 * _v1_2 * _v2_2
out2 = 5*_x2**3 - 4*_x2**2*_y2**2 + 13*_x2*_y2**2 + _x2**2 - 10*_y2
_witness2 = FP([1, out2, _x2, _y2, _v1_2, _v2_2, _v3_2, _v4_2])
# ============================================== R1CS =============================================

R = FP([[0, 0, 1, 0, 0, 0, 0, 0],
         [0, 0, 0, 1, 0, 0, 0, 0],
         [0, 0, 5, 0, 0, 0, 0, 0],
         [0, 0, 0, 0, 4, 0, 0, 0],
         [0, 0, 13, 0, 0, 0, 0, 0]])

L = FP([[0, 0, 1, 0, 0, 0, 0, 0],
         [0, 0, 0, 1, 0, 0, 0, 0],
         [0, 0, 0, 0, 1, 0, 0, 0],
         [0, 0, 0, 0, 0, 1, 0, 0],
         [0, 0, 0, 0, 0, 1, 0, 0]])

O = FP([[0, 0, 0, 0, 1, 0, 0, 0],
         [0, 0, 0, 0, 0, 1, 0, 0],
         [0, 0, 0, 0, 0, 0, 1, 0],
         [0, 0, 0, 0, 0, 0, 0, 1],
         [0, 1, 0, 10, FP(p - 1), 0, FP(p - 1), 1]])
assert all(np.equal(np.matmul(L, _witness1) * np.matmul(R, _witness1), np.matmul(O, _witness1))), "not equal"
# # ============================================== QAP =============================================
mtxs = [L, R, O]
poly_m = []

for m in mtxs:
    poly_list = []
    for i in range(0, m.shape[1]):
        points_x = FP(np.zeros(m.shape[0], dtype=int))
        points_y = FP(np.zeros(m.shape[0], dtype=int))
        for j in range(0, m.shape[0]):
            points_x[j] = FP(j+1)
            points_y[j] = m[j][i]

        poly = galois.lagrange_poly(points_x, points_y)
        coef = poly.coefficients()[::-1]
        if len(coef) < m.shape[0]:
            coef = np.append(coef, np.zeros(m.shape[0] - len(coef), dtype=int))
        poly_list.append(coef)
    
    poly_m.append(FP(poly_list))

Lp = poly_m[0]
Rp = poly_m[1]
Op = poly_m[2]

T = galois.Poly([1, p-1], field=FP)
for i in range(2, L.shape[0] + 1):
    T *= galois.Poly([1, p-i], field=FP)

# # ============================================== groth16 =============================================
qap = groth16.QAP(Lp, Rp, Op, T)
tau_basis_length = qap.T.degree * 2
state = initialize_state(tau_basis_length)

def sample_contribution():
    return {
        "tau": random_scalar(),
        "alpha": random_scalar(),
        "beta": random_scalar(),
        "gamma": random_scalar(),
        "delta": random_scalar(),
    }


for _ in range(NUM_CONTRIBUTORS):
    apply_contribution(state, sample_contribution())

target_points = compute_target_points(state, qap)
transcript = serialize_transcript(state, target_points, qap, TRANSCRIPT_NUM_PUBLIC)
_pk,vk = groth16.setup(qap, transcript=transcript)

# # ============================================== proof 1 =============================================
proof1 = groth16.prove(_pk,_witness1, qap)
w_public = groth16.get_witness_public(_pk, _witness1)
# w_public là phần public, phần còn lại của _witness1 sẽ giữ private để prove
ok = groth16.verifier(vk,w_public, proof1)
print(ok)

# # ============================================== proof 2 =============================================
proof2 = groth16.prove(_pk,_witness2, qap)
w_public2 = groth16.get_witness_public(_pk, _witness2)
ok = groth16.verifier(vk,w_public2, proof2)
print(ok)

# # ============================================== proof aggregation =============================================
proof3 = proof2
proof3.A = groth16.add(proof3.A, proof1.A)
proof3.B = groth16.add(proof3.B, proof1.B)
proof3.C = groth16.add(proof3.C, proof1.C)
w_public3 = w_public2
ok = groth16.verifier(vk,w_public3, proof3)
print(ok)
