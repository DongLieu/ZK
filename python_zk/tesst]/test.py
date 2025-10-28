from py_ecc import bn128

# Example: Scalar multiplication on elliptic curve
G = bn128.G1  # Generator point
scalar = 1

# Perform scalar multiplication
result = bn128.multiply(G, scalar)
print("Result of scalar multiplication:", result)


from py_ecc.bn128 import G1, multiply, add, FQ, eq, Z1
from py_ecc.bn128 import curve_order as p
import numpy as np
from functools import reduce
import random

def random_element():
    return random.randint(0, p)

def add_points(*points):
    return reduce(add, points, Z1)

# if points = G1, G2, G3, G4 and scalars = a,b,c,d vector_commit returns
# aG1 + bG2 + cG3 + dG4
def vector_commit(points, scalars):
    return reduce(add, [multiply(P, i) for P, i in zip(points, scalars)], Z1)

from py_ecc.bn128 import G1, multiply, add, Z1
from functools import reduce

# Giả sử có 3 điểm trên G1
points = [G1, multiply(G1, 2), multiply(G1, 3)]

# Hệ số vô hướng tương ứng
scalars = [5, 7, 11]

# Cam kết vector
commitment = vector_commit(points, scalars)


k = add(add(multiply(points[0], 5), multiply(points[1], 7)), multiply(points[2], 11))

print(commitment)
print(k)
