from py_ecc.bn128 import G1, multiply, add, FQ, eq, Z1
from py_ecc.bn128 import curve_order as p
import numpy as np
from functools import reduce
import random

G = [(FQ(6286155310766333871795042970372566906087502116590250812133967451320632869759), FQ(2167390362195738854837661032213065766665495464946848931705307210578191331138)),
     (FQ(6981010364086016896956769942642952706715308592529989685498391604818592148727), FQ(8391728260743032188974275148610213338920590040698592463908691408719331517047))]

a = np.array([1,2])
A = add(multiply(G[1],a[1]) , multiply(G[0],a[0]))
L = multiply(G[1],a[0])
R = multiply(G[0],a[1])

u = 3
a_ = a[0] + a[1]*u

# L+R = (a1+a2)(G1 + G2)

# L + u*A + u^2*R = (a1 + a2*u)(uG1 + G2)-> 

def check(A,L,R,a_,u):
    vt = add(add(L, multiply(A, u)), multiply(R, u*u))
    vp = multiply(add(multiply(G[0], u), G[1]), a_)
    return vt==vp

print(check(A, L, R, a_, u))