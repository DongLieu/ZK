import json
import random
from pathlib import Path
from galois import Poly, GF
from py_ecc.optimized_bn128 import optimized_curve as curve
import numpy as np
from py_ecc.optimized_bn128 import (
    multiply,
    G1,
    G2,
    add,
    normalize,
    curve_order,
    pairing,
    neg,
)

# p = 21888242871839275222246405745257275088548364400416034343698204186575808495617
p = curve_order
FP = GF(p)


def _point_at_infinity():
    return curve.Z1


def _to_int(value):
    if isinstance(value, str):
        value = value.strip()
        base = 16 if value.startswith("0x") else 10
        return int(value, base)
    return int(value)


def _deserialize_g1(coords):
    if len(coords) != 2:
        raise ValueError("G1 point must have two coordinates.")
    x, y = (_to_int(c) % p for c in coords)
    return (curve.FQ(x), curve.FQ(y), curve.FQ.one())


def _deserialize_g2(coords):
    if len(coords) != 2 or any(len(c) != 2 for c in coords):
        raise ValueError("G2 point must have two FQ2 coordinates.")
    x_coeffs = [_to_int(c) % p for c in coords[0]]
    y_coeffs = [_to_int(c) % p for c in coords[1]]
    return (
        curve.FQ2(x_coeffs),
        curve.FQ2(y_coeffs),
        curve.FQ2([1, 0]),
    )


def load_ceremony_transcript(path):
    path = Path(path)
    with path.open("r", encoding="utf-8") as infile:
        return json.load(infile)


def _require_key(transcript, key):
    if key not in transcript:
        raise ValueError(f"Transcript missing required key '{key}'.")
    return transcript[key]


def _deserialize_point_list(raw_points, parser):
    if not isinstance(raw_points, list):
        raise ValueError("Transcript section must be a list of points.")
    return [parser(point) for point in raw_points]


def _evaluate_poly_on_points(poly: Poly, point_basis):
    coeffs = poly.coefficients()[::-1]
    if len(coeffs) > len(point_basis):
        raise ValueError("Polynomial degree exceeds basis size.")

    acc = None
    for coeff, basis_point in zip(coeffs, point_basis):
        coeff_int = int(coeff) % p
        if coeff_int == 0:
            continue
        term = multiply(basis_point, coeff_int)
        acc = term if acc is None else add(acc, term)
    return acc if acc is not None else _point_at_infinity()


def _evaluate_poly_list_on_points(polys, point_basis):
    if point_basis is None:
        return None
    return [_evaluate_poly_on_points(poly, point_basis) for poly in polys]


def _sum_point_lists(point_lists):
    non_empty = [lst for lst in point_lists if lst is not None]
    if not non_empty:
        return None
    length = len(non_empty[0])
    for lst in non_empty:
        if len(lst) != length:
            raise ValueError("Mismatched list lengths when summing points.")

    result = []
    for i in range(length):
        acc = _point_at_infinity()
        for lst in non_empty:
            acc = add(acc, lst[i])
        result.append(acc)
    return result


def _maybe_point_list(transcript, key, parser):
    if key not in transcript:
        return None
    return _deserialize_point_list(transcript[key], parser)


def _build_query_from_basis(L_polys, R_polys, O_polys, basis, fallback_points):
    if fallback_points is not None:
        return fallback_points

    if len(L_polys) == 0 and len(R_polys) == 0 and len(O_polys) == 0:
        return []

    beta_basis = basis.get("beta")
    alpha_basis = basis.get("alpha")
    const_basis = basis.get("const")

    contributions = []
    if beta_basis is not None:
        contributions.append(_evaluate_poly_list_on_points(L_polys, beta_basis))
    if alpha_basis is not None:
        contributions.append(_evaluate_poly_list_on_points(R_polys, alpha_basis))
    if const_basis is not None:
        contributions.append(_evaluate_poly_list_on_points(O_polys, const_basis))

    queries = _sum_point_lists(contributions)
    if queries is None:
        raise ValueError(
            "Transcript missing basis to reconstruct query and no fallback provided."
        )
    return queries


class QAP:
    def __init__(self, L: Poly, R: Poly, O: Poly, T: Poly):
        self.L = L
        self.R = R
        self.O = O
        self.T = T
