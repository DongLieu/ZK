import plonk

# Build the same arithmetic relation used by draft-grorth16/test-groth16.py:
# f(x, y) = 5x^3 - 4x^2y^2 + 13xy^2 + x^2 - 10y.
circuit = plonk.build_example_circuit()
pk, vk = plonk.keygen(circuit)

# Witness 1 ---------------------------------------------------------------
witness1 = plonk.example_witness(2, 3)
proof1 = plonk.prove(pk, witness1)
public1 = plonk.get_public_inputs(witness1)
ok1 = plonk.verify(vk, public1, proof1)
print("Proof 1 valid:", ok1)

# Witness 2 ---------------------------------------------------------------
witness2 = plonk.example_witness(4, 5)
proof2 = plonk.prove(pk, witness2)
public2 = plonk.get_public_inputs(witness2)
ok2 = plonk.verify(vk, public2, proof2)
print("Proof 2 valid:", ok2)
