import numpy as np
import galois
from functools import reduce



GF = galois.GF(79)
# 1, out, x, y, v1, v2, v3
L = np.array([
    [0, 0, 1, 0, 0, 0, 0],
    [0, 0, 0, 0, 1, 0, 0],
    [0, 0, 0, -5, 0, 0, 0],
    [0, 0, 0, 0, 0, 0, 1],
])

R = np.array([
    [0, 0, 1, 0, 0, 0, 0],
    [0, 0, 0, 0, 1, 0, 0],
    [0, 0, 0, 1, 0, 0, 0],
    [0, 0, 0, 0, 1, 0, 0],
])

O = np.array([
    [0, 0, 0, 0, 1, 0, 0],
    [0, 0, 0, 0, 0, 1, 0],
    [0, 0, 0, 0, 0, 0, 1],
    [0, 1, 0, 0, 0, -1, 0],
])

L = (L + 79) % 79
R = (R + 79) % 79
O = (O + 79) % 79

print(L)

L_galois = GF(L)
R_galois = GF(R)
O_galois = GF(O)


x = GF(4)
y = GF(-2 + 79) # we are using 79 as the field size, so 79 - 2 is -2
v1 = x * x
v2 = v1 * v1         # x^4
v3 = GF(-5 + 79)*y * y
out = v3*v1 + v2    # -5y^2 * x^2

witness = GF(np.array([1, out, x, y, v1, v2, v3]))

assert all(np.equal(np.matmul(L_galois, witness) * np.matmul(R_galois, witness), np.matmul(O_galois, witness))), "not equal"

print(all(np.equal(np.matmul(L_galois, witness) * np.matmul(R_galois, witness), np.matmul(O_galois, witness))))


def interpolate_column(col):
    xs = GF(np.array([1,2,3,4]))
    return galois.lagrange_poly(xs, col)

# axis 0 is the columns.
# apply_along_axis is the same as doing a for loop over the columns and collecting the results in an array
# trục 0 là các cột.
# apply_along_axis giống như thực hiện vòng lặp for trên các cột và thu thập kết quả trong một mảng
U_polys = np.apply_along_axis(interpolate_column, 0, L_galois)
V_polys = np.apply_along_axis(interpolate_column, 0, R_galois)
W_polys = np.apply_along_axis(interpolate_column, 0, O_galois)

print(U_polys[:2])
print(V_polys[:2])
print(W_polys[:1])

# [Poly(0, GF(79)) Poly(0, GF(79))]# [Poly(0, GF(79)) Poly(0, GF(79))]# [Poly(0, GF(79))]

def inner_product_polynomials_with_witness(polys, witness):
    mul_ = lambda x, y: x * y
    sum_ = lambda x, y: x + y
    return reduce(sum_, map(mul_, polys, witness))

term_1 = inner_product_polynomials_with_witness(U_polys, witness)

term_2 = inner_product_polynomials_with_witness(V_polys, witness)

term_3 = inner_product_polynomials_with_witness(W_polys, witness)

# t = (x - 1)(x - 2)(x - 3)(x - 4)
t = galois.Poly([1, 78], field = GF) * galois.Poly([1, 77], field = GF) * galois.Poly([1, 76], field = GF) * galois.Poly([1, 75], field = GF)

h = (term_1 * term_2 - term_3) // t

assert term_1 * term_2 == term_3+ h * t, "division has a remainder"

