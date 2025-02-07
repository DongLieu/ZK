import galois
import numpy as np

p = 17
GF = galois.GF(p)

x_values = GF(np.array([1, 2]))

def L(v):
    return galois.lagrange_poly(x_values, v)

p1 = L(GF(np.array([6, 4])))
p2 = L(GF(np.array([3, 7])))
q1 = L(GF(np.array([3, 12])))
q2 = L(GF(np.array([9, 6])))

print(p1)
# 15x + 8 (mod 17)
print(p2)
# 4x + 16 (mod 17)
print(q1)
# 9x + 11 (mod 17)
print(q2)
# 14x + 12 (mod 17)

import random
u = random.randint(0, p)
tau = GF(u) # a random point

left_hand_side = p1(tau) * GF(2) + p2(tau) * GF(4) # 2(15x+8) +4(4x+16) = 46x+80
right_hand_side = q1(tau) * GF(2) + q2(tau) * GF(2) # 2(9x+11) + 2(14x+12) = 46x +46

v = left_hand_side == right_hand_side

print(v)

print("======")

# có thể kiểm tra bằng cách chọn ngâũ nhiên một điểm va thử
# điều đó có đungc với Ra*La=Oa hya khong?

# R1CS đến QAP: Kiểm tra một cách ngắn gọn