class ProverKey:
    def __init__(
        self,
        tau_G1,
        tau_G2,
        alpha_G1,
        beta_G1,
        beta_G2,
        delta_G1,
        delta_G2,
        K_delta_G1,
        target_G1,
    ):
        self.tau_G1 = tau_G1
        self.tau_G2 = tau_G2
        self.alpha_G1 = alpha_G1
        self.beta_G1 = beta_G1
        self.beta_G2 = beta_G2
        self.delta_G1 = delta_G1
        self.delta_G2 = delta_G2
        self.K_delta_G1 = K_delta_G1
        self.target_G1 = target_G1

class VerifierKey:
    def __init__(self, alpha_G1, beta_G2, gamma_G2, delta_G2, K_gamma_G1):
        self.alpha_G1 = alpha_G1
        self.beta_G2 = beta_G2
        self.gamma_G2 = gamma_G2
        self.delta_G2 = delta_G2
        self.K_gamma_G1 = K_gamma_G1

class Proof:
    def __init__(self, A, B, C):
        self.A = A
        self.B = B
        self.C = C


def setup(qap: QAP, transcript=None, transcript_path=None, verbose=False):
    if transcript_path is not None:
        transcript = load_ceremony_transcript(transcript_path)

    if transcript is not None:
        return _setup_from_transcript(qap, transcript, verbose=verbose)

    # generating toxic waste
    alpha = FP(random.randint(2, p - 1))
    beta = FP(random.randint(2, p - 1))
    gamma = FP(random.randint(2, p - 1))
    delta = FP(random.randint(2, p - 1))
    tau = FP(random.randint(2, p - 1))
    l=random.randrange(2, qap.L.shape[0])

    if verbose:
        print("alpha=", alpha)
        print("beta=", beta)
        print("delta=", delta)
        print("tau=", tau)

    beta_L = beta * qap.L
    alpha_R = alpha * qap.R
    K = beta_L + alpha_R + qap.O
    Kp = to_poly(K)
    K_eval = evaluate_poly_list(Kp, tau)

    T_tau = qap.T(tau)

    pow_tauTtau_div_delta = [
        (tau ** i * T_tau) / delta for i in range(0, qap.T.degree - 1)
    ]
    target_G1 = [multiply(G1, int(pTd)) for pTd in pow_tauTtau_div_delta]

    K_gamma, K_delta = [k / gamma for k in K_eval[:l]], [k / delta for k in K_eval[l:]]

    # generating SRS
    tau_G1 = [multiply(G1, int(tau ** i)) for i in range(0, qap.T.degree)]
    tau_G2 = [multiply(G2, int(tau ** i)) for i in range(0, qap.T.degree)]
    alpha_G1 = multiply(G1, int(alpha))
    beta_G1 = multiply(G1, int(beta))
    beta_G2 = multiply(G2, int(beta))
    gamma_G2 = multiply(G2, int(gamma))
    delta_G1 = multiply(G1, int(delta))
    delta_G2 = multiply(G2, int(delta))
    K_gamma_G1 = [multiply(G1, int(k)) for k in K_gamma]
    K_delta_G1 = [multiply(G1, int(k)) for k in K_delta]

    pk = ProverKey(
        tau_G1,
        tau_G2,
        alpha_G1,
        beta_G1,
        beta_G2,
        delta_G1,
        delta_G2,
        K_delta_G1,
        target_G1,
    )

    vk = VerifierKey(alpha_G1, beta_G2, gamma_G2, delta_G2, K_gamma_G1)

    return pk, vk #,alpha,beta, delta, tau


