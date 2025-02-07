from py_ecc.bn128 import G1, G2, pairing, multiply, eq

# 2 * 3 = 6
P_1 = multiply(G1, 2)
P_2 = multiply(G2, 3)

# 4 * 5 = 20
Q_1 = multiply(G1, 4)
Q_2 = multiply(G2, 5)

# 10 * 12 = 120 (6 * 20 = 120 also)
R_1 = multiply(G1, 10)
R_2 = multiply(G2, 12)

# assert eq(pairing(P_2, P_1) * pairing(Q_2, Q_1), pairing(R_2, R_1))

# Fails!

# 13 * 2 = 26
R_1 = multiply(G1, 13)
R_2 = multiply(G2, 2)

# b ^ {2 * 3} * b ^ {4 * 5} = b ^ {13 * 2}
# b ^ 6 * b ^ 20 = b ^ 26

assert eq(pairing(P_2, P_1) * pairing(Q_2, Q_1), pairing(R_2, R_1))