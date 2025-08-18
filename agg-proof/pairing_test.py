from groth16 import G1, G2, pairing, multiply, add

# 2 * 3 = 6
P_1 = multiply(G1, 3)
P_11 = multiply(P_1,7)
P_22 = multiply(G1, 21)

t = [7,10944121435919637611123202872628637544274182200208017171849102093287904247799,10944121435919637611123202872628637544274182200208017171849102093287904247812]
m = 1
r = G1
for i in t:
    m +=i
    print("i=", i)
    r = add(r, multiply(G1, i))

mm = multiply(G1, m)

print("00000=", multiply(G1, 8))
print(m)
print("++++")
print(mm == r)
print(mm)
print(r)
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

# assert eq(pairing(P_2, P_1) * pairing(Q_2, Q_1), pairing(R_2, R_1))