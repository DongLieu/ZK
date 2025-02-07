# <[3,2,5,10],[2^3,2^2,2^1,2^0]> = 3*8+2*4+5*2+10*1

# ZK-SNARK sử dụng để đánh giá đa thức ở giá trị bí mật.
# [Q3,Q2,Q1,G1] = [r^3.G1,r^2.G1, r^1.G1, r^0.G1]

from py_ecc.bn128 import G1,G2, multiply, add
from py_ecc.bn128 import is_on_curve
from functools import reduce
from py_ecc.bn128 import pairing
from py_ecc.bn128 import FQ

# Tính tích vô hướng giữa hai danh sách
def inner_product(points, coeffs):
    return reduce(add, map(multiply, points, coeffs))

## Trusted Setup
tau = 88
degree = 3

# tau^3, tau^2, tau, 1
pub_srs = [multiply(G1, tau**i) for i in range(degree,-1,-1)]


print(G1)
print(multiply(G1, 2))
## Evaluate
# p(x) = 4x^2 + 7x + 8
coeffs = [0, 4, 7, 8]

# valid = pairing(A, B) == pairing(C, D)
# print(valid)

poly_at_tau = inner_product(pub_srs, coeffs)

x, y = poly_at_tau  # poly_at_tau có dạng (x, y)
poly_at_tau_valid = (FQ(x), FQ(y))  # Đúng định dạng

print(is_on_curve(poly_at_tau_valid, FQ(3)))
print(poly_at_tau)


# ========================== verryfi TS ============================== # 
# neu cung cap O = r.G2 khi do e(O, Qi) == e(G2, Qi+1) // r.G2.r^i.G1 == G2.r^(r+1).G1
pub_O = multiply(G2, tau)
valid = pairing(pub_O, pub_srs[2]) == pairing(G2, pub_srs[1]) # do pub_srs sap xep tu cao -> thap
print(valid)


# ========================== nhieu nguoi ky ============================== # 
# tau2 chi bit ve 'pub_O' va 'pub_srs'
tau2 = 86
degree2 = 3
## Evaluate
# p(x) = 4x^2 + 9x + 8
coeffs2 = [0, 4, 9, 8]

# (tau*tau2)^3, (tau*tau2)^2, (tau*tau2), 1
pub_srs2 = [multiply(pub_srs[degree2-i], tau2**i) for i in range(degree2,-1,-1)]

poly_at_tau2 = inner_product(pub_srs2, coeffs2)

x, y = poly_at_tau2  # poly_at_tau có dạng (x, y)
poly_at_tau_valid2 = (FQ(x), FQ(y))  # Đúng định dạng

print("=======")
print(is_on_curve(poly_at_tau_valid2, FQ(3)))

pub_O2 = multiply(pub_O, tau2)
valid2 = pairing(pub_O2, pub_srs2[2]) == pairing(G2, pub_srs2[1]) # do pub_srs sap xep tu cao -> thap
print(valid2)
