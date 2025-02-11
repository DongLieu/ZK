from galois import Poly, GF
import galois
import numpy as np
from py_ecc.optimized_bn128 import (
    multiply,
    G1,
    G2,
    add,
    normalize,
    curve_order,
    pairing,
)

p = 21888242871839275222246405745257275088548364400416034343698204186575808495617
FP = GF(p)

prv_r = FP(12)
prv_s = FP(13)

prv_alpha = FP(17)
prv_beta = FP(117)

prv_tau = FP(20)
prv_delta = FP(5)
prv_gamma = FP(4)
prv_x = FP(2)
prv_y = FP(3)
# ==============================================R1CS =============================================

v1 = prv_x * prv_x
v2 = prv_y * prv_y
v3 = 5 * prv_x * v1
v4 = 4 * v1 * v2
out = 5*prv_x**3 - 4*prv_x**2*prv_y**2 + 13*prv_x*prv_y**2 + prv_x**2 - 10*prv_y

prv_witness = FP([1, out, prv_x, prv_y, v1, v2, v3, v4])

print("w =", prv_witness)

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

assert all(np.equal(np.matmul(L, prv_witness) * np.matmul(R, prv_witness), np.matmul(O, prv_witness))), "not equal"
Lw = np.dot(L, prv_witness)
Rw = np.dot(R, prv_witness)
Ow = np.dot(O, prv_witness)
LwRw = np.multiply(Lw, Rw)
assert np.all(LwRw == Ow)
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

# print(f'''L
# {Lp}
# ''')

# print(f'''R
# {Rp}
# ''')

# print(f'''O
# {Op}
# ''')

# # ============================================== def groth16-- =============================================
def split_poly(poly):
    coef = [int(c) for c in poly.coefficients()]
    p1 = coef[-2:]
    p2 = coef[:-2] + [0] * 2

    return galois.Poly(p1, field=FP), galois.Poly(p2, field=FP)

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
        print("-"*10)
        print(normalize(evaluation))
    return evaluation
# # ============================================== groth16-- =============================================
T = galois.Poly([1, p-1], field=FP)
for i in range(2, L.shape[0] + 1):
    T *= galois.Poly([1, p-i], field=FP)


U = galois.Poly((prv_witness @ Lp)[::-1])
V = galois.Poly((prv_witness @ Rp)[::-1])
W = galois.Poly((prv_witness @ Op)[::-1])
H = (U * V - W) // T

u = U(prv_tau)
v = V(prv_tau)
_w = W(prv_tau)
T_tau = T(prv_tau)
ht = H(prv_tau)*T_tau

assert u * v - _w == ht, f"{u} * {v} - {_w} != {ht}"

tau_G1 = [multiply(G1, int(prv_tau**i)) for i in range(0, T.degree)]
# G1[τ^0 * T(τ)], G1[τ^1 * T(τ)], ..., G1[τ^d-1 * T(τ)]
target_G1 = [multiply(G1, int(prv_tau**i * T_tau)) for i in range(0, T.degree - 1)]
# G2[τ^0], G2[τ^1], ..., G2[τ^d-1]
tau_G2 = [multiply(G2, int(prv_tau**i)) for i in range(0, T.degree)]

# G1[u0 * τ^0] + G1[u1 * τ^1] + ... + G1[ud-1 * τ^d-1]
A_G1 = evaluate_poly(U, tau_G1)
# G2[v0 * τ^0] + G2[v1 * τ^1] + ... + G2[vd-1 * τ^d-1]
B_G2 = evaluate_poly(V, tau_G2)
# G1[w0 * τ^0] + G1[w1 * τ^1] + ... + G1[wd-1 * τ^d-1]
B_G1 = evaluate_poly(V, tau_G1)
# G1[w0 * τ^0] + G1[w1 * τ^1] + ... + G1[wd-1 * τ^d-1]
Cw_G1 = evaluate_poly(W, tau_G1)
# G1[h0 * τ^0 * T(τ)] + G1[h1 * τ^1 * T(τ)] + ... + G1[hd-2 * τ^d-2 * T(τ)]
HT_G1 = evaluate_poly(H, target_G1)

C_G1 = add(Cw_G1, HT_G1)
assert pairing(B_G2, A_G1) == pairing(G2, C_G1), "Pairing check failed!"
print("Pairing check passed!")
# ================================
# alpha_G1 = multiply(G1, int(alpha))
# beta_G2 = multiply(G2, int(beta))

# A_G1_aplha = add(A_G1, alpha_G1)
# B_G2_beta = add(B_G2, beta_G2)
# # pairing(A, B) = pairing(α, β) + pairing(β, A) + pairing(α, B) + pairing(C, G2)
# # assert pairing(B_G2_beta, A_G1_aplha) == pairing(beta_G2, alpha_G1) + pairing(beta_G2, A_G1_aplha) + pairing(B_G2_beta, alpha_G1) + pairing(B_G2_beta, alpha_G1),  "Pairing check failed!"
# ================================
U1, U2 = split_poly(U)
V1, V2 = split_poly(V)
W1, W2 = split_poly(W)

w1 = W1(prv_tau)
w2 = W2(prv_tau)

u1 = U1(prv_tau)
u2 = U2(prv_tau)

v1 = V1(prv_tau)
v2 = V2(prv_tau)

c = (prv_beta * u2 + prv_alpha * v2 + w2) + ht 
k = (prv_beta * u1 + prv_alpha * v1 + w1)

assert (u + prv_alpha) * (v + prv_beta) == prv_alpha * prv_beta + k + c
# # ============================================== groth16++ =============================================

U1, U2 = split_poly(U)
W1, W2 = split_poly(W)

u1 = U1(prv_tau)
u2 = U2(prv_tau)

w1 = W1(prv_tau)
w2 = W2(prv_tau)

a = u + prv_alpha + prv_r * prv_delta
b = v + prv_beta + prv_s * prv_delta

c = ((prv_beta * u2 + prv_alpha * v2 + w2) * prv_delta**-1 + ht * prv_delta**-1) + prv_s * a + prv_r * b - prv_r * prv_s * prv_delta
k = (prv_beta * u1 + prv_alpha * v1 + w1) * prv_gamma**-1

assert a * b == prv_alpha * prv_beta + k * prv_gamma + c * prv_delta