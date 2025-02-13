import random
import groth16
import numpy as np
import galois

FP = groth16.FP
p = groth16.p

_x = FP(2)
_y = FP(3)
_witness = FP([1, _x, _y])

# ============================================== R1CS =============================================

R = FP([[1, 1, 0]])

L = FP([[1, 0, 0]])

O = FP([[0, 0, 1]])
assert all(np.equal(np.matmul(L, _witness) * np.matmul(R, _witness), np.matmul(O, _witness))), "not equal"
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


print(Lp)
print(Rp)
print(Op)