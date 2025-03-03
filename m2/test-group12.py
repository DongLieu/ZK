from py_ecc.bn128 import G1, G2,G12, pairing, multiply, add, eq, is_on_curve,b12
from py_ecc.bn128 import FQ12
from py_ecc.bn128 import curve_order

P_1 = multiply(G1, 5)
P_2 = multiply(G2, 5)

Q_1 = multiply(G1, 11)
Q_2 = multiply(G2, 2)

e1 = pairing(Q_2, Q_1)
e2 = pairing(P_2, P_1)
eG = pairing(G2, G1)

# Kiểm tra tính song tuyến tính
if e2 == e1*eG**3: #25 = 22 + 3
    print("2 Tính chất song tuyến tính đúng")
else:
    print(" 2 Tính chất song tuyến tính sai")


# v = is_on_curve(e1, b12)
def is_in_G12(e1):
    """ Kiểm tra e1 có thuộc nhóm G12 không """
    if not isinstance(e1, FQ12):
        return False  # e1 phải thuộc FQ12
    return e1 ** curve_order == FQ12.one()  # Kiểm tra e1^curve_order == 1

# Giả sử e1 là kết quả của một phép pairing
print(is_in_G12(e1))  # True nếu e1 thuộc G12
print(is_in_G12(e2))  # True nếu e1 thuộc G12
print(is_in_G12(eG))  # True nếu e1 thuộc G12
print(is_in_G12(G12))  # True nếu e1 thuộc G12

# 200tr:30tr
# 