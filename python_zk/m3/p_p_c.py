from py_ecc.bn128 import G1, G2, pairing, multiply, add, eq, curve_order

p = curve_order
#y = 3x^2 - 2x + 1
def _func_y(x):
    return 3*x**2 -2 * x + 1

def _func_pi(x):
    return 5*x**2 + 4 * x + 3

_a = 3
_b = p-2
_c = 1
_gama1 = 3
_gama2 = 4
_gama3 = 5
_n = 3
_m = 5
G = multiply(G1, _n)
B = multiply(G1, _m)

# C1 = 1G +3B
C1 = add(multiply(G, _c), multiply(B, _gama1))
# C2 = -2G +4B
C2 = add(multiply(G, _b), multiply(B, _gama2))
# C3 = 3G +5B
C3 = add(multiply(G, _a), multiply(B, _gama3))
###################################################################################################################################
# gui u
u = 30
# tinh y, pi roi gui lai 
y = _func_y(u)
pi =_func_pi(u)

# C, u, y, pi

def veryfi(C1, C2, C3, u, y, pi, G, B):
    vt = add(multiply(G, y), multiply(B, pi)) 
    vp = add(add(C1, multiply(C2, u)), multiply(C3, u**2) ) 
    print(vt)
    print(vp)
    return vt == vp

print(veryfi(C1, C2, C3, u, y, pi, G, B))