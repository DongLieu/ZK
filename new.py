
# Import module
import random
import groth16
import numpy as np
import galois

FP = groth16.FP
p = groth16.p

# f(x,y) = x^3 + x^2*y^2 +x*y^2-x^2+y^4 
_x = FP(2)
_y = FP(3)
_v1 = _x * _x
_v2 = _y * _y
out = _x**3 + _x**2*_y**2 + _x*_y**2 - _x**2 + _y**4
_witness = FP([1, out, _x, _y, _v1, _v2])
# ============================================== R1CS =============================================

R = FP([ [0, 0, 1, 0, 0, 0],
         [0, 0, 0, 1, 0, 0],
         [0, 0, 0, 0, 1, 1]])

L = FP([ [0, 0, 1, 0, 0, 0],
         [0, 0, 0, 1, 0, 0],
         [0, 0, 1, 0, 0, 1]])

O = FP([ [0, 0, 0, 0, 1, 0],
         [0, 0, 0, 0, 0, 1],
         [0, 1, 0, 0, 1, 0]])
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

# # ============================================== groth16 =============================================
qap = groth16.QAP(Lp, Rp, Op, T)
_pk,vk = groth16.keygen(qap)
proof = groth16.prove(_pk,_witness, qap)

w_public = groth16.get_witness_public(_pk, _witness)
ok = groth16.verifier(vk,w_public, proof)
print(ok)
