
# Import module
import random
import groth16
import numpy as np
import galois

FP = groth16.FP
p = groth16.p

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
_pk,vk = groth16.keygen(qap)

# # ============================================== proof 1 =============================================
proof1 = groth16.prove(_pk,_witness1, qap)
w_public = groth16.get_witness_public(_pk, _witness1)
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