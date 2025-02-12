
# Import module
import groth16
from galois import Poly, GF
import numpy as np
import galois
from py_ecc.optimized_bn128 import (
    multiply,
    G1,
    G2,
    add,
    normalize,
    curve_order,
)

FP = groth16.FP
p = 21888242871839275222246405745257275088548364400416034343698204186575808495617

prv_x = FP(2)
prv_y = FP(3)
v1 = prv_x * prv_x
v2 = prv_y * prv_y
v3 = 5 * prv_x * v1
v4 = 4 * v1 * v2
out = 5*prv_x**3 - 4*prv_x**2*prv_y**2 + 13*prv_x*prv_y**2 + prv_x**2 - 10*prv_y
witness = FP([1, out, prv_x, prv_y, v1, v2, v3, v4])
# ==============================================R1CS =============================================

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

# print(L.shape[0])

T = galois.Poly([1, p-1], field=FP)
for i in range(2, L.shape[0] + 1):
    T *= galois.Poly([1, p-i], field=FP)

# # ============================================== lib =============================================
qap = groth16.QAP(Lp, Rp, Op, T)
pk,vk = groth16.keygen(qap=qap)

# print(witness[:2])
# print(witness[2:])

proof = groth16.prove(pk, witness[:2], witness[2:], qap)

v = groth16.verifier(vk,witness[:2], proof)
print(v)
# print(pk.__repr__)