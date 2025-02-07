from py_ecc.bn128 import G1, G2, multiply, neg, eq, pairing

# chosen arbitrarily
x = 10
y = 100
A = multiply(G1, x)
B = multiply(G2, y)

A_p = neg(A)
B_p = neg(B)

assert eq(pairing(B, A), pairing(B_p, A_p))

