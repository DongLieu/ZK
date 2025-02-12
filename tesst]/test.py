from py_ecc import bn128

# Example: Scalar multiplication on elliptic curve
G = bn128.G1  # Generator point
scalar = 1

# Perform scalar multiplication
result = bn128.multiply(G, scalar)
print("Result of scalar multiplication:", result)
