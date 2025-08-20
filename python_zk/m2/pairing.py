from py_ecc.bn128 import G1, G2, pairing, multiply, add, eq

# Tạo các điểm trên đường cong
P = multiply(G1, 5)
Q = multiply(G2, 3)
P2 = multiply(G1, 10)
Q2 = multiply(G2, 2)

e1 = pairing(Q, P)
e2 = pairing(Q2, P2)
e3 = pairing(add(Q, Q2), add(P, P2))

# Kiểm tra tính song tuyến tính
if pairing(Q**2, P**3) == e1**6:
    print("4Tính chất song tuyến tính đúng")
else:
    print("4Tính chất song tuyến tính sai")





# Kiểm tra tính song tuyến tính *pairing(G2, G1)
if e1 == pairing(G2, multiply(G1,16))/pairing(G2, G1):
    print("3 Tính chất song tuyến tính đúng")
else:
    print("3 Tính chất song tuyến tính sai")




# Kiểm tra tính song tuyến tính
if e3 == e1 * e2*pairing(Q2, P)*pairing(Q, P2):
    print("0Tính chất song tuyến tính đúng")
else:
    print("0Tính chất song tuyến tính sai")



# Tính các pairing
e1 = pairing(Q, P)
e2 = pairing(Q, P2)
e3 = pairing(Q, add(P, P2) )

# Kiểm tra tính song tuyến tính
if e3 == e1 * e2:
    print("Tính chất song tuyến tính đúng")
else:
    print("Tính chất song tuyến tính sai")

# Kiểm tra tính song tuyến tính
if pairing(add(Q, Q), P) == pairing(Q, P) * pairing(Q, P):
    print("lll1Tính chất song tuyến tính đúng")
else:
    print("llll1Tính chất song tuyến tính sai")




# Kiểm tra tính song tuyến tính
if e2 == e1 * e1:
    print("2 Tính chất song tuyến tính đúng")
else:
    print(" 2 Tính chất song tuyến tính sai")


print("===============")

P_1 = multiply(G1, 3)
P_2 = multiply(G2, 8)

Q_1 = multiply(G1, 6)
Q_2 = multiply(G2, 4)

assert eq(pairing(P_2, P_1), pairing(Q_2, Q_1))

