
from galois import Poly, GF
from py_ecc.optimized_bn128 import (
    multiply,
    G1,
    curve_order,
)
p = curve_order
FP = GF(p)

# f(x,y) = x^3 + x^2*y^2 +x*y^2-x^2+y^4 
n = 10
x_ran = 2
pow_x_G1 = [
    (multiply(G1, x_ran ** i)) for i in range(0, n+4) #
]


_x = FP(2)
_y = FP(3)
_t1 = _x * _x # x^2
_t2 = _y * _y # y^2
_t3 = _t1*_x # x^3
_t4 = _t1*_t2 # x^2*y^2
_t5 = _x*_t2 #x*y^2
_t6 = _t2*_t2 # y^4
_t7 = _t3 +_t4 # x^3 +x^2*y^2
_t8 = _t5 - _t1 # x*y^2 -x^2
_t9 = _t8 + _t7 # x*y^2 -x^2 +x^3 +x^2*y^2
_t10 = _t9 +_t6 # x*y^2 -x^2 +x^3 +x^2*y^2 + y^4

witness_values = {
    "w_L":  [_x, _y, _t1, _t1, _x, _t2, _t3, _t5, _t8, _t9],
    "w_R":  [_x, _y, _x, _t2, _t2, _t2, _t4, _t1, _t7, _t6],
    "w_O":  [_t1, _t2, _t3, _t4, _t5, _t6, _t7, _t8, _t9, _t10],
}


# ============================================== plonk =============================================

q = {
    "q_M":  [1, 1, 1, 1, 1, 1, 0, 0, 0, 0],
    "q_L":  [0, 0, 0, 0, 0, 0, 1, 1, 1, 1],
    "q_R":  [0, 0, 0, 0, 0, 0, 1, -1, 1, 1],
    "q_O":  [-1, -1, -1, -1, -1, -1, -1, -1, -1, -1],
    "q_C":  [0, 0, 0, 0, 0, 0, 0, 0, 0, 0],
}

# Wire assignment (indexing wires)
wires = {
    "w_L":  [0, 1, 2, 3, 4, 5, 6, 7, 8, 9],
    "w_R":  [1, 2, 3, 4, 5, 6, 7, 8, 9, 0],
    "w_O":  [2, 3, 4, 5, 6, 7, 8, 9, 0, 1],
}

# Hoán vị permutation sigma
sigma = {
    "sigma_L": [wires["w_R"][i] for i in range(n)],  
    "sigma_R": [wires["w_O"][i] for i in range(n)],  
    "sigma_O": [wires["w_L"][i] for i in range(n)],  
}