def _setup_from_transcript(qap: QAP, transcript, verbose=False):
    public_count = transcript.get("num_public")
    expected_tau = qap.T.degree
    tau_G1_raw = _require_key(transcript, "tau_g1")
    tau_G2_raw = _require_key(transcript, "tau_g2")
    tau_G1 = _deserialize_point_list(tau_G1_raw, _deserialize_g1)
    tau_G2 = _deserialize_point_list(tau_G2_raw, _deserialize_g2)
    if len(tau_G1) < expected_tau or len(tau_G2) < expected_tau:
        raise ValueError("Transcript does not provide enough tau powers.")
    tau_G1 = tau_G1[:expected_tau]
    tau_G2 = tau_G2[:expected_tau]

    alpha_G1 = _deserialize_g1(_require_key(transcript, "alpha_g1"))
    beta_G1 = _deserialize_g1(_require_key(transcript, "beta_g1"))
    beta_G2 = _deserialize_g2(_require_key(transcript, "beta_g2"))
    gamma_G2 = _deserialize_g2(_require_key(transcript, "gamma_g2"))
    delta_G1 = _deserialize_g1(_require_key(transcript, "delta_g1"))
    delta_G2 = _deserialize_g2(_require_key(transcript, "delta_g2"))

    raw_K_gamma = transcript.get("K_gamma_g1")
    raw_K_delta = transcript.get("K_delta_g1")
    if public_count is None:
        if raw_K_gamma is None:
            raise ValueError(
                "Transcript must set 'num_public' or provide 'K_gamma_g1' directly."
            )
        public_count = len(raw_K_gamma)
    public_count = int(public_count)
    total_polys = qap.L.shape[0]
    if public_count < 0 or public_count > total_polys:
        raise ValueError("Invalid num_public value inside transcript.")

    K_gamma_fallback = (
        _deserialize_point_list(raw_K_gamma, _deserialize_g1)
        if raw_K_gamma is not None
        else None
    )
    if K_gamma_fallback is not None and len(K_gamma_fallback) != public_count:
        raise ValueError("K_gamma_g1 length does not match num_public.")

    K_delta_fallback = (
        _deserialize_point_list(raw_K_delta, _deserialize_g1)
        if raw_K_delta is not None
        else None
    )
    if (
        K_delta_fallback is not None
        and len(K_delta_fallback) != total_polys - public_count
    ):
        raise ValueError("K_delta_g1 length inconsistent with circuit size.")

    target_G1_raw = _require_key(transcript, "target_g1")
    target_G1 = _deserialize_point_list(target_G1_raw, _deserialize_g1)
    expected_target = qap.T.degree - 1
    if len(target_G1) < expected_target:
        raise ValueError("Transcript target vector too short for circuit.")
    target_G1 = target_G1[:expected_target]

    L_polys = to_poly(qap.L)
    R_polys = to_poly(qap.R)
    O_polys = to_poly(qap.O)
    L_gamma = L_polys[:public_count]
    R_gamma = R_polys[:public_count]
    O_gamma = O_polys[:public_count]
    L_delta = L_polys[public_count:]
    R_delta = R_polys[public_count:]
    O_delta = O_polys[public_count:]

    gamma_basis = {
        "beta": _maybe_point_list(
            transcript, "beta_tau_over_gamma_g1", _deserialize_g1
        ),
        "alpha": _maybe_point_list(
            transcript, "alpha_tau_over_gamma_g1", _deserialize_g1
        ),
        "const": _maybe_point_list(transcript, "tau_over_gamma_g1", _deserialize_g1),
    }
    delta_basis = {
        "beta": _maybe_point_list(
            transcript, "beta_tau_over_delta_g1", _deserialize_g1
        ),
        "alpha": _maybe_point_list(
            transcript, "alpha_tau_over_delta_g1", _deserialize_g1
        ),
        "const": _maybe_point_list(transcript, "tau_over_delta_g1", _deserialize_g1),
    }

    K_gamma_G1 = _build_query_from_basis(
        L_gamma, R_gamma, O_gamma, gamma_basis, K_gamma_fallback
    )
    K_delta_G1 = _build_query_from_basis(
        L_delta, R_delta, O_delta, delta_basis, K_delta_fallback
    )

    if verbose:
        print("Loaded SRS from transcript with:")
        print(f"- {len(tau_G1)} G1 tau powers")
        print(f"- {len(K_gamma_G1)} K_gamma entries")
        print(f"- {len(K_delta_G1)} K_delta entries")

    pk = ProverKey(
        tau_G1,
        tau_G2,
        alpha_G1,
        beta_G1,
        beta_G2,
        delta_G1,
        delta_G2,
        K_delta_G1,
        target_G1,
    )
    vk = VerifierKey(alpha_G1, beta_G2, gamma_G2, delta_G2, K_gamma_G1)

    return pk, vk


