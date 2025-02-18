import random
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
    
def keygen(qap: QAP):  # -> (ProverKey, VerifierKey)
    # generating toxic waste
    alpha = FP(random.randint(2, p - 1))
    beta = FP(random.randint(2, p - 1))
    gamma = FP(random.randint(2, p - 1))
    delta = FP(random.randint(2, p - 1))
    tau = FP(random.randint(2, p - 1))
    l=random.randrange(2, qap.L.shape[0])

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

    return pk, vk


def prove(pk: ProverKey, w: [], qap: QAP):
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

    return Proof(A_G1, B_G2, C_G1)

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