def prove(pk: ProverKey, w: [], qap: QAP, verbose=False):
    r = FP(random.randint(2, p - 1))
    s = FP(random.randint(2, p - 1))

    w_priv = w[len(w)-len(pk.K_delta_G1):]

    U = Poly((w @ qap.L)[::-1])
    V = Poly((w @ qap.R)[::-1])
    W = Poly((w @ qap.O)[::-1])

    H = (U * V - W) // qap.T
    rem = (U * V - W) % qap.T

    assert rem == 0

    # [K/δ*w]G1
    Kw_delta_G1_terms = [
        multiply(point, int(scaler)) for point, scaler in zip(pk.K_delta_G1, w_priv)
    ]
    Kw_delta_G1 = Kw_delta_G1_terms[0]
    for i in range(1, len(Kw_delta_G1_terms)):
        Kw_delta_G1 = add(Kw_delta_G1, Kw_delta_G1_terms[i])

    r_delta_G1 = multiply(pk.delta_G1, int(r))
    s_delta_G1 = multiply(pk.delta_G1, int(s))
    s_delta_G2 = multiply(pk.delta_G2, int(s))

    A_G1 = evaluate_poly(U, pk.tau_G1)
    A_G1 = add(A_G1, pk.alpha_G1)
    A_G1 = add(A_G1, r_delta_G1)
    if verbose:
        print("U=", U.coefficients()[::-1])
        # print(pk.tau_G1)
        print("r=",r)
        print("evaluate_poly(U, pk.tau_G1)=",evaluate_poly(U, pk.tau_G1))
        print("pk.alpha_G1=",pk.alpha_G1)
        print("pk.r_delta_G1=",r_delta_G1)
        print("======")


    B_G2 = evaluate_poly(V, pk.tau_G2)
    B_G2 = add(B_G2, pk.beta_G2)
    B_G2 = add(B_G2, s_delta_G2)

    B_G1 = evaluate_poly(V, pk.tau_G1)
    B_G1 = add(B_G1, pk.beta_G1)
    B_G1 = add(B_G1, s_delta_G1)

    As_G1 = multiply(A_G1, int(s))
    Br_G1 = multiply(B_G1, int(r))
    rs_delta_G1 = multiply(pk.delta_G1, int(-r * s))

    HT_G1 = evaluate_poly(H, pk.target_G1)

    C_G1 = add(Kw_delta_G1, HT_G1)
    C_G1 = add(C_G1, As_G1)
    C_G1 = add(C_G1, Br_G1)
    C_G1 = add(C_G1, rs_delta_G1)

    return Proof(A_G1, B_G2, C_G1) #,U.coefficients()[::-1], r

def verifier(vk: VerifierKey, w_pub: [], proof: Proof, verbose=False):
    e1 = pairing(proof.B,proof.A)
    e2 = pairing(vk.beta_G2, vk.alpha_G1)

    # [K/δ*w]G1
    Kw_gamma_G1_terms = [
        multiply(point, int(scaler)) for point, scaler in zip(vk.K_gamma_G1, w_pub)
    ]
    Kw_gamma_G1 = Kw_gamma_G1_terms[0]
    for i in range(1, len(Kw_gamma_G1_terms)):
        Kw_gamma_G1 = add(Kw_gamma_G1, Kw_gamma_G1_terms[i])

    e3 = pairing(vk.gamma_G2,Kw_gamma_G1)

    e4 = pairing(vk.delta_G2,proof.C)
    if verbose:
        print("self.B on curve:", curve.is_on_curve(proof.B, curve.b2))
        print("self.A on curve:", curve.is_on_curve(proof.A, curve.b))
        print("vk.beta_G2 on curve:", curve.is_on_curve(vk.beta_G2, curve.b2))
        print("vk.alpha_G1 on curve:", curve.is_on_curve(vk.alpha_G1, curve.b))
        print("vk.gamma_G2 on curve:", curve.is_on_curve(vk.gamma_G2, curve.b2))
        print("Kw_gamma_G1 on curve:", curve.is_on_curve(Kw_gamma_G1, curve.b))
        print("vk.delta_G2 on curve:", curve.is_on_curve(vk.delta_G2, curve.b2))
        print("self.C on curve:", curve.is_on_curve(proof.C, curve.b))
        print("neg_A * neg_B == e2 * e3 * e4 ?:",pairing(neg(proof.B),neg(proof.A)) == e2 * e3 * e4)

    return e1 == e2 * e3 * e4


def get_witness_public(pk: ProverKey, w: [],):
    return  w[:len(w)-len(pk.K_delta_G1)]

def to_poly(mtx):
    poly_list = []
    for i in range(0, mtx.shape[0]):
        poly_list.append(Poly(mtx[i][::-1]))
    return poly_list


def evaluate_poly_list(poly_list, x):
    results = []
    for poly in poly_list:
        results.append(poly(x))
    return results


def evaluate_poly(poly, trusted_points, verbose=False):
    coeff = poly.coefficients()[::-1]

    assert len(coeff) == len(trusted_points), "Polynomial degree mismatch!"

    if verbose:
        [print(normalize(point)) for point in trusted_points]

    terms = [multiply(point, int(coeff)) for point, coeff in zip(trusted_points, coeff)]
    evaluation = terms[0]
    for i in range(1, len(terms)):
        evaluation = add(evaluation, terms[i])

    if verbose:
        print("-" * 10)
        print(normalize(evaluation))
    return evaluation
